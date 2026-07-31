package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

type VectorMemory struct {
	mu       sync.RWMutex
	entries  []VectorEntry
	filePath string
}

type VectorEntry struct {
	ID        string             `json:"id"`
	Content   string             `json:"content"`
	Tags      []string           `json:"tags"`
	CreatedAt time.Time          `json:"created_at"`
	Tokens    map[string]float64 `json:"-"`
}

type SearchResult struct {
	Entry VectorEntry
	Score float64
}

func NewVectorMemory() (*VectorMemory, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".delta", "memory")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "vectors.json")

	vm := &VectorMemory{
		filePath: path,
	}
	vm.load()
	return vm, nil
}

func (vm *VectorMemory) load() {
	data, err := os.ReadFile(vm.filePath)
	if err != nil {
		return
	}
	var entries []VectorEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	for i := range entries {
		entries[i].Tokens = tokenize(entries[i].Content)
	}
	vm.entries = entries
}

func (vm *VectorMemory) save() error {
	data, err := json.MarshalIndent(vm.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(vm.filePath, data, 0600)
}

func (vm *VectorMemory) Store(content string, tags []string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	entry := VectorEntry{
		ID:        fmt.Sprintf("mem-%d", time.Now().UnixNano()),
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now(),
		Tokens:    tokenize(content),
	}
	vm.entries = append(vm.entries, entry)
	return vm.save()
}

func (vm *VectorMemory) Search(query string, limit int) []SearchResult {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	queryTokens := tokenize(query)
	var results []SearchResult

	for _, entry := range vm.entries {
		if entry.Tokens == nil {
			entry.Tokens = tokenize(entry.Content)
		}
		score := cosineSimilarity(queryTokens, entry.Tokens)

		tagScore := 0.0
		for qt := range queryTokens {
			for _, tag := range entry.Tags {
				if strings.Contains(strings.ToLower(tag), strings.ToLower(qt)) {
					tagScore += 0.3
				}
			}
		}
		score += tagScore

		if score > 0 {
			results = append(results, SearchResult{Entry: entry, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func (vm *VectorMemory) SearchByTag(tag string, limit int) []VectorEntry {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	tag = strings.ToLower(tag)
	var results []VectorEntry
	for _, entry := range vm.entries {
		for _, t := range entry.Tags {
			if strings.Contains(strings.ToLower(t), tag) {
				results = append(results, entry)
				break
			}
		}
		if len(results) >= limit {
			break
		}
	}
	return results
}

func (vm *VectorMemory) Recent(limit int) []VectorEntry {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if limit <= 0 || limit > len(vm.entries) {
		limit = len(vm.entries)
	}

	entries := make([]VectorEntry, limit)
	for i := 0; i < limit; i++ {
		entries[i] = vm.entries[len(vm.entries)-1-i]
	}
	return entries
}

func tokenize(text string) map[string]float64 {
	tokens := make(map[string]float64)
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, word := range words {
		if len(word) < 2 {
			continue
		}
		tokens[word]++
	}
	for word, count := range tokens {
		tokens[word] = 1 + math.Log2(count)
	}
	return tokens
}

func cosineSimilarity(a, b map[string]float64) float64 {
	dotProduct := 0.0
	magnA := 0.0
	magnB := 0.0

	for k, v := range a {
		dotProduct += v * b[k]
		magnA += v * v
	}
	for _, v := range b {
		magnB += v * v
	}

	if magnA == 0 || magnB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(magnA) * math.Sqrt(magnB))
}
