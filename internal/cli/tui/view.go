package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) View() string {
	if m.wsData != nil && m.wsData.shown {
		return m.workspaceView()
	}
	m.render()
	parts := []string{
		m.header(),
		m.t.sep.Render("─" + strings.Repeat("─", max(m.w-2, 0)) + "─"),
		m.vp.View(),
	}
	if dd := m.dropdownView(); dd != "" {
		parts = append(parts, dd)
	}
	if m.helpShown {
		parts = append(parts, m.keysHelp())
	}
	parts = append(parts, m.ta.View(), m.status(), m.footer())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// keysHelp renders the keyboard shortcut overlay.
func (m *model) keysHelp() string {
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" KEYBOARD SHORTCUTS "))
	pairs := [][2]string{
		{"Enter", "send prompt"},
		{"Shift+Enter", "newline"},
		{"Ctrl+K", "command palette"},
		{"Ctrl+M", "switch model"},
		{"Ctrl+P", "switch provider"},
		{"Ctrl+N", "new session"},
		{"Ctrl+S", "save session"},
		{"Ctrl+W", "workspace view"},
		{"Ctrl+H", "this help"},
		{"Ctrl+L", "clear conversation"},
		{"↑/↓", "history / scroll"},
		{"j/k", "scroll"},
		{"g", "jump to bottom"},
		{"Space", "lock scroll"},
		{"Tab", "focus input"},
		{"Esc", "stop streaming"},
		{"Ctrl+C", "quit (press twice)"},
	}
	for _, p := range pairs {
		lines = append(lines, fmt.Sprintf(" %s  %s", m.t.fk.Render(p[0]), m.t.fd.Render(p[1])))
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m *model) header() string {
	l := m.t.head.Render(" Δ ")
	i := m.t.subh.Render(fmt.Sprintf(" %s • %s ", m.modelName, m.provName))
	if m.minimal {
		i = m.t.subh.Render(" " + m.modelName + " ")
	}
	var extra string
	if w := m.ws(); w.branch != "" && !m.minimal {
		extra = m.t.stat.Render(" " + w.branch + " ")
	}
	ts := m.t.stat.Render(time.Now().Format("Jan 02 15:04"))
	sp := m.w - lipgloss.Width(l) - lipgloss.Width(i) - lipgloss.Width(ts) - lipgloss.Width(extra) - 4
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, l, strings.Repeat(" ", sp), extra, i, ts)
}

func (m *model) status() string {
	p := ""
	if m.streaming {
		p = m.sp.View() + " "
	}
	inputLen := len(m.ta.Value())
	inputInfo := ""
	if inputLen > 0 {
		inputInfo = m.t.dim.Render(fmt.Sprintf(" %dch ", inputLen))
	}
	var lock string
	if m.scrollLocked && !m.streaming {
		lock = m.t.badge.Render("  lock ")
	}
	var toast string
	if m.toast != nil {
		toast = m.t.badge.Render("  " + m.toast.text + " ")
	}
	l := m.t.stat.Render(fmt.Sprintf(" %s%s%s%s", p, m.statusText, inputInfo, lock))
	r := m.t.dim.Render(fmt.Sprintf("msgs:%d  $%.4f  %dtok", len(m.entries), m.cost, m.tok))
	if toast != "" {
		r = toast + " " + r
	}
	sp := m.w - lipgloss.Width(l) - lipgloss.Width(r) - 2
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, l, strings.Repeat(" ", sp), r)
}

func (m *model) footer() string {
	if m.minimal {
		return m.t.brd.Render(strings.Repeat("─", max(0, m.w-2)))
	}
	keys := []struct{ k, d string }{
		{"Enter", "send"}, {"Ctrl+K", "palette"}, {"Ctrl+M", "model"}, {"Ctrl+P", "provider"},
		{"↑↓", "hist"}, {"j/k", "scroll"}, {"/help", "cmds"}, {"Esc", "stop"}, {"Ctrl+C", "quit"},
	}
	var p []string
	for _, x := range keys {
		p = append(p, m.t.fk.Render(x.k)+" "+m.t.fd.Render(x.d))
	}
	return fmt.Sprintf("%s\n %s", m.t.brd.Render(strings.Repeat("─", max(0, m.w-2))), strings.Join(p, "  "))
}

