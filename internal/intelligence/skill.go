package intelligence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SkillEngine struct {
	skillsDir string
	skills    []Skill
}

type Skill struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Version      int             `json:"version"`
	Tags         []string        `json:"tags"`
	Triggers     []string        `json:"triggers"`
	Plan         json.RawMessage `json:"plan,omitempty"`
	Output       string          `json:"output,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
	UsageCount   int             `json:"usage_count"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func NewSkillEngine() *SkillEngine {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".delta", "skills")
	os.MkdirAll(dir, 0700)

	e := &SkillEngine{skillsDir: dir}
	e.load()
	return e
}

func (e *SkillEngine) load() {
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

func (e *SkillEngine) Save(name, description string, tags, triggers []string, planJSON json.RawMessage) (*Skill, error) {
	id := fmt.Sprintf("skill-%d", time.Now().UnixNano())
	skill := Skill{
		ID:          id,
		Name:        name,
		Description: description,
		Version:     1,
		Tags:        tags,
		Triggers:    triggers,
		Plan:        planJSON,
		UsageCount:  1,
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

func (e *SkillEngine) Find(query string) []Skill {
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
		for _, trigger := range skill.Triggers {
			if strings.Contains(strings.ToLower(trigger), query) {
				score += 2
			}
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

func (e *SkillEngine) Get(id string) (*Skill, error) {
	for _, skill := range e.skills {
		if skill.ID == id {
			return &skill, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found", id)
}

func (e *SkillEngine) Update(id string, plan json.RawMessage, output string) error {
	for i := range e.skills {
		if e.skills[i].ID == id {
			e.skills[i].Plan = plan
			e.skills[i].Output = output
			e.skills[i].Version++
			e.skills[i].UpdatedAt = time.Now()
			return e.saveSkill(e.skills[i])
		}
	}
	return fmt.Errorf("skill %q not found", id)
}

func (e *SkillEngine) RecordUsage(id string) {
	for i := range e.skills {
		if e.skills[i].ID == id {
			e.skills[i].UsageCount++
			e.skills[i].UpdatedAt = time.Now()
			e.saveSkill(e.skills[i])
			return
		}
	}
}

func (e *SkillEngine) Delete(id string) error {
	for i, skill := range e.skills {
		if skill.ID == id {
			e.skills = append(e.skills[:i], e.skills[i+1:]...)
			os.Remove(filepath.Join(e.skillsDir, id+".json"))
			return nil
		}
	}
	return fmt.Errorf("skill %q not found", id)
}

func (e *SkillEngine) List(limit int) []Skill {
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

func (e *SkillEngine) Count() int {
	return len(e.skills)
}

func (e *SkillEngine) Export(id string) ([]byte, error) {
	skill, err := e.Get(id)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(skill, "", "  ")
}

func (e *SkillEngine) Import(data []byte) (*Skill, error) {
	var skill Skill
	if err := json.Unmarshal(data, &skill); err != nil {
		return nil, err
	}
	skill.ID = fmt.Sprintf("skill-%d", time.Now().UnixNano())
	skill.CreatedAt = time.Now()
	skill.UpdatedAt = time.Now()

	if err := os.WriteFile(filepath.Join(e.skillsDir, skill.ID+".json"), data, 0600); err != nil {
		return nil, err
	}
	e.skills = append(e.skills, skill)
	return &skill, nil
}

func (e *SkillEngine) Learn(goal string, plan interface{}) {
	name := generateSkillName(goal)
	var planJSON json.RawMessage
	if b, err := json.Marshal(plan); err == nil {
		planJSON = b
	}
	if _, err := e.Save(name, "Auto-learned from: "+goal, extractTags(goal), extractTags(goal), planJSON); err != nil {
		return
	}
}

func (e *SkillEngine) saveSkill(skill Skill) error {
	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.skillsDir, skill.ID+".json"), data, 0600)
}

func generateSkillName(text string) string {
	words := strings.Fields(text)
	if len(words) > 6 {
		return strings.Join(words[:6], "-")
	}
	return strings.Join(words, "-")
}

func extractTags(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var tags []string
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) > 3 && !seen[w] {
			tags = append(tags, w)
			seen[w] = true
		}
		if len(tags) >= 5 {
			break
		}
	}
	return tags
}
