package router

import (
	"sort"
	"sync"

	"github.com/DevAnimecx/deltacode/internal/telemetry"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

// AdaptiveRouter extends the base Router with telemetry-driven provider scoring.
type AdaptiveRouter struct {
	Router
	tele   *telemetry.Store
	mu     sync.RWMutex
	scores map[string]float64
}

func NewAdaptiveRouter(defaultProvider, defaultModel string, tele *telemetry.Store) *AdaptiveRouter {
	r := &AdaptiveRouter{
		Router: *NewRouter(defaultProvider, defaultModel),
		tele:   tele,
		scores: map[string]float64{},
	}
	if tele != nil {
		r.refreshScores()
	}
	return r
}

func (r *AdaptiveRouter) refreshScores() {
	if r.tele == nil {
		return
	}
	metrics := r.tele.Metrics()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scores = make(map[string]float64, len(metrics))
	for _, m := range metrics {
		r.scores[m.Provider+"/"+m.Model] = m.Score()
	}
}

// Score returns the telemetry score (0..1) for a provider/model pair.
func (r *AdaptiveRouter) Score(provider, model string) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.scores[provider+"/"+model]; ok {
		return s
	}
	return 0.5
}

// BestPairFor returns the highest-scoring configured pair for a task type,
// falling back to static rules.
func (r *AdaptiveRouter) BestPairFor(taskType TaskType, providers []models.ProviderConfig) (string, string) {
	r.refreshScores()

	var candidates []struct {
		provider string
		model    string
		score    float64
		priority int
	}
	for _, rule := range r.rules {
		if rule.TaskType != taskType {
			continue
		}
		if !providerExists(rule.Provider, providers) {
			continue
		}
		candidates = append(candidates, struct {
			provider string
			model    string
			score    float64
			priority int
		}{rule.Provider, rule.Model, r.Score(rule.Provider, rule.Model), rule.Priority})
	}

	if len(candidates) == 0 {
		return r.defaultProvider, r.defaultModel
	}

	sort.Slice(candidates, func(i, j int) bool {
		// Weighted: score dominates when there is data, priority breaks ties.
		si := candidates[i].score + float64(candidates[i].priority)*0.0005
		sj := candidates[j].score + float64(candidates[j].priority)*0.0005
		return si > sj
	})
	return candidates[0].provider, candidates[0].model
}

func (r *AdaptiveRouter) RouteAdaptive(prompt string, providers []models.ProviderConfig) (string, string) {
	taskType := r.Classify(prompt)
	return r.BestPairFor(taskType, providers)
}
