package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = v.Width
		m.h = v.Height
		vpH := m.h - 9
		if vpH < 5 {
			vpH = 5
		}
		m.vp.Width = v.Width - 2
		m.vp.Height = vpH
		m.ta.SetWidth(v.Width - 6)
		m.ta.SetHeight(3)
		m.render()
		return m, nil

	case tea.MouseMsg:
		m.vp, _ = m.vp.Update(msg)
		return m, nil

	case tea.KeyMsg:
		if v.String() == "ctrl+c" {
			if m.streaming {
				m.cancelStream()
				m.addSys("Cancelled.")
				return m, nil
			}
			if m.quitConfirm {
				m.saveSession()
				return m, tea.Quit
			}
			m.quitConfirm = true
			m.addSys("Press Ctrl+C again to quit.")
			return m, nil
		}
		m.quitConfirm = false
		if m.streaming {
			if v.String() == "esc" {
				m.cancelStream()
				m.addSys("Cancelled.")
			}
			return m, nil
		}
		return m, m.onKey(v)

	case spinner.TickMsg:
		m.sp, _ = m.sp.Update(msg)
		return m, nil

	case chunk:
		return m, m.onChunk(v)

	case tick:
		m.dotTick = (m.dotTick + 1) % 4
		m.render()
		if m.streaming {
			return m, m.tick()
		}
		return m, nil

	}

	return m, nil
}

func (m *model) onKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		t := strings.TrimSpace(m.ta.Value())
		if t == "" {
			return nil
		}
		m.inputHistory = append(m.inputHistory, t)
		m.historyIdx = -1
		m.ta.Reset()
		if strings.HasPrefix(t, "/") {
			return m.slash(t)
		}
		m.ta.Blur()
		return m.submit(t)

	case "up":
		if m.ta.Focused() && len(m.inputHistory) > 0 {
			if m.historyIdx == -1 {
				m.historyIdx = len(m.inputHistory) - 1
			} else if m.historyIdx > 0 {
				m.historyIdx--
			}
			m.ta.SetValue(m.inputHistory[m.historyIdx])
			return nil
		}
		m.vp, _ = m.vp.Update(msg)
		return nil

	case "down":
		if m.ta.Focused() && m.historyIdx >= 0 {
			m.historyIdx++
			if m.historyIdx >= len(m.inputHistory) {
				m.historyIdx = -1
				m.ta.Reset()
				return nil
			}
			m.ta.SetValue(m.inputHistory[m.historyIdx])
			return nil
		}
		m.vp, _ = m.vp.Update(msg)
		return nil

	case "tab":
		if m.ta.Focused() {
			m.ta.Blur()
		} else {
			m.ta.Focus()
		}
		return nil

	case "ctrl+l":
		m.entries = nil
		m.render()
		return nil

	case "home":
		m.vp.GotoTop()
		return nil
	case "end":
		m.vp.GotoBottom()
		return nil

	case "j":
		if !m.ta.Focused() {
			m.vp.LineDown(1)
			return nil
		}
	case "k":
		if !m.ta.Focused() {
			m.vp.LineUp(1)
			return nil
		}
	case "g", "G":
		if !m.ta.Focused() {
			m.vp.GotoBottom()
			return nil
		}

	case "ctrl+d":
		if !m.ta.Focused() {
			m.vp.HalfViewDown()
			return nil
		}
	case "ctrl+u":
		if !m.ta.Focused() {
			m.vp.HalfViewUp()
			return nil
		}

	case "e":
		if !m.ta.Focused() {
			return m.editLastUser()
		}
	case "r":
		if !m.ta.Focused() {
			return m.resendLastUser()
		}
	case "d":
		if !m.ta.Focused() {
			return m.deleteMsg()
		}
	case "c":
		if !m.ta.Focused() {
			return m.copyMsg()
		}
	case "t":
		if !m.ta.Focused() {
			m.toggleReasoning()
			return nil
		}
	case "y":
		if !m.ta.Focused() {
			return m.yankCodeBlock()
		}
	case "w":
		if !m.ta.Focused() {
			return m.writeCodeBlock()
		}
	}

	if !m.ta.Focused() {
		m.ta.Focus()
	}
	m.ta, _ = m.ta.Update(msg)
	return nil
}

