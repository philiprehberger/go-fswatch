// Package fswatch provides a polling-based file system watcher for Go.
//
// It uses polling (not OS-level events) to detect file changes, which means
// zero external dependencies. File modification times are tracked via os.Stat,
// directories are walked with filepath.WalkDir, and glob matching uses
// filepath.Match from the standard library.
package fswatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Option configures a Watcher.
type Option func(*config)

type config struct {
	paths        []string
	globs        []string
	ignores      []string
	debounce     time.Duration
	pollInterval time.Duration
	recursive    bool
}

func defaultConfig() config {
	return config{
		debounce:     500 * time.Millisecond,
		pollInterval: 1 * time.Second,
		recursive:    true,
	}
}

// Paths sets the directories to watch.
func Paths(paths ...string) Option {
	return func(c *config) {
		c.paths = append(c.paths, paths...)
	}
}

// Glob sets file patterns to include (e.g., "*.yaml", "*.go").
// Patterns are matched using filepath.Match against the file's base name.
// If no glob patterns are set, all files are included.
func Glob(patterns ...string) Option {
	return func(c *config) {
		c.globs = append(c.globs, patterns...)
	}
}

// Ignore sets patterns to exclude (e.g., ".git", "*.tmp", "node_modules").
// Patterns are matched using filepath.Match against the file's base name.
func Ignore(patterns ...string) Option {
	return func(c *config) {
		c.ignores = append(c.ignores, patterns...)
	}
}

// Debounce sets the debounce interval. Events are batched and delivered
// after no new events have been detected for this duration. Default is 500ms.
func Debounce(d time.Duration) Option {
	return func(c *config) {
		c.debounce = d
	}
}

// PollInterval sets how often the watcher checks for file system changes.
// Default is 1s.
func PollInterval(d time.Duration) Option {
	return func(c *config) {
		c.pollInterval = d
	}
}

// Recursive sets whether subdirectories are watched. Default is true.
func Recursive(enabled bool) Option {
	return func(c *config) {
		c.recursive = enabled
	}
}

// Watcher watches directories for file system changes using polling.
type Watcher struct {
	cfg      config
	onChange func(events []Event)
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
}

// New creates a new Watcher with the given options.
// At least one path must be specified via the Paths option.
func New(opts ...Option) (*Watcher, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if len(cfg.paths) == 0 {
		return nil, errors.New("fswatch: at least one path is required")
	}
	return &Watcher{
		cfg:  cfg,
		done: make(chan struct{}),
	}, nil
}

// OnChange registers a callback that is called with a batch of events
// whenever file system changes are detected. Only one callback can be
// registered; subsequent calls overwrite the previous callback.
func (w *Watcher) OnChange(fn func(events []Event)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = fn
}

// Start begins watching for file system changes. It blocks until the
// provided context is cancelled or Close is called.
func (w *Watcher) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancel = cancel
	w.mu.Unlock()

	defer func() {
		cancel()
		close(w.done)
	}()

	snapshot := w.scan()
	ticker := time.NewTicker(w.cfg.pollInterval)
	defer ticker.Stop()

	var pending []Event
	var debounceTimer *time.Timer
	var debounceCh <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			// Deliver any remaining pending events before exiting.
			if len(pending) > 0 {
				w.deliver(pending)
			}
			return ctx.Err()

		case <-ticker.C:
			current := w.scan()
			events := diff(snapshot, current)
			snapshot = current

			if len(events) > 0 {
				pending = append(pending, events...)
				// Reset debounce timer.
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.NewTimer(w.cfg.debounce)
				debounceCh = debounceTimer.C
			}

		case <-debounceCh:
			if len(pending) > 0 {
				w.deliver(pending)
				pending = nil
			}
			debounceCh = nil
		}
	}
}

// Close stops the watcher. It is safe to call multiple times.
func (w *Watcher) Close() error {
	w.mu.Lock()
	cancel := w.cancel
	w.mu.Unlock()

	if cancel != nil {
		cancel()
		<-w.done
	}
	return nil
}

// deliver calls the registered onChange callback with the given events.
func (w *Watcher) deliver(events []Event) {
	w.mu.Lock()
	fn := w.onChange
	w.mu.Unlock()

	if fn != nil {
		fn(events)
	}
}

// scan walks all configured paths and returns a snapshot of file modification times.
func (w *Watcher) scan() map[string]time.Time {
	snapshot := make(map[string]time.Time)
	for _, root := range w.cfg.paths {
		if w.cfg.recursive {
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if w.isIgnored(d.Name()) && path != root {
						return filepath.SkipDir
					}
					return nil
				}
				if w.matchFile(d.Name()) {
					info, err := d.Info()
					if err == nil {
						snapshot[path] = info.ModTime()
					}
				}
				return nil
			})
		} else {
			entries, err := os.ReadDir(root)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if w.matchFile(entry.Name()) {
					info, err := entry.Info()
					if err == nil {
						snapshot[filepath.Join(root, entry.Name())] = info.ModTime()
					}
				}
			}
		}
	}
	return snapshot
}

// matchFile checks whether a filename matches the configured glob and ignore patterns.
func (w *Watcher) matchFile(name string) bool {
	if w.isIgnored(name) {
		return false
	}
	if len(w.cfg.globs) == 0 {
		return true
	}
	for _, pattern := range w.cfg.globs {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// isIgnored checks whether a name matches any ignore pattern.
func (w *Watcher) isIgnored(name string) bool {
	for _, pattern := range w.cfg.ignores {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// diff compares two snapshots and returns events describing the changes.
func diff(prev, curr map[string]time.Time) []Event {
	var events []Event

	for path, modTime := range curr {
		prevTime, exists := prev[path]
		if !exists {
			events = append(events, Event{Path: path, Op: Create, ModTime: modTime})
		} else if !modTime.Equal(prevTime) {
			events = append(events, Event{Path: path, Op: Modify, ModTime: modTime})
		}
	}

	for path := range prev {
		if _, exists := curr[path]; !exists {
			events = append(events, Event{Path: path, Op: Delete})
		}
	}

	return events
}
