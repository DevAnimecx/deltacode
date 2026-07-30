package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) View() string {
	m.render()
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header(),
		m.t.sep.Render("─"+strings.Repeat("─", max(m.w-2, 0))+"─"),
		m.vp.View(),
		m.ta.View(),
		m.status(),
		m.footer(),
	)
}

func (m *model) header() string {
	l := m.t.head.Render(" Δ ")
	i := m.t.subh.Render(fmt.Sprintf(" %s • %s ", m.modelName, m.provName))
	ts := m.t.stat.Render(time.Now().Format("15:04:05"))
	sp := m.w - lipgloss.Width(l) - lipgloss.Width(i) - lipgloss.Width(ts) - 4
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, l, strings.Repeat(" ", sp), i, ts)
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
	l := m.t.stat.Render(fmt.Sprintf(" %s%s%s", p, m.statusText, inputInfo))
	r := m.t.dim.Render(fmt.Sprintf("msgs:%d  $%.4f  %dtok", len(m.entries), m.cost, m.tok))
	sp := m.w - lipgloss.Width(l) - lipgloss.Width(r) - 2
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, l, strings.Repeat(" ", sp), r)
}

func (m *model) footer() string {
	keys := []struct{ k, d string }{
		{"Enter", "send"}, {"Esc", "stop"}, {"Tab", "focus"},
		{"↑↓", "hist"}, {"↑↓", "scroll"}, {"j/k", "scroll"},
		{"/help", "cmds"}, {"Ctrl+L", "clear"}, {"Ctrl+C", "quit"},
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
	m.atBottom = atEnd

	if !atEnd && len(m.entries) > 3 {
		rem := m.vp.TotalLineCount() - m.vp.YOffset
		out = append(out, m.t.scr.Render(fmt.Sprintf("  ↑ %d more  ", rem)))
		out = append(out, "")
	}

	for _, e := range m.entries {
		switch e.role {
		case "user":
			out = append(out, m.t.uP.String())
			for _, line := range strings.Split(e.content, "\n") {
				out = append(out, m.t.uM.Render("  "+line))
			}
		case "assistant":
			out = append(out, m.t.aP.String())
			if e.reasoning != "" {
				if e.showReasoning {
					out = append(out, m.t.badge.Render(fmt.Sprintf(" ▼ %d tok reasoning", len(strings.Fields(e.reasoning)))))
					out = append(out, "")
					for _, line := range strings.Split(e.reasoning, "\n") {
						if line != "" {
							out = append(out, m.t.dim.Italic(true).Render("  " + line))
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
			if e.collapsed && len(body) > 200 {
				body = body[:200] + "\n\n" + m.t.dim.Render("  [▼ " + fmt.Sprintf("%d more chars]", len(body)-200))
			}
			out = append(out, m.renderMD(body))
			if e.duration > 0 || e.tokens > 0 || e.cost > 0 {
				var parts []string
				if e.duration > 0 {
					parts = append(parts, fmt.Sprintf("%.1fs", e.duration.Seconds()))
				}
				if e.tokens > 0 {
					parts = append(parts, fmt.Sprintf("%d tok", e.tokens))
				}
				if e.cost > 0 {
					parts = append(parts, fmt.Sprintf("$%.4f", e.cost))
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
		out = append(out, m.t.badge.Render(fmt.Sprintf(" %s Thinking  %.1fs ", dots[m.dotTick%len(dots)], elapsed)))
	}

	m.vp.SetContent(strings.Join(out, "\n"))
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
			result = append(result, m.t.badge.Render(" y)copy"))
			if b.filename != "" {
				result = append(result, m.t.badge.Render(" w)save"))
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
				out = append(out, m.t.badge.Render(" " + fn + " "))
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
	m.entries = append(m.entries, entry{role: "user", content: c})
}

func (m *model) addAsst(c string, reasoning string) {
	m.entries = append(m.entries, entry{
		role: "assistant", content: c, reasoning: reasoning,
		duration: time.Since(m.startTime), tokens: m.tok, cost: m.cost,
		showReasoning: true,
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
