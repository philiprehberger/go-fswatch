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

func TestWatcher_OnCreate(t *testing.T) {
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
	var createEvents []Event
	done := make(chan struct{}, 1)

	w.OnCreate(func(e Event) {
		mu.Lock()
		createEvents = append(createEvents, e)
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

	time.Sleep(testPoll * 3)

	if err := os.WriteFile(filepath.Join(dir, "created.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for OnCreate callback")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(createEvents) == 0 {
		t.Fatal("OnCreate callback was not called")
	}
	if createEvents[0].Op != Create {
		t.Errorf("expected Create op, got %v", createEvents[0].Op)
	}
	if filepath.Base(createEvents[0].Path) != "created.txt" {
		t.Errorf("expected created.txt, got %s", createEvents[0].Path)
	}
}

func TestWatcher_OnModify(t *testing.T) {
	dir := t.TempDir()

	f := filepath.Join(dir, "modme.txt")
	if err := os.WriteFile(f, []byte("original"), 0644); err != nil {
		t.Fatal(err)
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
	var modifyEvents []Event
	done := make(chan struct{}, 1)

	w.OnModify(func(e Event) {
		mu.Lock()
		modifyEvents = append(modifyEvents, e)
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

	time.Sleep(testPoll * 3)
	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(f, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for OnModify callback")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(modifyEvents) == 0 {
		t.Fatal("OnModify callback was not called")
	}
	if modifyEvents[0].Op != Modify {
		t.Errorf("expected Modify op, got %v", modifyEvents[0].Op)
	}
}

func TestWatcher_OnDelete(t *testing.T) {
	dir := t.TempDir()

	f := filepath.Join(dir, "delme.txt")
	if err := os.WriteFile(f, []byte("bye"), 0644); err != nil {
		t.Fatal(err)
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
	var deleteEvents []Event
	done := make(chan struct{}, 1)

	w.OnDelete(func(e Event) {
		mu.Lock()
		deleteEvents = append(deleteEvents, e)
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

	time.Sleep(testPoll * 3)

	if err := os.Remove(f); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for OnDelete callback")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(deleteEvents) == 0 {
		t.Fatal("OnDelete callback was not called")
	}
	if deleteEvents[0].Op != Delete {
		t.Errorf("expected Delete op, got %v", deleteEvents[0].Op)
	}
}

func TestWatcher_PerEventAndOnChangeBothFire(t *testing.T) {
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
	onChangeCalled := false
	onCreateCalled := false
	done := make(chan struct{}, 1)

	w.OnChange(func(events []Event) {
		mu.Lock()
		onChangeCalled = true
		mu.Unlock()
	})

	w.OnCreate(func(e Event) {
		mu.Lock()
		onCreateCalled = true
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

	time.Sleep(testPoll * 3)

	if err := os.WriteFile(filepath.Join(dir, "both.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for callbacks")
	}

	// Small extra wait to ensure both callbacks had time.
	time.Sleep(testDebounce)

	mu.Lock()
	defer mu.Unlock()

	if !onChangeCalled {
		t.Error("OnChange was not called")
	}
	if !onCreateCalled {
		t.Error("OnCreate was not called")
	}
}

func TestWatchFile(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "watched.txt")
	other := filepath.Join(dir, "other.txt")

	// Create the target file.
	if err := os.WriteFile(target, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var received []Event
	done := make(chan struct{}, 1)

	w, err := WatchFile(target, func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	},
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("WatchFile() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	time.Sleep(testPoll * 3)

	// Create a different file (should be ignored by glob).
	if err := os.WriteFile(other, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modify the watched file.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(target, []byte("updated"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("timed out waiting for WatchFile event")
	}

	// Extra wait for any other events.
	time.Sleep(testDebounce * 2)

	mu.Lock()
	defer mu.Unlock()

	for _, e := range received {
		if filepath.Base(e.Path) == "other.txt" {
			t.Error("received event for other.txt, which should not be watched")
		}
	}

	found := false
	for _, e := range received {
		if filepath.Base(e.Path) == "watched.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected event for watched.txt, got: %v", received)
	}
}

func TestMaxDepth(t *testing.T) {
	dir := t.TempDir()

	// Create nested directories: dir/a/b/c
	dirA := filepath.Join(dir, "a")
	dirAB := filepath.Join(dirA, "b")
	dirABC := filepath.Join(dirAB, "c")
	for _, d := range []string{dirA, dirAB, dirABC} {
		if err := os.Mkdir(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create files at each level before starting watcher.
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirA, "level1.txt"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirAB, "level2.txt"), []byte("2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirABC, "level3.txt"), []byte("3"), 0644); err != nil {
		t.Fatal(err)
	}

	// MaxDepth(1): should see root and level 1, but not level 2 or 3.
	w, err := New(
		Paths(dir),
		MaxDepth(1),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Wait for initial scan.
	time.Sleep(testPoll * 3)

	snap := w.Snapshot()

	// Check that root.txt and level1.txt are tracked.
	foundRoot := false
	foundLevel1 := false
	foundLevel2 := false
	foundLevel3 := false

	for path := range snap {
		base := filepath.Base(path)
		switch base {
		case "root.txt":
			foundRoot = true
		case "level1.txt":
			foundLevel1 = true
		case "level2.txt":
			foundLevel2 = true
		case "level3.txt":
			foundLevel3 = true
		}
	}

	if !foundRoot {
		t.Error("root.txt not found in snapshot (depth 0)")
	}
	if !foundLevel1 {
		t.Error("level1.txt not found in snapshot (depth 1)")
	}
	if foundLevel2 {
		t.Error("level2.txt found in snapshot but should be excluded (depth 2 > maxDepth 1)")
	}
	if foundLevel3 {
		t.Error("level3.txt found in snapshot but should be excluded (depth 3 > maxDepth 1)")
	}
}

func TestMaxDepth_Zero(t *testing.T) {
	dir := t.TempDir()

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "top.txt"), []byte("t"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("d"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New(
		Paths(dir),
		MaxDepth(0),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	time.Sleep(testPoll * 3)

	snap := w.Snapshot()

	foundTop := false
	foundDeep := false
	for path := range snap {
		switch filepath.Base(path) {
		case "top.txt":
			foundTop = true
		case "deep.txt":
			foundDeep = true
		}
	}

	if !foundTop {
		t.Error("top.txt should be in snapshot at depth 0")
	}
	if foundDeep {
		t.Error("deep.txt should not be in snapshot with MaxDepth(0)")
	}
}

func TestSnapshot(t *testing.T) {
	dir := t.TempDir()

	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(f1, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New(
		Paths(dir),
		PollInterval(testPoll),
		Debounce(testDebounce),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Before starting, snapshot should be empty.
	snap := w.Snapshot()
	if len(snap) != 0 {
		t.Errorf("expected empty snapshot before Start, got %d entries", len(snap))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	time.Sleep(testPoll * 3)

	snap = w.Snapshot()
	if len(snap) != 2 {
		t.Errorf("expected 2 entries in snapshot, got %d", len(snap))
	}

	// Verify returned map is a copy by modifying it.
	for k := range snap {
		delete(snap, k)
	}
	snap2 := w.Snapshot()
	if len(snap2) != 2 {
		t.Error("modifying returned snapshot affected watcher internal state")
	}
}