func (m *model) findLastIdx(role string) int {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == role {
			return i
		}
	}
	return -1
}

func (m *model) editLastUser() tea.Cmd {
	idx := m.findLastIdx("user")
	if idx < 0 {
		return nil
	}
	m.ta.SetValue(m.entries[idx].content)
	m.entries = m.entries[:idx]
	m.ta.Focus()
	return nil
}

func (m *model) resendLastUser() tea.Cmd {
	idx := m.findLastIdx("user")
	if idx < 0 {
		return nil
	}
	prompt := m.entries[idx].content
	m.entries = m.entries[:idx]
	m.ta.Reset()
	m.ta.Blur()
	return m.submit(prompt)
}

func (m *model) deleteMsg() tea.Cmd {
	idx := m.findLastIdx("user")
	if idx < 0 {
		return nil
	}
	if idx+1 < len(m.entries) && m.entries[idx+1].role == "assistant" {
		m.entries = append(m.entries[:idx], m.entries[idx+2:]...)
	} else {
		m.entries = append(m.entries[:idx], m.entries[idx+1:]...)
	}
	m.render()
	return nil
}

func (m *model) copyMsg() tea.Cmd {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == "assistant" || m.entries[i].role == "user" {
			content := m.entries[i].content
			if err := clipboardWrite(content); err != nil {
				m.addSys("Copy failed: " + err.Error())
			} else {
				m.addSys("Copied to clipboard.")
			}
			return nil
		}
	}
	return nil
}

func (m *model) slash(cmd string) tea.Cmd {
	p := strings.Fields(strings.ToLower(cmd))
	switch p[0] {
	case "/clear":
		m.entries = nil
		m.render()
	case "/help":
		m.addSys("━━━ Commands ━━━")
		m.addSys("/clear      Clear all messages")
		m.addSys("/cost       Show session cost & tokens")
		m.addSys("/model N    Switch model")
		m.addSys("/think      Toggle reasoning display")
		m.addSys("/help       Show this help")
		m.addSys("")
		m.addSys("━━━ Keys ━━━")
		m.addSys("Enter      Send message")
		m.addSys("Esc        Stop streaming")
		m.addSys("Tab        Focus chat/input")
		m.addSys("↑↓         Input history (when focused)")
		m.addSys("j/k        Scroll chat (when unfocused)")
		m.addSys("g/G        Scroll to top/bottom")
		m.addSys("E          Edit last message")
		m.addSys("R          Resend last message")
		m.addSys("D          Delete last user message")
		m.addSys("C          Copy last message")
		m.addSys("Ctrl+L     Clear all")
		m.addSys("Ctrl+C     Quit")
	case "/cost":
		m.addSys(fmt.Sprintf("Cost: $%.4f  |  Tokens: %d", m.cost, m.tok))
	case "/model":
		if len(p) > 1 {
			m.modelName = p[1]
			m.addSys("Switched to: " + p[1])
		} else {
			m.addSys("Current: " + m.modelName)
		}
	case "/think":
		m.addSys("Reasoning: " + m.t.fd.Render("always show"))
	default:
		m.addSys("Unknown: " + cmd + "  (try /help)")
	}
	return nil
}

