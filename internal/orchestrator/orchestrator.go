package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/internal/telemetry"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

// Phase names the orchestrator can route differently.
type Phase string

const (
	PhasePlan   Phase = "planning"
	PhaseCode   Phase = "codegen"
	PhaseReview Phase = "review"
	PhaseTests  Phase = "testgen"
	PhaseDocs   Phase = "docs"
)

// RoutingMode selects the routing policy.
type RoutingMode string

const (
	RouteLatency   RoutingMode = "latency"
	RouteCost      RoutingMode = "cost"
	RouteConfidence RoutingMode = "confidence"
	RouteBalanced  RoutingMode = "balanced"
	RouteDefault   RoutingMode = "default" // user's default provider
)

// Endpoint is a provider+model pair that can serve a call.
type Endpoint struct {
	ProviderName string
	Model        string
	ProviderCfg  models.ProviderConfig
}

// Router decides which endpoint serves a phase.
type Router struct {
	mu      sync.RWMutex
	configs map[string]models.ProviderConfig // by provider name
	defaults map[string]string               // phase -> "provider:model"
	mode     RoutingMode
	tele     *telemetry.Store
}

// NewRouter builds a router from configured providers.
func NewRouter(providers []models.ProviderConfig, mode RoutingMode, tele *telemetry.Store) *Router {
	r := &Router{
		configs:  map[string]models.ProviderConfig{},
		defaults: map[string]string{},
		mode:     mode,
		tele:     tele,
	}
	for _, p := range providers {
		r.configs[p.Name] = p
	}
	return r
}

// SetMode changes the routing policy.
func (r *Router) SetMode(m RoutingMode) { r.mode = m }

// Mode returns the current routing mode.
func (r *Router) Mode() RoutingMode { return r.mode }

// SetPhaseDefault pins a phase to a specific provider:model.
func (r *Router) SetPhaseDefault(phase Phase, providerModel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaults[string(phase)] = providerModel
}

// Endpoints lists all usable provider:model pairs in preference order.
func (r *Router) Endpoints() []Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Endpoint
	for name, cfg := range r.configs {
		models_ := cfg.Models
		if len(models_) == 0 {
			models_ = []string{""} // provider default
		}
		for _, m := range models_ {
			out = append(out, Endpoint{ProviderName: name, Model: m, ProviderCfg: cfg})
		}
	}
	return out
}

// score computes a composite endpoint score for the given mode.
func (r *Router) score(e Endpoint, phase Phase) float64 {
	// Phase affinity: defaults pin to 1.0.
	r.mu.RLock()
	def := r.defaults[string(phase)]
	r.mu.RUnlock()
	if def == e.ProviderName+":"+e.Model {
		return 1.0
	}
	if r.tele == nil {
		return 0.5
	}
	score := 0.5
	if m, ok := r.tele.MetricFor(e.ProviderName, e.Model); ok {
		score = m.Score()
	}
	switch r.mode {
	case RouteLatency:
		return score // telemetry score already latency-weighted
	case RouteCost:
		return score
	case RouteConfidence:
		return score
	default:
		return score
	}
}

// Select returns the best endpoint for a phase.
func (r *Router) Select(phase Phase) (Endpoint, bool) {
	endpoints := r.Endpoints()
	if len(endpoints) == 0 {
		return Endpoint{}, false
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return r.score(endpoints[i], phase) > r.score(endpoints[j], phase)
	})
	return endpoints[0], true
}

// FallbackChain returns ordered endpoints for a phase (best first).
func (r *Router) FallbackChain(phase Phase) []Endpoint {
	endpoints := r.Endpoints()
	sort.Slice(endpoints, func(i, j int) bool {
		return r.score(endpoints[i], phase) > r.score(endpoints[j], phase)
	})
	return endpoints
}

// Call routes a chat request and fails over across providers.
type CallResult struct {
	Endpoint Endpoint
	Response *models.ChatResponse
	Attempts int
	Latency  time.Duration
	Err      error
}