func (m *model) render() {
	var out []string
	atEnd := m.vp.ScrollPercent() >= 0.99
	if !m.scrollLocked {
		m.atBottom = atEnd
	}

	if !atEnd && len(m.entries) > 3 {
		rem := m.vp.TotalLineCount() - m.vp.YOffset
		out = append(out, m.t.scr.Render(fmt.Sprintf("  ↑ %d more — press g to jump  ", rem)))
		out = append(out, "")
	}

	// Dim entries older than the last 4.
	activeFrom := 0
	for i, e := range m.entries {
		if e.role == "user" || e.role == "assistant" {
			activeFrom = i
			if len(m.entries)-i < 6 {
				break
			}
		}
	}

	for i := range m.entries {
		e := &m.entries[i]
		dimmed := i < activeFrom && !m.minimal
		switch e.role {
		case "user":
			style := m.t.uP
			if dimmed {
				style = style.Foreground(lipgloss.Color("242"))
			}
			out = append(out, style.String())
			for _, line := range strings.Split(e.content, "\n") {
				if dimmed {
					out = append(out, m.t.dim.Render("  "+line))
				} else {
					out = append(out, m.t.uM.Render("  "+line))
				}
			}
		case "assistant":
			ap := m.t.aP
			if dimmed {
				ap = ap.Foreground(lipgloss.Color("242"))
			}
			out = append(out, ap.String())
			if e.reasoning != "" && !m.minimal {
				if e.showReasoning && m.reasoningVisible {
					out = append(out, m.t.badge.Render(fmt.Sprintf(" ▼ %d tok reasoning", len(strings.Fields(e.reasoning)))))
					out = append(out, "")
					for _, line := range strings.Split(e.reasoning, "\n") {
						if line != "" {
							out = append(out, m.t.dim.Italic(true).Render("  "+line))
						}
					}
					out = append(out, "")
					out = append(out, m.t.dim.Render("  ── response ──"))
					out = append(out, "")
				} else {
					out = append(out, m.t.badge.Render(fmt.Sprintf(" ▶ %d tok reasoning", len(strings.Fields(e.reasoning)))))
				}
			}
			body := e.content
			if e.collapsed && m.collapseLong && len(body) > 400 && !m.scrollLocked && false {
				// (reserved for per-entry collapse toggling)
			}
			if m.wrapEnabled {
				if body != "" && !e.mdCached {
					e.mdCache = m.renderMD(body)
					e.mdCached = true
				}
				if e.mdCached {
					body = e.mdCache
				}
				out = append(out, body)
			} else {
				out = append(out, m.renderPlainNoWrap(body))
			}
			if e.duration > 0 || e.tokens > 0 || e.cost > 0 || !e.ts.IsZero() {
				var parts []string
				if !e.ts.IsZero() {
					parts = append(parts, e.ts.Format("15:04"))
				}
				if e.duration > 0 {
					parts = append(parts, fmt.Sprintf("%.1fs", e.duration.Seconds()))
				}
				if e.tokens > 0 {
					parts = append(parts, fmt.Sprintf("%d tok", e.tokens))
				}
				if e.cost > 0 {
					parts = append(parts, fmt.Sprintf("$%.4f", e.cost))
				}
				if e.model != "" {
					parts = append(parts, e.model)
				}
				out = append(out, m.t.meta.Render("  "+strings.Join(parts, " · ")))
			}
		case "streaming":
			out = append(out, m.t.aP.String())
			out = append(out, m.streamRender(e.content))
		case "system":
			out = append(out, m.t.sysM.Render("  "+e.content))
		case "error":
			out = append(out, m.t.errM.Render("  ✗ "+e.content))
		}
		out = append(out, "")
	}

	if m.streaming {
		elapsed := time.Since(m.startTime).Seconds()
		dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		stat := fmt.Sprintf(" %s Thinking  %.1fs ", dots[m.dotTick%len(dots)], elapsed)
		if m.tok > 0 {
			stat += fmt.Sprintf(" %dtok $%.4f ", m.tok, m.cost)
		}
		out = append(out, m.t.badge.Render(stat))
	}

	m.vp.SetContent(strings.Join(out, "\n"))
}

