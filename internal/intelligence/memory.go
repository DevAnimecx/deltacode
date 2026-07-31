package intelligence

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

type Namespace string

const (
	NSGlobal   Namespace = "global"
	NSWorkspace Namespace = "workspace"
	NSProject  Namespace = "project"
	NSSession  Namespace = "session"
	NSUser     Namespace = "user"
)

type MemoryEntry struct {
	ID        string    `json:"id"`
	Namespace Namespace `json:"namespace"`
	Key       string    `json:"key"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	Tokens    map[string]float64 `json:"-"`
}

type SearchResult struct {
	Entry MemoryEntry
	Score float64
}

type Memory struct {
	mu       sync.RWMutex
	entries  []MemoryEntry
	filePath string
}

func NewMemory() *Memory {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".delta", "memory")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "intelligence.json")
	m := &Memory{filePath: path}
	m.load()
	return m
}

func (m *Memory) load() {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return
	}
	var entries []MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	for i := range entries {
		entries[i].Tokens = m.tokenize(entries[i].Content)
	}
	m.entries = entries
}

func (m *Memory) save() error {
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0600)
}

func (m *Memory) Store(ns Namespace, key, content string, tags ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, e := range m.entries {
		if e.Namespace == ns && e.Key == key {
			m.entries[i].Content = content
			m.entries[i].Tags = tags
			m.entries[i].Tokens = m.tokenize(content)
			return m.save()
		}
	}

	entry := MemoryEntry{
		ID:        fmt.Sprintf("mem-%d", time.Now().UnixNano()),
		Namespace: ns,
		Key:       key,
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now(),
		Tokens:    m.tokenize(content),
	}
	m.entries = append(m.entries, entry)
	return m.save()
}

func (m *Memory) Get(ns Namespace, key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, e := range m.entries {
		if e.Namespace == ns && e.Key == key {
			return e.Content, true
		}
	}
	return "", false
}

func (m *Memory) Delete(ns Namespace, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.entries {
		if e.Namespace == ns && e.Key == key {
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			m.save()
			return
		}
	}
}

func (m *Memory) Search(query string, limit int) []SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	queryTokens := m.tokenize(query)
	var results []SearchResult

	for _, entry := range m.entries {
		if entry.Tokens == nil {
			entry.Tokens = m.tokenize(entry.Content)
		}
		score := m.cosineSimilarity(queryTokens, entry.Tokens)

		for qt := range queryTokens {
			for _, tag := range entry.Tags {
				if strings.Contains(strings.ToLower(tag), strings.ToLower(qt)) {
					score += 0.3
				}
			}
		}

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

func (m *Memory) SearchNamespace(ns Namespace, query string, limit int) []SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	queryTokens := m.tokenize(query)
	var results []SearchResult

	for _, entry := range m.entries {
		if entry.Namespace != ns {
			continue
		}
		if entry.Tokens == nil {
			entry.Tokens = m.tokenize(entry.Content)
		}
		score := m.cosineSimilarity(queryTokens, entry.Tokens)
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

func (m *Memory) ListNamespaces() []Namespace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[Namespace]bool{}
	for _, e := range m.entries {
		seen[e.Namespace] = true
	}
	var result []Namespace
	for ns := range seen {
		result = append(result, ns)
	}
	return result
}

func (m *Memory) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

func (m *Memory) tokenize(text string) map[string]float64 {
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

func (m *Memory) cosineSimilarity(a, b map[string]float64) float64 {
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
