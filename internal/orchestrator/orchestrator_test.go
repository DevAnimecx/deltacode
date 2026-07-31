package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

func testProviders() []models.ProviderConfig {
	return []models.ProviderConfig{
		{Name: "openai", Type: models.ProviderOpenAI, BaseURL: "http://127.0.0.1:1/v1", Models: []string{"gpt-4o-mini"}},
		{Name: "deepseek", Type: models.ProviderDeepSeek, BaseURL: "http://127.0.0.1:1/v1", Models: []string{"deepseek-chat"}},
		{Name: "ollama", Type: models.ProviderOllama, BaseURL: "http://127.0.0.1:1", Models: []string{"llama3"}},
	}
}

func TestEndpoints(t *testing.T) {
	r := NewRouter(testProviders(), RouteBalanced, nil)
	eps := r.Endpoints()
	if len(eps) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(eps))
	}
}

func TestSelect(t *testing.T) {
	r := NewRouter(testProviders(), RouteBalanced, nil)
	e, ok := r.Select(PhaseCode)
	if !ok || e.ProviderName == "" {
		t.Fatal("select failed")
	}
	chain := r.FallbackChain(PhaseCode)
	if len(chain) != 3 {
		t.Fatalf("fallback chain: %d", len(chain))
	}
}

func TestPhaseDefaultPin(t *testing.T) {
	r := NewRouter(testProviders(), RouteBalanced, nil)
	r.SetPhaseDefault(PhasePlan, "ollama:llama3")
	e, _ := r.Select(PhasePlan)
	if e.ProviderName != "ollama" || e.Model != "llama3" {
		t.Fatalf("phase pin not honored: %+v", e)
	}
	// Other phases unaffected.
	e2, _ := r.Select(PhaseCode)
	if e2.ProviderName == "ollama" {
		t.Fatal("pin leaked to other phase")
	}
}

func TestCallFailsOver(t *testing.T) {
	// All endpoints point at a dead address; the router should exhaust the
	// chain and return an error (proving fail-over attempted).
	r := NewRouter(testProviders(), RouteBalanced, nil)
	res := r.Call(context.Background(), PhaseCode, models.ChatRequest{Messages: []models.Message{{Role: models.RoleUser, Content: "hi"}}})
	if res.Err == nil {
		t.Fatal("expected error from dead endpoints")
	}
	if res.Attempts != 3 {
		t.Fatalf("expected 3 attempts (fail-over), got %d", res.Attempts)
	}
}

func TestConfidence(t *testing.T) {
	if Confidence(nil) != 0 {
		t.Fatal("nil confidence should be 0")
	}
	empty := Confidence(&models.ChatResponse{})
	if empty != 0 {
		t.Fatal("empty confidence should be 0")
	}
	rich := Confidence(&models.ChatResponse{Message: models.Message{
		Content: "Here is the implementation:\n\n```go\npackage main\n```\n\nDone.",
	}})
	if rich <= 0.5 {
		t.Fatalf("rich response should score high, got %v", rich)
	}
}

func TestValidateStructured(t *testing.T) {
	resp := &models.ChatResponse{Message: models.Message{
		Content: "```json\n{\"tasks\": [{\"id\": \"1\"}]}\n```",
	}}
	var out struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	ok, err := ValidateStructured(resp, &out)
	if err != nil || !ok {
		t.Fatalf("valid json rejected: %v", err)
	}
	if len(out.Tasks) != 1 || out.Tasks[0].ID != "1" {
		t.Fatalf("parsed wrong: %+v", out)
	}
	bad := &models.ChatResponse{Message: models.Message{Content: "not json"}}
	if _, err := ValidateStructured(bad, &out); err == nil {
		t.Fatal("invalid json accepted")
	}
}

func TestVoteMajority(t *testing.T) {
	r := NewRouter(testProviders(), RouteBalanced, nil)
	// Dead endpoints -> no responses; must not panic and should report totals.
	vr := r.Vote(context.Background(), PhaseCode, models.ChatRequest{}, 2)
	if vr.Total != 2 || vr.Winner != nil {
		t.Fatalf("vote with dead endpoints: %+v", vr)
	}
	if len(vr.Responses) != 0 {
		t.Fatal("expected no responses")
	}
}

func TestVoteSelectsMajority(t *testing.T) {
	// Simulate the agreement logic directly.
	a := &models.ChatResponse{Message: models.Message{Content: "answer A"}}
	b := &models.ChatResponse{Message: models.Message{Content: "answer A"}}
	c := &models.ChatResponse{Message: models.Message{Content: "answer C"}}
	votes := []*models.ChatResponse{a, b, c}
	best := votes[0]
	bestCount := 0
	for _, x := range votes {
		count := 0
		for _, y := range votes {
			if strings.TrimSpace(x.Message.Content) == strings.TrimSpace(y.Message.Content) {
				count++
			}
		}
		if count > bestCount {
			best, bestCount = x, count
		}
	}
	if best != a || bestCount != 2 {
		t.Fatalf("majority not detected: %+v %d", best, bestCount)
	}
}