// clearMD invalidates cached markdown bodies (needed on theme changes).
func (m *model) clearMD() {
	for i := range m.entries {
		m.entries[i].mdCached = false
	}
}

func (m *model) renderPlainNoWrap(content string) string {
	s := lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("252"))
	var out []string
	for _, line := range strings.Split(content, "\n") {
		out = append(out, s.Render("  "+line))
	}
	return strings.Join(out, "\n")
}

func (m *model) renderMD(content string) string {
	if m.glam == nil {
		return plainMD(content)
	}
	r, err := m.glam.Render(content)
	if err != nil || r == "" {
		return plainMD(content)
	}
	return m.addCodeBadges(r, content)
}

func (m *model) addCodeBadges(glamRendered, rawContent string) string {
	blocks := extractCodeBlocks(rawContent)
	if len(blocks) == 0 {
		return glamRendered
	}

	var result []string
	lines := strings.Split(glamRendered, "\n")
	blockIdx := 0

	for _, line := range lines {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "───") {
			result = append(result, line)
			continue
		}
		if strings.Contains(stripped, "──────────") && blockIdx < len(blocks) {
			b := blocks[blockIdx]
			label := " " + b.language + " "
			if b.language == "" {
				label = " code "
			}
			if b.filename != "" {
				label = " " + b.filename + " "
			}
			result = append(result, m.t.codeL.Render(label))
			if len(blocks) > 1 {
				result = append(result, m.t.badge.Render(fmt.Sprintf(" y%d)copy", blockIdx+1)))
				if b.filename != "" {
					result = append(result, m.t.badge.Render(fmt.Sprintf(" s%d)save", blockIdx+1)))
				}
			} else {
				result = append(result, m.t.badge.Render(" y)copy"))
				if b.filename != "" {
					result = append(result, m.t.badge.Render(" w)save"))
				}
			}
			blockIdx++
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func plainMD(content string) string {
	s := lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("252"))
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "```") {
			continue
		}
		out = append(out, s.Render("  "+line))
	}
	return strings.Join(out, "\n")
}

func (m *model) streamRender(content string) string {
	var out []string
	lines := strings.Split(content, "\n")
	inCode := false
	var code []string
	lang := ""

	flush := func() {
		if len(code) == 0 {
			return
		}
		l := " code "
		if lang != "" {
			l = " " + lang + " "
		}
		out = append(out, m.t.codeL.Render(l))
		if lang != "" {
			fn := extractFilename(lang, code)
			if fn != "" {
				out = append(out, m.t.badge.Render(" "+fn+" "))
			}
		}
		for _, ln := range code {
			out = append(out, m.t.codeBg.Render("  "+ln))
		}
		code = nil
		lang = ""
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			flush()
			inCode = !inCode
			if inCode {
				lang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
			}
			continue
		}
		if inCode {
			code = append(code, line)
			continue
		}
		flush()
		if line == "" {
			out = append(out, "")
		} else {
			out = append(out, m.t.dim.Render("  "+line))
		}
	}
	flush()
	return strings.Join(out, "\n")
}

func (m *model) addUser(c string) {
	m.messages = append(m.messages, models.Message{Role: models.RoleUser, Content: c})
	m.entries = append(m.entries, entry{role: "user", content: c, ts: time.Now()})
}

func (m *model) addAsst(c string, reasoning string) {
	m.entries = append(m.entries, entry{
		role:          "assistant",
		content:       c,
		reasoning:     reasoning,
		duration:      time.Since(m.startTime),
		tokens:        m.tok,
		cost:          m.cost,
		showReasoning: m.reasoningVisible,
		ts:            time.Now(),
		model:         m.modelName,
	})
}

func (m *model) addStream(c string) {
	if n := len(m.entries); n > 0 && m.entries[n-1].role == "streaming" {
		m.entries[n-1].content += c
	} else {
		m.entries = append(m.entries, entry{role: "streaming", content: c})
	}
}

func (m *model) addSys(c string) {
	m.entries = append(m.entries, entry{role: "system", content: c})
}

func (m *model) addErr(c string) {
	m.entries = append(m.entries, entry{role: "error", content: c})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