func (m *model) submit(prompt string) tea.Cmd {
	m.lastPrompt = prompt
	m.addUser(prompt)
	m.statusText = "Thinking..."
	m.startTime = time.Now()
	m.streaming = true
	m.quitConfirm = false
	m.stopCh = make(chan struct{})
	m.sb.Reset()
	m.rb.Reset()
	m.render()
	m.vp.GotoBottom()
	m.atBottom = true

	conf := m.cfg.GetConfig()
	pName, modelName := m.rtr.Route(prompt, conf.Providers)
	m.modelName = modelName
	m.provName = pName

	provCfg, err := m.cfg.GetProvider(pName)
	if err != nil {
		provCfg, err = m.cfg.GetProvider(conf.DefaultProvider)
	}
	if err != nil || provCfg == nil {
		m.streaming = false
		m.statusText = "Error"
		m.addErr("No provider configured. Use `delta provider add` first.")
		m.ta.Focus()
		return nil
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		m.streaming = false
		m.statusText = "Error"
		m.addErr("Provider error: " + err.Error())
		m.ta.Focus()
		return nil
	}

	ctxPrompt := prompt
	if m.ctxEng != nil {
		ctxPrompt = m.ctxEng.BuildPrompt(prompt)
	}

	msgs := []models.Message{
		{Role: models.RoleSystem, Content: "You are Delta Code, an expert software engineer. Write production-ready code. Be concise and precise. Use markdown with code blocks."},
	}
	if m.vector != nil {
		if res := m.vector.Search(prompt, 3); len(res) > 0 {
			var sb strings.Builder
			sb.WriteString("Context from previous sessions:\n")
			for i, r := range res {
				c := r.Entry.Content
				if len(c) > 500 {
					c = c[:500] + "..."
				}
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c))
			}
			msgs = append(msgs, models.Message{Role: models.RoleSystem, Content: sb.String()})
		}
	}

	if len(m.messages) > 0 {
		start := 0
		if len(m.messages) > 10 {
			start = len(m.messages) - 10
		}
		msgs = append(msgs, m.messages[start:]...)
	}

	msgs = append(msgs, models.Message{Role: models.RoleUser, Content: ctxPrompt})

	req := models.ChatRequest{
		Model:       modelName,
		Messages:    msgs,
		Temperature: 0.3,
		MaxTokens:   8192,
	}

	ch := make(chan chunk, 100)
	m.streamCh = ch
	stop := m.stopCh

	go m.streamWorker(ch, stop, req, p)

	return tea.Batch(m.nextChunk(), m.tick())
}

func (m *model) yankCodeBlock() tea.Cmd {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == "assistant" && m.entries[i].content != "" {
			lang, err := copyCodeBlock(m.entries[i].content)
			if err != nil {
				m.addSys("Copy: " + err.Error())
			} else if lang != "" {
				m.addSys("Copied " + lang + " code to clipboard.")
			}
			return nil
		}
	}
	return nil
}

func (m *model) writeCodeBlock() tea.Cmd {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == "assistant" && m.entries[i].content != "" {
			path, err := saveCodeBlock(m.entries[i].content, ".")
			if err != nil {
				m.addSys("Save: " + err.Error())
			} else {
				m.addSys("Created " + path)
			}
			return nil
		}
	}
	return nil
}

func (m *model) toggleReasoning() {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == "assistant" && m.entries[i].reasoning != "" {
			m.entries[i].showReasoning = !m.entries[i].showReasoning
			m.render()
			return
		}
	}
}

func (m *model) cancelStream() {
	m.streaming = false
	m.statusText = "Cancelled"
	m.ta.Focus()
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	m.render()
}

func (m *model) storeMemory(prompt, resp string) {
	if m.mem != nil {
		m.mem.AddEntry(m.sid, "user", prompt, nil)
		m.mem.AddEntry(m.sid, "assistant", resp, map[string]any{"source": "tui"})
	}
	if m.vector != nil {
		m.vector.Store(fmt.Sprintf("Q: %s\nA: %s", prompt, resp), tag(prompt))
	}
}

func tag(text string) []string {
	f := strings.Fields(strings.ToLower(text))
	var out []string
	seen := map[string]bool{}
	for _, w := range f {
		if len(w) > 3 && !seen[w] {
			out = append(out, w)
			seen[w] = true
		}
		if len(out) >= 5 {
			break
		}
	}
	return out
}