// Call runs req through the best endpoint for phase, failing over.
func (r *Router) Call(ctx context.Context, phase Phase, req models.ChatRequest) CallResult {
	chain := r.FallbackChain(phase)
	if len(chain) == 0 {
		return CallResult{Err: fmt.Errorf("no providers configured")}
	}
	req.Stream = false
	start := time.Now()
	var lastErr error
	for i, e := range chain {
		p, err := provider.NewProvider(e.ProviderCfg)
		if err != nil {
			lastErr = err
			continue
		}
		req.Model = e.Model
		resp, err := p.Chat(req)
		lat := time.Since(start)
		if err == nil {
			return CallResult{Endpoint: e, Response: resp, Attempts: i + 1, Latency: lat}
		}
		lastErr = err
		if r.tele != nil {
			r.tele.RecordCall(e.ProviderName, e.Model, float64(lat.Milliseconds()), 0, 0, false, i)
		}
		select {
		case <-ctx.Done():
			return CallResult{Endpoint: e, Attempts: i + 1, Latency: lat, Err: ctx.Err()}
		default:
		}
	}
	return CallResult{Attempts: len(chain), Latency: time.Since(start), Err: lastErr}
}

// Vote runs the same request through n endpoints and returns the majority
// answer (by exact response equality) or the most confident response.
type VoteResult struct {
	Responses []*models.ChatResponse
	Winner    *models.ChatResponse
	Agreement int // 0..n how many agreed with winner
	Total     int
}

func (r *Router) Vote(ctx context.Context, phase Phase, req models.ChatRequest, n int) VoteResult {
	chain := r.FallbackChain(phase)
	if len(chain) > n {
		chain = chain[:n]
	}
	req.Stream = false
	var wg sync.WaitGroup
	results := make([]*models.ChatResponse, len(chain))
	for i, e := range chain {
		wg.Add(1)
		go func(i int, e Endpoint) {
			defer wg.Done()
			p, err := provider.NewProvider(e.ProviderCfg)
			if err != nil {
				return
			}
			r2 := req
			r2.Model = e.Model
			resp, err := p.Chat(r2)
			if err == nil {
				results[i] = resp
			}
		}(i, e)
	}
	wg.Wait()
	var collected []*models.ChatResponse
	for _, res := range results {
		if res != nil {
			collected = append(collected, res)
		}
	}
	vr := VoteResult{Responses: collected, Total: len(chain)}
	if len(collected) == 0 {
		return vr
	}
	// Majority by first response message group.
	best := collected[0]
	bestCount := 0
	for _, c := range collected {
		count := 0
		for _, o := range collected {
			if strings.TrimSpace(c.Message.Content) == strings.TrimSpace(o.Message.Content) {
				count++
			}
		}
		if count > bestCount {
			best, bestCount = c, count
		} else if count == bestCount && c.Message.Content != "" && len(c.Message.Content) > len(best.Message.Content) {
			best, bestCount = c, count
		}
	}
	vr.Winner = best
	vr.Agreement = bestCount
	return vr
}

// Confidence estimates how certain a response is:
// presence of structured markers, length, and self-consistency markers.
func Confidence(resp *models.ChatResponse) float64 {
	if resp == nil {
		return 0
	}
	content := strings.TrimSpace(resp.Message.Content)
	if content == "" {
		return 0
	}
	score := 0.4 // baseline: has content
	if strings.Contains(content, "```") {
		score += 0.2
	}
	if strings.Contains(content, "```json") || strings.Contains(content, "```go") ||
		strings.Contains(content, "```python") || strings.Contains(content, "```typescript") {
		score += 0.1
	}
	if len(content) > 200 {
		score += 0.1
	}
	// Self-consistency markers.
	lower := strings.ToLower(content)
	if strings.Contains(lower, "done") || strings.Contains(lower, "complete") {
		score += 0.1
	}
	if score > 1 {
		score = 1
	}
	return score
}

// ValidateStructured parses a JSON block from a response and reports validity.
func ValidateStructured(resp *models.ChatResponse, target any) (bool, error) {
	if resp == nil {
		return false, fmt.Errorf("empty response")
	}
	content := strings.TrimSpace(resp.Message.Content)
	if i := strings.Index(content, "```"); i != -1 {
		content = content[i+3:]
		if j := strings.Index(content, "\n"); j != -1 {
			content = content[j+1:]
		}
		if j := strings.LastIndex(content, "```"); j != -1 {
			content = content[:j]
		}
	}
	if err := json.Unmarshal([]byte(content), target); err != nil {
		return false, err
	}
	return true, nil
}
