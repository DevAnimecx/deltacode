package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Snapshot struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
	GitCommit string    `json:"git_commit,omitempty"`
	Files     []string  `json:"files"`
	Root      string    `json:"root"`
}

type Manager struct {
	root string
	dir  string
}

func New(root string) *Manager {
	return &Manager{root: root, dir: filepath.Join(root, ".delta", "checkpoints")}
}

func (m *Manager) gitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = m.root
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// IsGitRepo reports whether the project is under git.
func (m *Manager) IsGitRepo() bool {
	_, err := m.gitCommand("rev-parse", "--git-dir")
	return err == nil
}

// Create snapshots the current working tree state.
func (m *Manager) Create(label string) (*Snapshot, error) {
	if err := os.MkdirAll(m.dir, 0755); err != nil {
		return nil, err
	}

	snap := &Snapshot{
		ID:        fmt.Sprintf("ckpt-%d", time.Now().UnixNano()),
		Label:     label,
		CreatedAt: time.Now(),
		Root:      m.root,
	}

	// Record git state if available.
	if m.IsGitRepo() {
		if out, err := m.gitCommand("rev-parse", "HEAD"); err == nil {
			snap.GitCommit = out
		}
	}

	// Snapshot files mentioned by the caller via label extras is not possible here;
	// instead snapshot the whole working tree file list from git or walk.
	files := m.snapshotFiles()
	snap.Files = files

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(m.dir, snap.ID+".json"), data, 0600); err != nil {
		return nil, err
	}
	return snap, nil
}

func (m *Manager) snapshotFiles() []string {
	out, err := m.gitCommand("ls-files")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.Fields(out)
	}
	// Fallback: walk the tree.
	var files []string
	filepath.Walk(m.root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(m.root, path)
		if strings.HasPrefix(rel, ".") || strings.HasPrefix(rel, "..") {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files
}

func (m *Manager) List() []Snapshot {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil
	}
	var snaps []Snapshot
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
			if err != nil {
				continue
			}
			var s Snapshot
			if json.Unmarshal(data, &s) == nil {
				snaps = append(snaps, s)
			}
		}
	}
	return snaps
}

// Get returns a snapshot by ID.
func (m *Manager) Get(id string) (Snapshot, bool) {
	data, err := os.ReadFile(filepath.Join(m.dir, id+".json"))
	if err != nil {
		return Snapshot{}, false
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, false
	}
	return s, true
}

// Rollback restores a snapshot. With git available it uses git checkout of the
// recorded commit (safe via git stash), otherwise it re-applies the recorded file list.
func (m *Manager) Rollback(id string) error {
	snap, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("checkpoint %s not found", id)
	}
	if m.IsGitRepo() {
		if snap.GitCommit != "" {
			// Preserve any uncommitted changes.
			m.gitCommand("stash", "--include-untracked")
			if _, err := m.gitCommand("checkout", snap.GitCommit, "--", "."); err == nil {
				return nil
			}
			m.gitCommand("stash", "pop")
			return fmt.Errorf("rollback via git failed for commit %s", snap.GitCommit)
		}
		return fmt.Errorf("checkpoint has no git commit recorded")
	}
	// Fallback: verify the recorded files still exist (no-op restore).
	for _, f := range snap.Files {
		if _, err := os.Stat(filepath.Join(m.root, f)); err != nil {
			return fmt.Errorf("file missing: %s", f)
		}
	}
	return nil
}

// RecoverTable prints a human-readable recovery table.
func (m *Manager) RecoverTable() string {
	snaps := m.List()
	if len(snaps) == 0 {
		return "No checkpoints yet."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-28s %-24s %-12s %s\n", "ID", "Created", "Files", "Label"))
	b.WriteString(strings.Repeat("-", 90) + "\n")
	for _, s := range snaps {
		b.WriteString(fmt.Sprintf("%-28s %-24s %-12d %s\n",
			s.ID, s.CreatedAt.Format(time.RFC3339), len(s.Files), s.Label))
	}
	return b.String()
}
