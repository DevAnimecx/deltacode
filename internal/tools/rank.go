package tools

import (
	"sort"
	"strings"
	"time"
)

// RankWeights controls how the ranking engine balances factors.
type RankWeights struct {
	Capability   float64
	Reliability  float64
	Performance  float64
	Security     float64
	Preference   float64
	History      float64
	Cost         float64
}

func DefaultWeights() RankWeights {
	return RankWeights{
		Capability:  0.35,
		Reliability: 0.20,
		Performance: 0.15,
		Security:    0.10,
		Preference:  0.05,
		History:     0.10,
		Cost:        0.05,
	}
}

type RankedTool struct {
	Tool  *Tool
	Score float64
	Why   string
}

// Ranker scores candidate tools for a given capability request.
type Ranker struct {
	weights RankWeights
}

func NewRanker() *Ranker {
	return &Ranker{weights: DefaultWeights()}
}

func (r *Ranker) SetWeights(w RankWeights) { r.weights = w }

func (r *Ranker) capabilityMatch(t *Tool, query string) float64 {
	q := strings.ToLower(query)
	kw := keywordsFor(t)
	if q == "" {
		return 0.3
	}
	score := 0.0
	// Exact name/id match dominates.
	if t.Name() == q || t.ID() == q {
		return 1.0
	}
	if strings.Contains(kw, q) {
		score = 0.9
	}
	// Token-level match.
	tokens := tokenizeQuery(q)
	if len(tokens) > 0 {
		// A query token equal to the tool's own name wins.
		nameParts := strings.FieldsFunc(t.Name(), func(r rune) bool { return r == '-' })
		for _, tok := range tokens {
			for _, np := range nameParts {
				if tok == np {
					return 1.0
				}
			}
		}
		hits := 0
		kwWords := strings.Fields(kw)
		for _, tok := range tokens {
			wordHit := false
			for _, w := range kwWords {
				if w == tok {
					wordHit = true
					break
				}
			}
			if wordHit || strings.Contains(kw, tok) {
				hits++
			}
		}
		ratio := float64(hits) / float64(len(tokens))
		if ratio > score {
			score = ratio * 0.95
		}
	}
	return score
}

func tokenizeQuery(q string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(q, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if len(w) > 1 {
			out = append(out, w)
		}
	}
	return out
}

func (r *Ranker) securityScore(t *Tool) float64 {
	switch t.Manifest.TrustLevel {
	case "trusted", "verified":
		return 1.0
	case "user":
		return 0.9
	case "community":
		return 0.7
	case "untrusted":
		return 0.2
	}
	// Default by source.
	switch t.Manifest.Source {
	case "builtin":
		return 1.0
	case "user":
		return 0.9
	case "marketplace":
		return 0.7
	}
	return 0.5
}

func (r *Ranker) preferenceScore(t *Tool) float64 {
	if t.Stats.Calls > 0 {
		return 1.0
	}
	return 0.3
}

func (r *Ranker) costScore(t *Tool) float64 {
	if t.Stats.AvgRuntimeMs <= 0 {
		return 0.7
	}
	switch {
	case t.Stats.AvgRuntimeMs < 500:
		return 1.0
	case t.Stats.AvgRuntimeMs < 3000:
		return 0.8
	case t.Stats.AvgRuntimeMs < 10000:
		return 0.5
	default:
		return 0.2
	}
}

// Rank returns tools sorted by score for the query.
func (r *Ranker) Rank(query string, candidates []*Tool, limit int) []RankedTool {
	var ranked []RankedTool
	for _, t := range candidates {
		cap := r.capabilityMatch(t, query)
		if cap <= 0 && query != "" {
			continue
		}
		rel := t.Stats.Reliability()
		perf := costToPerformance(t.Stats.AvgRuntimeMs)
		sec := r.securityScore(t)
		pref := r.preferenceScore(t)
		hist := historyScore(t)
		cost := r.costScore(t)

		score := r.weights.Capability*cap +
			r.weights.Reliability*rel +
			r.weights.Performance*perf +
			r.weights.Security*sec +
			r.weights.Preference*pref +
			r.weights.History*hist +
			r.weights.Cost*cost

		var why []string
		if cap > 0.6 {
			why = append(why, "capability match")
		}
		if rel > 0.8 {
			why = append(why, "reliable")
		}
		if sec >= 0.9 {
			why = append(why, "trusted")
		}
		if t.Stats.Calls > 0 {
			why = append(why, "proven")
		}
		ranked = append(ranked, RankedTool{Tool: t, Score: score, Why: strings.Join(why, ",")})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// Best returns the single highest-ranked compatible tool.
func (r *Ranker) Best(query string, candidates []*Tool) (*Tool, float64, bool) {
	ranked := r.Rank(query, candidates, 1)
	if len(ranked) == 0 {
		return nil, 0, false
	}
	return ranked[0].Tool, ranked[0].Score, true
}

func costToPerformance(avgMs float64) float64 {
	if avgMs <= 0 {
		return 0.5
	}
	switch {
	case avgMs < 500:
		return 1.0
	case avgMs < 3000:
		return 0.85
	case avgMs < 10000:
		return 0.6
	default:
		return 0.3
	}
}

func historyScore(t *Tool) float64 {
	if t.Stats.Calls == 0 {
		return 0
	}
	recency := 0.0
	if !t.Stats.LastUsed.IsZero() {
		age := time.Since(t.Stats.LastUsed).Hours()
		recency = 1.0
		if age > 24 {
			recency = 1.0 / (1.0 + age/24)
		}
	}
	return 0.5*t.Stats.SuccessRate() + 0.5*recency
}
