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

type Layer string

const (
	LayerGlobal    Layer = "global"
	LayerWorkspace Layer = "workspace"
	LayerRepo      Layer = "repo"
	LayerFeature   Layer = "feature"
	LayerTask      Layer = "task"
	LayerSession   Layer = "session"
	LayerTemp      Layer = "temp"
)

type Namespace = Layer

const (
	NSGlobal   Namespace = LayerGlobal
	NSWorkspace Namespace = LayerWorkspace
	NSProject  Namespace = LayerRepo
	NSSession  Namespace = LayerSession
	NSUser     Namespace = LayerFeature
)

type MemoryEntry struct {
	ID          string    `json:"id"`
	Namespace   Namespace `json:"namespace"`
	Layer       Layer     `json:"layer"`
	Key         string    `json:"key"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	Priority    float64   `json:"priority"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Confidence  float64   `json:"confidence"`
	Source      string    `json:"source,omitempty"`
	Verified    bool      `json:"verified"`
	AccessCount int       `json:"access_count"`
	LastAccess  time.Time `json:"last_access,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tokens      map[string]float64 `json:"-"`
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

func NewMemoryAt(path string) *Memory {
	os.MkdirAll(filepath.Dir(path), 0700)
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
		if entries[i].Layer == "" {
			entries[i].Layer = Layer(entries[i].Namespace)
			if entries[i].Layer == "" {
				entries[i].Layer = LayerGlobal
			}
		}
		if entries[i].Namespace == "" {
			entries[i].Namespace = entries[i].Layer
		}
		entries[i].Tokens = m.tokenize(entries[i].Content)
	}
	m.entries = entries
	m.pruneExpired()
}

func (m *Memory) save() error {
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0600)
}

func (m *Memory) pruneExpired() {
	now := time.Now()
	kept := m.entries[:0]
	for _, e := range m.entries {
		if !e.ExpiresAt.IsZero() && e.ExpiresAt.Before(now) {
			continue
		}
		kept = append(kept, e)
	}
	m.entries = kept
}

func (m *Memory) Store(ns Namespace, key, content string, tags ...string) error {
	return m.StoreEx(ns, key, content, StoreOptions{Tags: tags})
}

type StoreOptions struct {
	Tags       []string
	Priority   float64
	ExpiresAt  time.Time
	Confidence float64
	Source     string
	Verified   bool
	Layer      Layer
}

func (m *Memory) StoreEx(ns Namespace, key, content string, opts StoreOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked()
	now := time.Now()
	layer := opts.Layer
	if layer == "" {
		layer = Layer(ns)
		if layer == "" {
			layer = LayerGlobal
		}
	}

	for i, e := range m.entries {
		if e.Namespace == ns && e.Key == key {
			m.entries[i].Content = content
			m.entries[i].Tags = opts.Tags
			m.entries[i].Priority = opts.Priority
			m.entries[i].Confidence = opts.Confidence
			m.entries[i].Source = opts.Source
			m.entries[i].Verified = opts.Verified
			m.entries[i].Layer = layer
			if !opts.ExpiresAt.IsZero() {
				m.entries[i].ExpiresAt = opts.ExpiresAt
			}
			m.entries[i].UpdatedAt = now
			m.entries[i].Tokens = m.tokenize(content)
			return m.save()
		}
	}

	entry := MemoryEntry{
		ID:         fmt.Sprintf("mem-%d", time.Now().UnixNano()),
		Namespace:  ns,
		Layer:      layer,
		Key:        key,
		Content:    content,
		Tags:       opts.Tags,
		Priority:   opts.Priority,
		ExpiresAt:  opts.ExpiresAt,
		Confidence: opts.Confidence,
		Source:     opts.Source,
		Verified:   opts.Verified,
		CreatedAt:  now,
		UpdatedAt:  now,
		Tokens:     m.tokenize(content),
	}
	m.entries = append(m.entries, entry)
	return m.save()
}

func (m *Memory) pruneExpiredLocked() {
	now := time.Now()
	kept := m.entries[:0]
	for _, e := range m.entries {
		if !e.ExpiresAt.IsZero() && e.ExpiresAt.Before(now) {
			continue
		}
		kept = append(kept, e)
	}
	m.entries = kept
}

func (m *Memory) Get(ns Namespace, key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for i, e := range m.entries {
		if e.Namespace == ns && e.Key == key {
			if !e.ExpiresAt.IsZero() && e.ExpiresAt.Before(now) {
				m.entries = append(m.entries[:i], m.entries[i+1:]...)
				return "", false
			}
			m.entries[i].AccessCount++
			m.entries[i].LastAccess = now
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked()

	if limit <= 0 {
		limit = 10
	}

	queryTokens := m.tokenize(query)
	var results []SearchResult

	for i := range m.entries {
		e := &m.entries[i]
		if e.Tokens == nil {
			e.Tokens = m.tokenize(e.Content)
		}
		score := m.cosineSimilarity(queryTokens, e.Tokens)

		for qt := range queryTokens {
			for _, tag := range e.Tags {
				if strings.Contains(strings.ToLower(tag), strings.ToLower(qt)) {
					score += 0.3
				}
			}
		}

		// Recency boost.
		age := time.Since(e.UpdatedAt)
		recencyBoost := 0.25 * math.Exp(-age.Hours()/(24*30))
		score += recencyBoost

		// Priority + confidence + access.
		score += e.Priority * 0.2
		score += e.Confidence * 0.1
		score += math.Min(0.15, float64(e.AccessCount)*0.02)

		if score > 0 {
			results = append(results, SearchResult{Entry: *e, Score: score})
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

func (m *Memory) SearchLayer(layer Layer, query string, limit int) []SearchResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked()

	if limit <= 0 {
		limit = 10
	}

	queryTokens := m.tokenize(query)
	var results []SearchResult

	for i := range m.entries {
		e := &m.entries[i]
		if e.Layer != layer {
			continue
		}
		if e.Tokens == nil {
			e.Tokens = m.tokenize(e.Content)
		}
		score := m.cosineSimilarity(queryTokens, e.Tokens)
		if score > 0 {
			results = append(results, SearchResult{Entry: *e, Score: score})
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
	return m.SearchLayer(Layer(ns), query, limit)
}

func (m *Memory) RecallRecent(layer Layer, limit int) []MemoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked()
	var result []MemoryEntry
	for i := range m.entries {
		if m.entries[i].Layer == layer {
			result = append(result, m.entries[i])
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func (m *Memory) Verify(ns Namespace, key string, verified bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.entries {
		if m.entries[i].Namespace == ns && m.entries[i].Key == key {
			m.entries[i].Verified = verified
			if verified {
				m.entries[i].Confidence = 1.0
			}
			m.entries[i].UpdatedAt = time.Now()
			m.save()
			return
		}
	}
}

func (m *Memory) SetConfidence(ns Namespace, key string, confidence float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.entries {
		if m.entries[i].Namespace == ns && m.entries[i].Key == key {
			m.entries[i].Confidence = confidence
			m.entries[i].UpdatedAt = time.Now()
			m.save()
			return
		}
	}
}

func (m *Memory) ListLayers() []Layer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[Layer]bool{}
	for _, e := range m.entries {
		seen[e.Layer] = true
	}
	var result []Layer
	for l := range seen {
		result = append(result, l)
	}
	return result
}

func (m *Memory) ListNamespaces() []Namespace {
	return m.ListLayers()
}

func (m *Memory) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked()
	return len(m.entries)
}

func (m *Memory) PruneExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	before := len(m.entries)
	m.pruneExpiredLocked()
	if len(m.entries) != before {
		m.save()
	}
	return before - len(m.entries)
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
