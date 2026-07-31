package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

func sessionsDir() string {
	dir := filepath.Join(os.Getenv("HOME"), ".delta", "sessions")
	os.MkdirAll(dir, 0755)
	return dir
}

func sessionFiles() []string {
	entries, err := os.ReadDir(sessionsDir())
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(sessionsDir(), e.Name()))
		}
	}
	return files
}

func (m *model) listSessions() []sessionMeta {
	var metas []sessionMeta
	for _, f := range sessionFiles() {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var data sessionData
		if json.Unmarshal(b, &data) != nil {
			continue
		}
		info, _ := os.Stat(f)
		title := data.Title
		if title == "" && len(data.Messages) > 0 {
			title = data.Messages[0].Content
		}
		title = strings.ReplaceAll(title, "\n", " ")
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		metas = append(metas, sessionMeta{
			File:     f,
			Title:    title,
			Messages: len(data.Messages),
			Cost:     data.Cost,
			Model:    data.Model,
			Updated:  info.ModTime(),
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Updated.After(metas[j].Updated)
	})
	return metas
}

func (m *model) showSessions() {
	m.sessions = m.listSessions()
	if len(m.sessions) == 0 {
		m.addSys("No saved sessions.")
		return
	}
	m.addSys(fmt.Sprintf("━━━ %d sessions (newest first) — press %s to load ━━━", len(m.sessions), m.t.fk.Render("1-9")))
	for i, s := range m.sessions {
		t := time.Since(s.Updated)
		when := "just now"
		switch {
		case t < time.Minute:
			when = "just now"
		case t < time.Hour:
			when = fmt.Sprintf("%dm ago", int(t.Minutes()))
		case t < 24*time.Hour:
			when = fmt.Sprintf("%dh ago", int(t.Hours()))
		default:
			when = fmt.Sprintf("%dd ago", int(t.Hours()/24))
		}
		if i < 9 {
			m.addSys(fmt.Sprintf("  %s %-42s %2d msgs  $%.3f  %s  %s",
				m.t.fk.Render(fmt.Sprintf("%d)", i+1)),
				s.Title, s.Messages, s.Cost, s.Model, m.t.dim.Render(when)))
		} else {
			m.addSys(fmt.Sprintf("  ·  %-42s %2d msgs  $%.3f  %s  %s",
				s.Title, s.Messages, s.Cost, s.Model, m.t.dim.Render(when)))
		}
	}
	m.addSys("Press 1-9 to resume a session.")
}

func (m *model) resumeSession(i int) {
	if i < 0 || i >= len(m.sessions) {
		return
	}
	b, err := os.ReadFile(m.sessions[i].File)
	if err != nil {
		m.addSys("Cannot load session: " + err.Error())
		return
	}
	var data sessionData
	if json.Unmarshal(b, &data) != nil {
		m.addSys("Corrupt session file.")
		return
	}
	m.messages = data.Messages
	m.cost = data.Cost
	m.tok = data.Tokens
	m.sessionTitle = data.Title
	if data.Model != "" {
		m.modelName = data.Model
	}
	if data.Provider != "" {
		m.provName = data.Provider
	}
	m.entries = nil
	for _, msg := range m.messages {
		switch msg.Role {
		case models.RoleUser:
			m.entries = append(m.entries, entry{role: "user", content: msg.Content, ts: time.Now()})
		case models.RoleAssistant:
			m.entries = append(m.entries, entry{role: "assistant", content: msg.Content, ts: time.Now()})
		}
	}
	m.addSys("Loaded session: " + m.sessions[i].Title)
	m.render()
	m.vp.GotoBottom()
}

func (m *model) autoTitle(prompt string) {
	if m.sessionTitle != "" {
		return
	}
	t := strings.ReplaceAll(prompt, "\n", " ")
	t = strings.TrimSpace(t)
	if len(t) > 30 {
		t = t[:30] + "…"
	}
	if t != "" {
		m.sessionTitle = t
		m.addSys("Session: " + m.sessionTitle)
	}
}

func (m *model) newSession(confirm bool) tea.Cmd {
	if confirm && !m.confirmAndReset("new") {
		m.addSys("Start a new session? Press Ctrl+N again to confirm.")
		return nil
	}
	m.confirmed = false
	m.confirmAction = ""
	m.messages = nil
	m.entries = nil
	m.cost = 0
	m.tok = 0
	m.sessionTitle = ""
	m.saveSession()
	m.splash()
	m.render()
	return nil
}

func (m *model) exportTranscript() tea.Cmd {
	var sb strings.Builder
	sb.WriteString("# Delta Code transcript\n\n")
	if m.sessionTitle != "" {
		sb.WriteString("**Session:** " + m.sessionTitle + "\n")
	}
	sb.WriteString(fmt.Sprintf("**Model:** %s | **Provider:** %s\n", m.modelName, m.provName))
	sb.WriteString(fmt.Sprintf("**Cost:** $%.4f | **Tokens:** %d\n\n", m.cost, m.tok))
	for _, e := range m.entries {
		switch e.role {
		case "user":
			sb.WriteString("## You\n\n" + e.content + "\n\n")
		case "assistant":
			sb.WriteString("## Delta\n\n" + e.content + "\n\n")
		case "error":
			sb.WriteString("## Error\n\n" + e.content + "\n\n")
		}
	}
	name := "delta-transcript.md"
	if m.sessionTitle != "" {
		clean := strings.Map(func(r rune) rune {
			if r == ' ' || r == ':' || r == '/' || r == '\\' || r == '|' {
				return '-'
			}
			if r < 32 {
				return -1
			}
			return r
		}, m.sessionTitle)
		name = "delta-" + clean + ".md"
	}
	path := name
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		m.addSys("Export failed: " + err.Error())
		return nil
	}
	m.toastNow("Exported to " + path)
	return nil
}

