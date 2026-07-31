package tui

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type providerHealth struct {
	latency   time.Duration
	ok        bool
	lastCheck time.Time
	fails     int
}

type healthStore struct {
	mu      sync.Mutex
	status  map[string]providerHealth
	cfg     *config.Manager
	running bool
}

func newHealthStore(cfg *config.Manager) *healthStore {
	return &healthStore{status: make(map[string]providerHealth), cfg: cfg}
}

func (h *healthStore) start() {
	if h.running {
		return
	}
	h.running = true
	go func() {
		h.pingAll()
		for {
			time.Sleep(45 * time.Second)
			h.pingAll()
		}
	}()
}

func (h *healthStore) pingAll() {
	conf := h.cfg.GetConfig()
	for _, p := range conf.Providers {
		h.ping(p.Name, p.BaseURL, p.APIKey)
	}
}

func (h *healthStore) ping(name, baseURL, apiKey string) {
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", baseURL+"/v1/models", nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	ok := false
	lat := time.Since(start)
	if err == nil {
		ok = resp.StatusCode < 500
		resp.Body.Close()
	}
	h.mu.Lock()
	s := h.status[name]
	s.lastCheck = time.Now()
	s.latency = lat
	if ok {
		s.fails = 0
	} else {
		s.fails++
	}
	s.ok = s.fails < 3
	h.status[name] = s
	h.mu.Unlock()
}

func (h *healthStore) get(name string) providerHealth {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.status[name]; ok {
		return s
	}
	return providerHealth{ok: true}
}

func (m *model) healthDot(name string) string {
	if m.health == nil {
		return ""
	}
	s := m.health.get(name)
	switch {
	case !s.ok:
		return m.t.errM.Render("●")
	case s.latency > 2*time.Second:
		return m.t.badge.Render("◐")
	case s.latency > 500*time.Millisecond:
		return m.t.dim.Render("◑")
	default:
		return m.t.fk.Render("●")
	}
}

func (m *model) healthLatency(name string) string {
	if m.health == nil {
		return ""
	}
	s := m.health.get(name)
	if s.lastCheck.IsZero() {
		return "checking..."
	}
	if !s.ok {
		return "(down)"
	}
	return fmt.Sprintf("(%dms)", s.latency.Milliseconds())
}

func (m *model) failover() *models.ProviderConfig {
	conf := m.cfg.GetConfig()
	for _, p := range conf.Providers {
		if p.Name == m.provName {
			continue
		}
		if m.health.get(p.Name).ok {
			return &p
		}
	}
	return nil
}
