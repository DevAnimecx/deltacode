package repointel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DevAnimecx/deltacode/internal/symbols"
)

// FileState tracks a watched file's fingerprint.
type FileState struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Mtime    time.Time `json:"mtime"`
	Hash     string    `json:"hash,omitempty"`
	Indexed  bool      `json:"indexed"`
	Language string    `json:"language,omitempty"`
}

// WatchKind classifies watch sources.
type WatchKind string

const (
	KindSource     WatchKind = "source"
	KindGit        WatchKind = "git"
	KindDependency WatchKind = "dependency"
	KindBuild      WatchKind = "build"
)

// Change is a detected modification.
type Change struct {
	Path     string
	Kind     WatchKind
	Added    bool
	Removed  bool
	Modified bool
}

// WatchConfig controls the watcher.
type WatchConfig struct {
	Root         string
	PollInterval time.Duration
	Index        bool // re-index changed files
	MaxFileSize  int64
	SkipDirs     []string
}

// Store persists the incremental index state.
type Store struct {
	Version    int                   `json:"version"`
	Updated    time.Time             `json:"updated"`
	Files      map[string]*FileState `json:"files"`
	IndexCount int                   `json:"index_count"`
}

func (s *Store) save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func loadStore(path string) *Store {
	s := &Store{Files: map[string]*FileState{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, s)
	}
	if s.Files == nil {
		s.Files = map[string]*FileState{}
	}
	return s
}

// Watcher polls the repository and reports changes incrementally.
type Watcher struct {
	mu        sync.Mutex
	cfg       WatchConfig
	state     *Store
	indexer   *symbols.Indexer
	statePath string
	stop      chan struct{}
	changed   chan Change
	errors    chan error
	running   bool
}

// NewWatcher creates a watcher for root; state persists at ~/.delta/repointel.json.
func NewWatcher(cfg WatchConfig) *Watcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 3 * time.Second
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 2 << 20
	}
	if len(cfg.SkipDirs) == 0 {
		cfg.SkipDirs = []string{".git", "node_modules", "vendor", ".venv", "dist", "build", "__pycache__", ".cache", "bin", "obj"}
	}
	home, _ := os.UserHomeDir()
	statePath := filepath.Join(home, ".delta", "repointel.json")
	if cfg.Root != "" {
		statePath = filepath.Join(cfg.Root, ".delta-repointel.json")
	}
	w := &Watcher{
		cfg:       cfg,
		state:     loadStore(statePath),
		indexer:   symbols.NewIndexer(cfg.Root),
		statePath: statePath,
		stop:      make(chan struct{}),
		changed:   make(chan Change, 256),
		errors:    make(chan error, 16),
	}
	return w
}

// Changed returns the change notification channel.
func (w *Watcher) Changed() <-chan Change { return w.changed }

// Errors returns watcher errors.
func (w *Watcher) Errors() <-chan error { return w.errors }

// Start begins polling. Only the first start takes effect.
func (w *Watcher) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.running = true
	go w.loop()
}

// Stop halts polling and persists state.
func (w *Watcher) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	w.persist()
	w.mu.Lock()
	w.running = false
	w.mu.Unlock()
}

func (w *Watcher) loop() {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()
	w.scanOnce()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.scanOnce()
		}
	}
}

// skip returns true for ignored paths.
func (w *Watcher) skip(path string) bool {
	if w.statePath != "" {
		if ap, err := filepath.Abs(path); err == nil {
			if sp, err2 := filepath.Abs(w.statePath); err2 == nil && ap == sp {
				return true
			}
		}
	}
	name := filepath.Base(path)
	for _, d := range w.cfg.SkipDirs {
		if name == d || strings.HasPrefix(name, d+".") {
			return true
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".lock", ".tmp", ".log", ".exe", ".dll", ".so", ".dylib", ".png", ".jpg", ".gif", ".ico":
		return true
	}
	return false
}

// scanOnce compares the filesystem against the persisted snapshot.
func (w *Watcher) scanOnce() {
	seen := map[string]bool{}
	root := w.cfg.Root
	if root == "" {
		root = "."
	}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if w.skip(path) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if w.skip(path) {
			return nil
		}
		rel := path
		if root != "." {
			if r, rerr := filepath.Rel(root, path); rerr == nil {
				rel = r
			}
		}
		seen[rel] = true
		w.checkFile(rel, info)
		return nil
	})
	// Detect removals.
	w.mu.Lock()
	var removed []string
	for rel := range w.state.Files {
		if !seen[rel] {
			removed = append(removed, rel)
		}
	}
	for _, rel := range removed {
		old := w.state.Files[rel]
		delete(w.state.Files, rel)
		w.notify(Change{Path: rel, Kind: w.kindFor(old), Removed: true})
	}
	w.mu.Unlock()
	w.persist()
}