func (m *model) copyTranscript() tea.Cmd {
	var sb strings.Builder
	for _, e := range m.entries {
		switch e.role {
		case "user":
			sb.WriteString("You: " + e.content + "\n\n")
		case "assistant":
			sb.WriteString("Delta: " + e.content + "\n\n")
		}
	}
	if err := clipboardWrite(sb.String()); err != nil {
		m.addSys("Copy failed: " + err.Error())
		return nil
	}
	m.toastNow("Transcript copied to clipboard")
	return nil
}

func (m *model) searchChat(query string) {
	q := strings.ToLower(query)
	found := 0
	for _, e := range m.entries {
		if e.role != "user" && e.role != "assistant" {
			continue
		}
		content := e.content
		if strings.Contains(strings.ToLower(content), q) {
			found++
			line := strings.ReplaceAll(content, "\n", " ")
			idx := strings.Index(strings.ToLower(line), q)
			if idx > 40 {
				line = "…" + line[idx-40:]
			}
			if len(line) > 120 {
				line = line[:120] + "…"
			}
			who := "you"
			if e.role == "assistant" {
				who = "Δ"
			}
			m.addSys(fmt.Sprintf("  %s %s", m.t.fk.Render(who+":"), line))
			if found >= 10 {
				break
			}
		}
	}
	if found == 0 {
		m.addSys("No matches for \"" + query + "\"")
	} else {
		m.addSys(fmt.Sprintf("━━━ %d match(es) ━━━", found))
	}
}

func (m *model) showStats() {
	userMsgs, asstMsgs := 0, 0
	var words int
	var codeBlocks int
	for _, e := range m.entries {
		switch e.role {
		case "user":
			userMsgs++
		case "assistant":
			asstMsgs++
		}
		if e.role == "assistant" {
			words += len(strings.Fields(e.content))
			codeBlocks += len(extractCodeBlocks(e.content))
		}
	}
	m.addSys("━━━ Session stats ━━━")
	m.addSys(fmt.Sprintf("Messages: %d (user %d · Δ %d)", userMsgs+asstMsgs, userMsgs, asstMsgs))
	m.addSys(fmt.Sprintf("Words generated: %d", words))
	m.addSys(fmt.Sprintf("Code blocks: %d", codeBlocks))
	m.addSys(fmt.Sprintf("Tokens: %d | Cost: $%.4f", m.tok, m.cost))
	m.addSys(fmt.Sprintf("Session: %s", m.sessionTitle))
	m.addSys(fmt.Sprintf("Model: %s | Provider: %s", m.modelName, m.provName))
	m.addSys(fmt.Sprintf("Avg tokens/msg: %d", avgTokens(m.tok, max(asstMsgs, 1))))
}

func avgTokens(tok, msgs int) int {
	if msgs == 0 {
		return 0
	}
	return tok / msgs
}
