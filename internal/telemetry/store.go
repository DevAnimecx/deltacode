package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ProviderMetric struct {
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	Calls          int       `json:"calls"`
	Successes      int       `json:"successes"`
	Failures       int       `json:"failures"`
	TotalLatencyMs float64   `json:"total_latency_ms"`
	TotalTokens    int       `json:"total_tokens"`
	TotalCost      float64   `json:"total_cost"`
	Retries        int       `json:"retries"`
	LastUsed       time.Time `json:"last_used"`
}

func (m *ProviderMetric) Score() float64 {
	if m.Calls == 0 {
		return 0.5
	}
	successRate := float64(m.Successes) / float64(m.Calls)
	avgLatency := m.TotalLatencyMs / float64(m.Calls)
	latencyScore := 1.0
	switch {
	case avgLatency < 2000:
		latencyScore = 1.0
	case avgLatency < 5000:
		latencyScore = 0.8
	case avgLatency < 10000:
		latencyScore = 0.6
	case avgLatency < 20000:
		latencyScore = 0.4
	default:
		latencyScore = 0.2
	}
	costScore := 1.0
	if m.TotalCost > 0 {
		costPerCall := m.TotalCost / float64(m.Calls)
		switch {
		case costPerCall < 0.01:
			costScore = 1.0
		case costPerCall < 0.05:
			costScore = 0.8
		case costPerCall < 0.2:
			costScore = 0.6
		default:
			costScore = 0.4
		}
	}
	return successRate*0.55 + latencyScore*0.25 + costScore*0.2
}

type Event struct {
	Time      time.Time      `json:"time"`
	Provider  string         `json:"provider,omitempty"`
	Model     string         `json:"model,omitempty"`
	Agent     string         `json:"agent,omitempty"`
	Phase     string         `json:"phase,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	Type      string         `json:"type"`
	LatencyMs float64        `json:"latency_ms,omitempty"`
	Tokens    int            `json:"tokens,omitempty"`
	Cost      float64        `json:"cost,omitempty"`
	OK        bool           `json:"ok"`
	Detail    string         `json:"detail,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

type Store struct {
	mu        sync.RWMutex
	dir       string
	metrics   map[string]*ProviderMetric
	events    []Event
	maxEvents int
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".delta", "telemetry")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, metrics: map[string]*ProviderMetric{}, maxEvents: 500}
	s.loadMetrics()
	s.loadEvents()
	return s, nil
}

func NewStoreAt(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, metrics: map[string]*ProviderMetric{}, maxEvents: 500}
	s.loadMetrics()
	s.loadEvents()
	return s, nil
}

func key(p, m string) string { return p + "/" + m }

func (s *Store) loadMetrics() {
	data, err := os.ReadFile(filepath.Join(s.dir, "metrics.json"))
	if err != nil {
		return
	}
	var metrics map[string]*ProviderMetric
	if err := json.Unmarshal(data, &metrics); err != nil {
		return
	}
	s.metrics = metrics
}

func (s *Store) saveMetrics() error {
	data, err := json.MarshalIndent(s.metrics, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "metrics.json"), data, 0600)
}

func (s *Store) loadEvents() {
	data, err := os.ReadFile(filepath.Join(s.dir, "events.json"))
	if err != nil {
		return
	}
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return
	}
	if len(events) > s.maxEvents {
		events = events[len(events)-s.maxEvents:]
	}
	s.events = events
}

func (s *Store) saveEvents() error {
	data, err := json.MarshalIndent(s.events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "events.json"), data, 0600)
}

func (s *Store) RecordCall(provider, model string, latencyMs float64, tokens int, cost float64, ok bool, retries int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(provider, model)
	m := s.metrics[k]
	if m == nil {
		m = &ProviderMetric{Provider: provider, Model: model}
		s.metrics[k] = m
	}
	m.Calls++
	m.TotalLatencyMs += latencyMs
	m.TotalTokens += tokens
	m.TotalCost += cost
	m.Retries += retries
	m.LastUsed = time.Now()
	if ok {
		m.Successes++
	} else {
		m.Failures++
	}
	return s.saveMetrics()
}

func (s *Store) RecordEvent(ev Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev.Time = time.Now()
	s.events = append(s.events, ev)
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
	return s.saveEvents()
}

func (s *Store) Metrics() []ProviderMetric {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []ProviderMetric
	for _, m := range s.metrics {
		result = append(result, *m)
	}
	return result
}

func (s *Store) MetricFor(provider, model string) (ProviderMetric, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.metrics[key(provider, model)]
	if !ok {
		return ProviderMetric{}, false
	}
	return *m, true
}

func (s *Store) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

func (s *Store) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var b []byte
	lines := []string{}
	totalCalls := 0
	totalTokens := 0
	totalCost := 0.0
	for _, m := range s.metrics {
		totalCalls += m.Calls
		totalTokens += m.TotalTokens
		totalCost += m.TotalCost
	}
	lines = append(lines, fmt.Sprintf("calls=%d tokens=%d cost=$%.4f", totalCalls, totalTokens, totalCost))
	_ = b
	return joinLines(lines)
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
