package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Engine struct {
	skillsDir string
	skills    []Skill
}

type Skill struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Prompt      string    `json:"prompt"`
	Output      string    `json:"output,omitempty"`
	Files       []string  `json:"files,omitempty"`
	UsageCount  int       `json:"usage_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewEngine() (*Engine, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".delta", "skills")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	e := &Engine{skillsDir: dir}
	e.load()
	return e, nil
}

func (e *Engine) load() {
	entries, err := os.ReadDir(e.skillsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(e.skillsDir, entry.Name()))
		if err != nil {
			continue
		}
		var skill Skill
		if err := json.Unmarshal(data, &skill); err != nil {
			continue
		}
		e.skills = append(e.skills, skill)
	}
}

func (e *Engine) Save(name, description, prompt string, tags []string) (*Skill, error) {
	id := fmt.Sprintf("skill-%d", time.Now().UnixNano())
	skill := Skill{
		ID:          id,
		Name:        name,
		Description: description,
		Tags:        tags,
		Prompt:      prompt,
		UsageCount:  0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(e.skillsDir, id+".json"), data, 0600); err != nil {
		return nil, err
	}

	e.skills = append(e.skills, skill)
	return &skill, nil
}

func (e *Engine) Find(query string) []Skill {
	query = strings.ToLower(query)
	var results []Skill

	for _, skill := range e.skills {
		score := 0
		if strings.Contains(strings.ToLower(skill.Name), query) {
			score += 3
		}
		if strings.Contains(strings.ToLower(skill.Description), query) {
			score += 2
		}
		for _, tag := range skill.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				score++
			}
		}
		if strings.Contains(strings.ToLower(skill.Prompt), query) {
			score++
		}
		if score > 0 {
			skill.UsageCount = score
			results = append(results, skill)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].UsageCount > results[j].UsageCount
	})

	return results
}

func (e *Engine) List(limit int) []Skill {
	if limit <= 0 || limit > len(e.skills) {
		limit = len(e.skills)
	}
	sorted := make([]Skill, len(e.skills))
	copy(sorted, e.skills)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UsageCount > sorted[j].UsageCount
	})
	return sorted[:limit]
}

func (e *Engine) Get(id string) (*Skill, error) {
	for _, skill := range e.skills {
		if skill.ID == id {
			return &skill, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found", id)
}

func (e *Engine) RecordUsage(id string) error {
	for i := range e.skills {
		if e.skills[i].ID == id {
			e.skills[i].UsageCount++
			e.skills[i].UpdatedAt = time.Now()
			return e.saveSkill(e.skills[i])
		}
	}
	return fmt.Errorf("skill %q not found", id)
}

func (e *Engine) Delete(id string) error {
	for i, skill := range e.skills {
		if skill.ID == id {
			e.skills = append(e.skills[:i], e.skills[i+1:]...)
			path := filepath.Join(e.skillsDir, id+".json")
			os.Remove(path)
			return nil
		}
	}
	return fmt.Errorf("skill %q not found", id)
}

func (e *Engine) Count() int {
	return len(e.skills)
}

func (e *Engine) saveSkill(skill Skill) error {
	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.skillsDir, skill.ID+".json"), data, 0600)
}
