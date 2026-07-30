package explain

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/delta-code/cli/internal/provider"
	"github.com/delta-code/cli/pkg/models"
)

type ChangePlan struct {
	Files    []string `json:"files"`
	Summary  string   `json:"summary"`
	Risk     string   `json:"risk"`
	Estimate string   `json:"estimate"`
	Cost     string   `json:"cost"`
	Affected []string `json:"affected"`
}

func PlanChanges(p models.ProviderConfig, model, prompt string) (*ChangePlan, error) {
	prov, err := provider.NewProvider(p)
	if err != nil {
		return nil, err
	}

	resp, err := prov.Chat(models.ChatRequest{
		Model: model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: `You are a planning assistant. Given a user request, analyze what changes are needed.
Return JSON with these fields:
- files: list of files that will be modified
- summary: 1-2 sentence summary of the change
- risk: "low", "medium", or "high"
- estimate: estimated time like "5 minutes"
- cost: estimated cost like "~$0.01"
- affected: list of functions/components affected
Return ONLY valid JSON.`},
			{Role: models.RoleUser, Content: prompt},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return nil, err
	}

	plan := &ChangePlan{}
	content := resp.Message.Content
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if err := json.Unmarshal([]byte(content), plan); err != nil {
		plan.Summary = content
		plan.Risk = "unknown"
	}
	if plan.Files == nil {
		plan.Files = getChangedFiles()
	}
	if plan.Affected == nil {
		plan.Affected = []string{}
	}

	return plan, nil
}

func ConfirmPlan(plan *ChangePlan) bool {
	fmt.Println("\nΔ Proposed Changes:")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  Summary:  %s\n", plan.Summary)
	fmt.Printf("  Files:    %s\n", strings.Join(plan.Files, ", "))
	fmt.Printf("  Risk:     %s\n", plan.Risk)
	fmt.Printf("  Estimate: %s\n", plan.Estimate)
	fmt.Printf("  Cost:     %s\n", plan.Cost)
	if len(plan.Affected) > 0 {
		fmt.Printf("  Affected: %s\n", strings.Join(plan.Affected, ", "))
	}
	fmt.Println(strings.Repeat("─", 50))
	fmt.Print("Apply changes? [Y/n]: ")

	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}

func getChangedFiles() []string {
	out, err := exec.Command("git", "diff", "--name-only").Output()
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, f := range lines {
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}
