package fswatch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	testPoll     = 50 * time.Millisecond
	testDebounce = 100 * time.Millisecond
	testWait     = 500 * time.Millisecond
)

func TestNew(t *testing.T) {
	dir := t.TempDir()

	w, err := New(Paths(dir))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if w == nil {
		t.Fatal("New() returned nil watcher")
	}
}

func TestNew_NoPaths(t *testing.T) {
	_, err := New()
	if err == nil {
		t.Fatal("New() without paths should return error")
	}
}

func TestWatcher_DetectsCreate(t *testing.T) {
	dir := t.TempDir()

	w, err := New(
		Paths(dir),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	var mu sync.Mutex
	var received []Event
	done := make(chan struct{}, 1)

	w.OnChange(func(events []Event) {
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Wait for initial scan to complete.
	time.Sleep(testPoll * 3)

	// Create a new file.
	f := filepath.Join(dir, "newfile.txt")
	if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for create event")
	}

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, e := range received {
		if e.Op == Create && filepath.Base(e.Path) == "newfile.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Create event for newfile.txt, got: %v", received)
	}
}

func TestWatcher_DetectsModify(t *testing.T) {
	dir := t.TempDir()

	// Create file before starting watcher.
	f := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(f, []byte("original"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	w, err := New(
		Paths(dir),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	var mu sync.Mutex
	var received []Event
	done := make(chan struct{}, 1)

	w.OnChange(func(events []Event) {
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Wait for initial scan.
	time.Sleep(testPoll * 3)

	// Modify the file — ensure mod time changes.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(f, []byte("modified"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for modify event")
	}

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, e := range received {
		if e.Op == Modify && filepath.Base(e.Path) == "existing.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Modify event for existing.txt, got: %v", received)
	}
}

func TestWatcher_DetectsDelete(t *testing.T) {
	dir := t.TempDir()

	// Create file before starting watcher.
	f := filepath.Join(dir, "todelete.txt")
	if err := os.WriteFile(f, []byte("bye"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	w, err := New(
		Paths(dir),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	var mu sync.Mutex
	var received []Event
	done := make(chan struct{}, 1)

	w.OnChange(func(events []Event) {
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Wait for initial scan.
	time.Sleep(testPoll * 3)

	// Delete the file.
	if err := os.Remove(f); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for delete event")
	}

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, e := range received {
		if e.Op == Delete && filepath.Base(e.Path) == "todelete.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Delete event for todelete.txt, got: %v", received)
	}
}

func TestWatcher_GlobFilter(t *testing.T) {
	dir := t.TempDir()

	w, err := New(
		Paths(dir),
		Glob("*.go"),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	var mu sync.Mutex
	var received []Event
	done := make(chan struct{}, 1)

	w.OnChange(func(events []Event) {
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Wait for initial scan.
	time.Sleep(testPoll * 3)

	// Create a .txt file (should be ignored) and a .go file (should match).
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "match.go"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for glob-filtered event")
	}

	// Give extra time for any additional events.
	time.Sleep(testDebounce * 2)

	mu.Lock()
	defer mu.Unlock()

	for _, e := range received {
		if filepath.Base(e.Path) == "ignore.txt" {
			t.Error("received event for ignore.txt, which should have been filtered by glob")
		}
	}

	found := false
	for _, e := range received {
		if e.Op == Create && filepath.Base(e.Path) == "match.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Create event for match.go, got: %v", received)
	}
}

func TestWatcher_IgnoreFilter(t *testing.T) {
	dir := t.TempDir()

	w, err := New(
		Paths(dir),
		Ignore("*.tmp"),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	var mu sync.Mutex
	var received []Event
	done := make(chan struct{}, 1)

	w.OnChange(func(events []Event) {
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Wait for initial scan.
	time.Sleep(testPoll * 3)

	// Create an ignored .tmp file and a normal .txt file.
	if err := os.WriteFile(filepath.Join(dir, "temp.tmp"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "normal.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for event")
	}

	// Give extra time for any additional events.
	time.Sleep(testDebounce * 2)

	mu.Lock()
	defer mu.Unlock()

	for _, e := range received {
		if filepath.Base(e.Path) == "temp.tmp" {
			t.Error("received event for temp.tmp, which should have been ignored")
		}
	}

	found := false
	for _, e := range received {
		if e.Op == Create && filepath.Base(e.Path) == "normal.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Create event for normal.txt, got: %v", received)
	}
}

func TestWatcher_Recursive(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory.
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("Mkdir error: %v", err)
	}

	w, err := New(
		Paths(dir),
		Recursive(true),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	var mu sync.Mutex
	var received []Event
	done := make(chan struct{}, 1)

	w.OnChange(func(events []Event) {
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Wait for initial scan.
	time.Sleep(testPoll * 3)

	// Create a file in the subdirectory.
	f := filepath.Join(subdir, "deep.txt")
	if err := os.WriteFile(f, []byte("deep"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for recursive event")
	}

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, e := range received {
		if e.Op == Create && filepath.Base(e.Path) == "deep.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Create event for deep.txt in subdirectory, got: %v", received)
	}
}
