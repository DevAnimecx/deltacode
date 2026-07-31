package repointel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	w := NewWatcher(WatchConfig{Root: dir, PollInterval: 100 * time.Millisecond, Index: false})
	defer w.Stop()

	events := make(chan Change, 20)
	go func() {
		for c := range w.Changed() {
			events <- c
		}
	}()
	next := func(desc string) Change {
		select {
		case c := <-events:
			return c
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for %s", desc)
			return Change{}
		}
	}

	w.Start()

	// Create a file.
	file := filepath.Join(dir, "hello.txt")
	os.WriteFile(file, []byte("one"), 0644)
	if c := next("add event"); !c.Added || c.Path != "hello.txt" {
		t.Fatalf("added: %+v", c)
	}

	// Modify it (wait for mtime resolution).
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(file, []byte("two"), 0644)
	if c := next("modify event"); !c.Modified || c.Path != "hello.txt" {
		t.Fatalf("modified: %+v", c)
	}

	// Remove it.
	os.Remove(file)
	if c := next("remove event"); !c.Removed || c.Path != "hello.txt" {
		t.Fatalf("removed: %+v", c)
	}

	snap := w.Snapshot()
	if len(snap) != 0 {
		t.Fatalf("expected empty snapshot, got %d files", len(snap))
	}
}

func TestWatcherSkipsDirs(t *testing.T) {
	dir := t.TempDir()
	w := NewWatcher(WatchConfig{Root: dir, PollInterval: 50 * time.Millisecond, Index: false})
	defer w.Stop()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0644)
	w.Start()
	time.Sleep(300 * time.Millisecond)
	snap := w.Snapshot()
	if len(snap) != 0 {
		t.Fatalf("skipped dirs leaked into snapshot: %+v", snap)
	}
}

func TestIncrementalIndex(t *testing.T) {
	dir := t.TempDir()
	w := NewWatcher(WatchConfig{Root: dir, PollInterval: 80 * time.Millisecond, Index: true})
	defer w.Stop()

	w.Start()
	file := filepath.Join(dir, "lib.go")
	os.WriteFile(file, []byte("package lib\n\nfunc Alpha() {}\n"), 0644)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if w.IndexedCount() >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if w.IndexedCount() < 1 {
		t.Fatal("index never picked up the new file")
	}
	first := w.IndexedCount()

	// Rewrite the same file with a new symbol; incremental reindex must
	// remove the old symbol and index the new one.
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(file, []byte("package lib\n\nfunc Beta() {}\n"), 0644)
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if w.IndexedCount() != 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = first
	syms := w.Indexer().Graph().GetSymbolsByFile(file)
	if len(syms) != 1 || syms[0].Name != "Beta" {
		t.Fatalf("incremental reindex wrong: %+v", syms)
	}
}

func TestStorePersists(t *testing.T) {
	dir := t.TempDir()
	home := os.Getenv("HOME")
	t.Setenv("HOME", t.TempDir())
	defer t.Setenv("HOME", home)

	w := NewWatcher(WatchConfig{Root: dir, PollInterval: 50 * time.Millisecond, Index: false})
	w.Start()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(w.Snapshot()) == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(w.Snapshot()) != 1 {
		t.Fatal("file not snapshotted")
	}
	w.Stop()

	// Reopen: state should reload from disk.
	w2 := NewWatcher(WatchConfig{Root: dir, PollInterval: 50 * time.Millisecond, Index: false})
	defer w2.Stop()
	snap := w2.Snapshot()
	if len(snap) != 1 || snap[0].Path != "a.txt" {
		t.Fatalf("state not restored: %+v", snap)
	}
}

func TestWatchKind(t *testing.T) {
	dir := t.TempDir()
	w := NewWatcher(WatchConfig{Root: dir, PollInterval: 50 * time.Millisecond, Index: false})
	defer w.Stop()
	w.Start()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	seen := map[string]WatchKind{}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(seen) < 2 {
		select {
		case c := <-w.Changed():
			seen[c.Path] = c.Kind
		case <-time.After(50 * time.Millisecond):
		}
	}
	if seen["go.mod"] != KindDependency {
		t.Fatalf("go.mod kind: %v", seen["go.mod"])
	}
	if seen["main.go"] != KindSource {
		t.Fatalf("main.go kind: %v", seen["main.go"])
	}
}