func (w *Watcher) kindFor(fs *FileState) WatchKind {
	ext := strings.ToLower(filepath.Ext(fs.Path))
	switch ext {
	case ".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".rs", ".java", ".c", ".cpp", ".cs", ".rb", ".php", ".swift", ".kt", ".scala":
		return KindSource
	}
	if fs.Path == "go.mod" || fs.Path == "go.sum" || fs.Path == "package.json" || fs.Path == "package-lock.json" ||
		fs.Path == "requirements.txt" || fs.Path == "pyproject.toml" || fs.Path == "Cargo.toml" || fs.Path == "Cargo.lock" ||
		fs.Path == "pom.xml" || fs.Path == "build.gradle" || fs.Path == "Gemfile" || fs.Path == "Gemfile.lock" {
		return KindDependency
	}
	switch fs.Path {
	case ".gitignore", "Makefile", "Dockerfile", "docker-compose.yml", ".github/workflows":
		return KindBuild
	}
	if strings.HasPrefix(filepath.Base(fs.Path), ".") {
		return KindGit
	}
	return KindSource
}

func (w *Watcher) checkFile(rel string, info os.FileInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()
	prev, exists := w.state.Files[rel]
	if !exists {
		fs := w.snapshotLocked(rel, info)
		w.state.Files[rel] = fs
		w.notify(Change{Path: rel, Kind: w.kindFor(fs), Added: true})
		w.reindexLocked(fs)
		return
	}
	if prev.Size == info.Size() && prev.Mtime.Equal(info.ModTime()) {
		return
	}
	fs := w.snapshotLocked(rel, info)
	w.state.Files[rel] = fs
	w.notify(Change{Path: rel, Kind: w.kindFor(fs), Modified: true})
	w.reindexLocked(fs)
}

func (w *Watcher) snapshotLocked(rel string, info os.FileInfo) *FileState {
	fs := &FileState{
		Path:    rel,
		Size:    info.Size(),
		Mtime:   info.ModTime(),
		Indexed: true,
	}
	if info.Size() <= w.cfg.MaxFileSize && isIndexable(rel) {
		if data, err := os.ReadFile(filepath.Join(w.cfg.Root, rel)); err == nil {
			sum := sha256.Sum256(data)
			fs.Hash = hex.EncodeToString(sum[:])
		}
	}
	fs.Language = strings.TrimPrefix(filepath.Ext(rel), ".")
	return fs
}

func (w *Watcher) reindexLocked(fs *FileState) {
	if !w.cfg.Index || !isIndexable(fs.Path) {
		return
	}
	full := filepath.Join(w.cfg.Root, fs.Path)
	w.indexer.Graph().RemoveSymbolsByFile(full)
	if err := w.indexer.IndexFile(full); err != nil {
		return
	}
	w.state.IndexCount = w.indexer.Graph().Count()
}

func (w *Watcher) notify(c Change) {
	select {
	case w.changed <- c:
	default:
	}
}

func (w *Watcher) persist() {
	w.mu.Lock()
	state := w.state
	w.mu.Unlock()
	state.Updated = time.Now()
	_ = state.save(w.statePath)
}

func isIndexable(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".rs", ".java", ".c", ".cpp", ".cs", ".rb", ".php", ".swift", ".kt", ".scala", ".md", ".json", ".yaml", ".yml", ".toml", ".mod", ".sum":
		return true
	}
	return false
}

// Indexer exposes the incremental symbol index.
func (w *Watcher) Indexer() *symbols.Indexer { return w.indexer }

// Snapshot returns the current file state.
func (w *Watcher) Snapshot() []FileState {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]FileState, 0, len(w.state.Files))
	for _, fs := range w.state.Files {
		out = append(out, *fs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// IndexedCount returns the number of indexed symbols.
func (w *Watcher) IndexedCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state.IndexCount
}
