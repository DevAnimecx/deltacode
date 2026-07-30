package tui

import (
	"time"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) streamWorker(ch chan chunk, stop chan struct{}, req models.ChatRequest, p provider.Provider) {
	defer func() {
		recover()
	}()
	defer close(ch)

	stream, err := p.ChatStream(req)
	if err != nil {
		select {
		case ch <- chunk{err: err}:
		case <-stop:
		}
		return
	}

	for raw := range stream {
		select {
		case <-stop:
			return
		default:
		}
		if raw.Error != nil {
			select {
			case ch <- chunk{err: raw.Error}:
			case <-stop:
			}
			return
		}
		if raw.Done {
			break
		}
		select {
		case ch <- chunk{content: raw.Content, reasoning: raw.Reasoning}:
		case <-stop:
			return
		}
	}

	select {
	case ch <- chunk{done: true}:
	case <-stop:
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
		m.streaming = false
		m.statusText = "Error"
		m.addErr("Error: " + c.err.Error())
		m.streamCh = nil
		m.ta.Focus()
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
		m.addStream(c.content)
	}
	if m.atBottom {
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
		m.addAsst(full, reasoning)
		m.messages = append(m.messages, models.Message{Role: models.RoleAssistant, Content: full})
		m.storeMemory(m.lastPrompt, full)
	}
	m.streamCh = nil
	m.ta.Focus()
	m.render()
}

func (m *model) stopStream() {
	m.streaming = false
	m.statusText = "Stopped"
	m.ta.Focus()
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	m.render()
}

func (m *model) runPrompt(prompt string) {
	m.submit(prompt)
}

func (m *model) calcCost(tok int) float64 {
	return float64(tok) * 0.000002
}
