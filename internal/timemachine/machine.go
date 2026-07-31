package timemachine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Checkpoint struct {
	ID        string     `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	Label     string     `json:"label"`
	Files     []FileSnap `json:"files"`
	Prompt    string     `json:"prompt"`
	Response  string     `json:"response"`
	Model     string     `json:"model"`
	Provider  string     `json:"provider"`
}

type FileSnap struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Action  string `json:"action"`
}

type Machine struct {
	dir string
}

func New() (*Machine, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".delta", "timemachine")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Machine{dir: dir}, nil
}

func (tm *Machine) Save(label, prompt, response, model, provider string) (*Checkpoint, error) {
	id := fmt.Sprintf("cp-%d", time.Now().UnixNano())
	cp := &Checkpoint{
		ID:        id,
		Timestamp: time.Now(),
		Label:     label,
		Prompt:    prompt,
		Response:  response,
		Model:     model,
		Provider:  provider,
	}

	// Capture current file state
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || e.IsDir() {
			continue
		}
		data, err := os.ReadFile(e.Name())
		if err != nil {
			continue
		}
		cp.Files = append(cp.Files, FileSnap{
			Path:    e.Name(),
			Content: string(data),
			Action:  "snapshot",
		})
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(tm.dir, id+".json"), data, 0600); err != nil {
		return nil, err
	}

	return cp, nil
}

func (tm *Machine) List(limit int) ([]Checkpoint, error) {
	entries, err := os.ReadDir(tm.dir)
	if err != nil {
		return nil, err
	}

	var checkpoints []Checkpoint
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tm.dir, e.Name()))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		checkpoints = append(checkpoints, cp)
	}

	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.After(checkpoints[j].Timestamp)
	})

	if limit > 0 && len(checkpoints) > limit {
		checkpoints = checkpoints[:limit]
	}
	return checkpoints, nil
}

func (tm *Machine) Get(id string) (*Checkpoint, error) {
	data, err := os.ReadFile(filepath.Join(tm.dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("checkpoint %q not found", id)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func (tm *Machine) Undo(id string) error {
	cp, err := tm.Get(id)
	if err != nil {
		return err
	}

	for _, f := range cp.Files {
		if f.Action == "snapshot" {
			if err := os.WriteFile(f.Path, []byte(f.Content), 0644); err != nil {
				return fmt.Errorf("failed to restore %s: %w", f.Path, err)
			}
		}
	}

	fmt.Printf("Restored checkpoint %q from %s\n", cp.Label, cp.Timestamp.Format(time.RFC822))
	return nil
}

func (tm *Machine) Replay(id string) error {
	cp, err := tm.Get(id)
	if err != nil {
		return err
	}

	fmt.Printf("=== Replay: %s ===\n", cp.Label)
	fmt.Printf("Time: %s\n", cp.Timestamp.Format(time.RFC822))
	fmt.Printf("Model: %s (%s)\n", cp.Model, cp.Provider)
	fmt.Printf("Prompt: %s\n", cp.Prompt)
	fmt.Printf("Response:\n%s\n", cp.Response)
	fmt.Println("=== End Replay ===")
	return nil
}

func (tm *Machine) Compare(id1, id2 string) error {
	cp1, err := tm.Get(id1)
	if err != nil {
		return err
	}
	cp2, err := tm.Get(id2)
	if err != nil {
		return err
	}

	fmt.Printf("Comparing %q vs %q\n", cp1.Label, cp2.Label)
	fmt.Println(strings.Repeat("─", 50))

	// Compare file states
	files1 := make(map[string]string)
	files2 := make(map[string]string)
	for _, f := range cp1.Files {
		files1[f.Path] = f.Content
	}
	for _, f := range cp2.Files {
		files2[f.Path] = f.Content
	}

	for path := range files1 {
		if _, exists := files2[path]; !exists {
			fmt.Printf("- %s (removed)\n", path)
		} else if files1[path] != files2[path] {
			fmt.Printf("~ %s (changed)\n", path)
		}
	}
	for path := range files2 {
		if _, exists := files1[path]; !exists {
			fmt.Printf("+ %s (added)\n", path)
		}
	}

	return nil
}

func (tm *Machine) Branch(id, branchName string) error {
	cp, err := tm.Get(id)
	if err != nil {
		return err
	}

	branchCp := *cp
	branchCp.ID = fmt.Sprintf("branch-%s-%d", branchName, time.Now().UnixNano())
	branchCp.Label = fmt.Sprintf("branch/%s: %s", branchName, cp.Label)

	data, _ := json.MarshalIndent(branchCp, "", "  ")
	filename := fmt.Sprintf("branch-%s-%d.json", branchName, time.Now().UnixNano())
	if err := os.WriteFile(filepath.Join(tm.dir, filename), data, 0600); err != nil {
		return err
	}

	fmt.Printf("Created branch %q at checkpoint %q\n", branchName, cp.Label)
	return nil
}

func (tm *Machine) Log(limit int) {
	checkpoints, err := tm.List(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if len(checkpoints) == 0 {
		fmt.Println("No checkpoints yet. Run `delta checkpoint` to create one.")
		return
	}

	fmt.Println("Δ Time Machine — Checkpoints")
	fmt.Println(strings.Repeat("─", 60))
	for _, cp := range checkpoints {
		label := cp.Label
		if len(label) > 40 {
			label = label[:40] + "..."
		}
		fmt.Printf("  %-25s %s  %s\n", cp.ID, cp.Timestamp.Format("2006-01-02 15:04"), label)
	}
}
