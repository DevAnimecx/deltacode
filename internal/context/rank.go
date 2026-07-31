package context

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/intelligence"
	"github.com/DevAnimecx/deltacode/internal/symbols"
)

type RankedFile struct {
	Path   string  `json:"path"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

type Ranker struct {
	projectDir string
	graph      *symbols.SymbolGraph
	mem        *intelligence.Memory
}

func NewRanker(projectDir string, graph *symbols.SymbolGraph, mem *intelligence.Memory) *Ranker {
	return &Ranker{projectDir: projectDir, graph: graph, mem: mem}
}

// RankFiles scores project files by recency (git), import distance, symbol relevance, and memory.
func (r *Ranker) RankFiles(query string, files []string) []RankedFile {
	var ranked []RankedFile
	queryTokens := tokenize(query)
	now := time.Now()

	for _, f := range files {
		score := 0.0
		var reasons []string

		// 1. Git recency: recently touched files are more relevant.
		if mod, err := statModTime(r.projectDir, f); err == nil {
			hours := now.Sub(mod).Hours()
			rec := 0.3 * expDecay(hours, 24*14)
			if rec > 0.01 {
				score += rec
				reasons = append(reasons, "recent")
			}
		}

		// 2. Semantic: file content matching query tokens.
		if data, err := os.ReadFile(filepath.Join(r.projectDir, f)); err == nil {
			content := strings.ToLower(string(data))
			matches := 0
			for t := range queryTokens {
				if strings.Contains(content, t) {
					matches++
				}
			}
			if matches > 0 {
				sem := 0.5 * float64(matches) / float64(len(queryTokens))
				score += sem
				reasons = append(reasons, "semantic")
			}

			// 3. Path-based: filename tokens matching query.
			base := strings.ToLower(filepath.Base(f))
			ext := filepath.Ext(base)
			base = strings.TrimSuffix(base, ext)
			for t := range queryTokens {
				if strings.Contains(base, t) || strings.Contains(t, base) {
					score += 0.4
					reasons = append(reasons, "name")
					break
				}
			}
		}

		// 4. Import distance: files near high-fan-in symbols get a small boost.
		if r.graph != nil {
			syms := r.graph.GetSymbolsByFile(filepath.Join(r.projectDir, f))
			if len(syms) > 0 {
				score += 0.05
			}
		}

		// 5. Memory relevance.
		if r.mem != nil {
			results := r.mem.SearchLayer(intelligence.LayerRepo, query, 3)
			for _, res := range results {
				if strings.Contains(res.Entry.Content, f) || strings.Contains(f, res.Entry.Content) {
					score += res.Score * 0.3
					reasons = append(reasons, "memory")
					break
				}
			}
		}

		if score > 0 {
			ranked = append(ranked, RankedFile{Path: f, Score: score, Reason: strings.Join(unique(reasons), ",")})
		}
	}

	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
	return ranked
}

func (r *Ranker) TopFiles(query string, files []string, limit int) []string {
	ranked := r.RankFiles(query, files)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]string, 0, len(ranked))
	for _, rf := range ranked {
		out = append(out, rf.Path)
	}
	return out
}

func tokenize(s string) map[string]bool {
	tokens := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '.')
	}) {
		if len(w) > 1 {
			tokens[w] = true
		}
	}
	return tokens
}

func expDecay(x, halfLife float64) float64 {
	return math.Exp(-x / halfLife)
}

func statModTime(projectDir, f string) (time.Time, error) {
	info, err := os.Stat(filepath.Join(projectDir, f))
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func unique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}
