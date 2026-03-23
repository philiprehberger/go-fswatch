# go-fswatch

[![CI](https://github.com/philiprehberger/go-fswatch/actions/workflows/ci.yml/badge.svg)](https://github.com/philiprehberger/go-fswatch/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/philiprehberger/go-fswatch.svg)](https://pkg.go.dev/github.com/philiprehberger/go-fswatch) [![License](https://img.shields.io/github/license/philiprehberger/go-fswatch)](LICENSE)

Polling-based file system watcher for Go. Zero dependencies

## Installation

```bash
go get github.com/philiprehberger/go-fswatch
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/philiprehberger/go-fswatch"
)

func main() {
	w, err := fswatch.New(
		fswatch.Paths("./src", "./config"),
		fswatch.Glob("*.go", "*.yaml"),
		fswatch.Ignore(".git", "*.tmp", "node_modules"),
		fswatch.Debounce(300*time.Millisecond),
		fswatch.PollInterval(1*time.Second),
		fswatch.Recursive(true),
	)
	if err != nil {
		panic(err)
	}

	w.OnChange(func(events []fswatch.Event) {
		for _, e := range events {
			fmt.Printf("%s %s\n", e.Op, e.Path)
		}
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := w.Start(ctx); err != nil && err != context.Canceled {
		panic(err)
	}
}
```

### Per-Event Callbacks

Register callbacks for specific event types. These fire in addition to `OnChange`:

```go
w.OnCreate(func(e fswatch.Event) {
	fmt.Println("created:", e.Path)
})

w.OnModify(func(e fswatch.Event) {
	fmt.Println("modified:", e.Path)
})

w.OnDelete(func(e fswatch.Event) {
	fmt.Println("deleted:", e.Path)
})
```

### Single File Watching

Use `WatchFile` to watch a single file without setting up paths and globs manually:

```go
w, err := fswatch.WatchFile("config.yaml", func(e fswatch.Event) {
	fmt.Printf("config changed: %s\n", e.Op)
})
if err != nil {
	panic(err)
}

ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
_ = w.Start(ctx)
```

### Snapshot

Retrieve the current map of tracked files and their last modification times:

```go
snap := w.Snapshot()
for path, modTime := range snap {
	fmt.Printf("%s last modified at %s\n", path, modTime)
}
```

### Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `Paths(...)` | Directories to watch | required |
| `Glob(...)` | Include patterns | all files |
| `Ignore(...)` | Exclude patterns | none |
| `Debounce(d)` | Debounce interval | 500ms |
| `PollInterval(d)` | Poll frequency | 1s |
| `Recursive(bool)` | Watch subdirs | true |
| `MaxDepth(n)` | Limit recursion depth (0 = top dir only) | unlimited |

### Events

| Op | Description |
|----|-------------|
| `Create` | New file detected |
| `Modify` | File content changed |
| `Delete` | File removed |

## API

| Function / Type | Description |
|-----------------|-------------|
| `New(opts ...Option) (*Watcher, error)` | Create a new watcher |
| `WatchFile(path string, fn func(Event), opts ...Option) (*Watcher, error)` | Watch a single file |
| `(*Watcher).OnChange(fn func([]Event))` | Register batch change callback |
| `(*Watcher).OnCreate(fn func(Event))` | Register create-only callback |
| `(*Watcher).OnModify(fn func(Event))` | Register modify-only callback |
| `(*Watcher).OnDelete(fn func(Event))` | Register delete-only callback |
| `(*Watcher).Start(ctx context.Context) error` | Start watching (blocks) |
| `(*Watcher).Close() error` | Stop watching |
| `(*Watcher).Snapshot() map[string]time.Time` | Get tracked files and mod times |
| `Paths(paths ...string) Option` | Set directories to watch |
| `Glob(patterns ...string) Option` | Set include patterns |
| `Ignore(patterns ...string) Option` | Set exclude patterns |
| `Debounce(d time.Duration) Option` | Set debounce interval |
| `PollInterval(d time.Duration) Option` | Set poll frequency |
| `Recursive(enabled bool) Option` | Enable/disable recursive watching |
| `MaxDepth(n int) Option` | Limit recursion depth |
| `Event` | File system change event |
| `Op` | Operation type (Create, Modify, Delete) |

## Development

```bash
go test ./...
go vet ./...
```

## License

MIT
