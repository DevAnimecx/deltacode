package tui

import (
	"strings"
	"sync"
	"time"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
)

type stopOnce struct {
	once sync.Once
	ch   chan struct{}
}

func (s *stopOnce) close() {
	s.once.Do(func() {
		if s.ch != nil {
			close(s.ch)
			s.ch = nil
		}
	})
}

func (m *model) streamWorker(ch chan chunk, stop *stopOnce, req models.ChatRequest, p provider.Provider) {
	defer func() {
		recover()
	}()
	defer close(ch)

	if stop == nil || stop.ch == nil {
		return
	}

	stream, err := p.ChatStream(req)
	if err != nil {
		select {
		case ch <- chunk{err: err}:
		case <-stop.ch:
		}
		return
	}

	var hadContent bool
	for raw := range stream {
		select {
		case <-stop.ch:
			return
		default:
		}
		if raw.Error != nil {
			select {
			case ch <- chunk{err: raw.Error}:
			case <-stop.ch:
			}
			return
		}
		if raw.Done {
			break
		}
		if raw.Content != "" {
			hadContent = true
		}
		if raw.Reasoning != "" || raw.Content != "" {
			select {
			case ch <- chunk{content: raw.Content, reasoning: raw.Reasoning}:
			case <-stop.ch:
				return
			}
		}
	}

	if !hadContent {
		// Fallback: some proxies return empty streams. Retry non-streaming.
		resp, err := p.Chat(req)
		if err != nil {
			select {
			case ch <- chunk{err: err}:
			case <-stop.ch:
			}
			return
		}
		select {
		case ch <- chunk{content: resp.Message.Content, usage: &resp.Usage}:
		case <-stop.ch:
			return
		}
	}

	select {
	case ch <- chunk{done: true}:
	case <-stop.ch:
	}
}

func (m *model) nextChunk() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.streamCh
		if !ok {
			return chunk{done: true}
		}
		return msg
	}
}

func (m *model) tick() tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
		return tick{}
	})
}

func (m *model) onChunk(c chunk) tea.Cmd {
	if c.err != nil {
		m.failStream(c.err)
		return nil
	}
	if c.done {
		m.finishStream()
		return nil
	}
	if c.reasoning != "" {
		m.rb.WriteString(c.reasoning)
	}
	if c.content != "" {
		m.sb.WriteString(c.content)
		m.estTokens(c.content)
		m.addStream(c.content)
	}
	if c.usage != nil {
		m.tok = c.usage.TotalTokens
		if c.usage.TotalTokens > 0 {
			m.cost = m.calcCost(c.usage.TotalTokens)
		}
	}
	if m.atBottom && !m.scrollLocked {
		m.vp.GotoBottom()
	}
	return m.nextChunk()
}

func (m *model) finishStream() {
	full := m.sb.String()
	reasoning := m.rb.String()
	m.streaming = false
	m.statusText = "Ready"

	if full != "" {
		// Convert the streaming entry into a final assistant entry
		// instead of appending a duplicate.
		if n := len(m.entries); n > 0 && m.entries[n-1].role == "streaming" {
			m.entries[n-1] = entry{
				role:          "assistant",
				content:       full,
				reasoning:     reasoning,
				duration:      time.Since(m.startTime),
				tokens:        m.tok,
				cost:          m.cost,
				showReasoning: m.reasoningVisible,
				collapsed:     m.collapseLong,
				ts:            time.Now(),
			}
		} else {
			m.addAsst(full, reasoning)
		}
		m.messages = append(m.messages, models.Message{Role: models.RoleAssistant, Content: full})
		m.storeMemory(m.lastPrompt, full)
		m.autoTitle(m.lastPrompt)
	}

	m.stopOnce = nil
	m.streamCh = nil
	m.ta.Focus()
	m.saveSession()
	m.render()
}

func (m *model) failStream(err error) {
	m.streaming = false
	m.statusText = "Error"
	m.lastError = err
	m.addErr("Error: " + err.Error())
	m.addSys("Press R to retry, or type a new prompt.")
	m.stopOnce = nil
	m.streamCh = nil
	m.ta.Focus()
	m.render()
}

// stopStream keeps any partial content as an assistant entry.
func (m *model) stopStream() {
	m.streaming = false
	m.statusText = "Stopped"
	m.stopOnce.close()
	m.stopOnce = nil

	partial := m.sb.String()
	if partial != "" {
		if n := len(m.entries); n > 0 && m.entries[n-1].role == "streaming" {
			m.entries[n-1] = entry{
				role:          "assistant",
				content:       partial + "\n\n" + m.t.dim.Render("(stopped)"),
				reasoning:     m.rb.String(),
				duration:      time.Since(m.startTime),
				tokens:        m.tok,
				cost:          m.cost,
				showReasoning: m.reasoningVisible,
				ts:            time.Now(),
			}
			m.messages = append(m.messages, models.Message{Role: models.RoleAssistant, Content: partial})
		}
	}
	m.streamCh = nil
	m.ta.Focus()
	m.saveSession()
	m.render()
}

func (m *model) estTokens(text string) {
	if text == "" {
		return
	}
	// Rough estimate: ~4 chars per token for code, ~5 for prose.
	m.tok += len(text) / 5
	m.cost = m.calcCost(m.tok)
}

func (m *model) runPrompt(prompt string) {
	m.submit(prompt)
}

func (m *model) calcCost(tok int) float64 {
	// Per-token pricing varies by model; use a conservative average.
	rate := 0.000002
	lower := strings.ToLower(m.modelName)
	switch {
	case strings.Contains(lower, "mini"), strings.Contains(lower, "haiku"), strings.Contains(lower, "flash"):
		rate = 0.0000006
	case strings.Contains(lower, "claude"), strings.Contains(lower, "sonnet"):
		rate = 0.000005
	case strings.Contains(lower, "gpt-4"):
		rate = 0.000004
	}
	return float64(tok) * rate
}
