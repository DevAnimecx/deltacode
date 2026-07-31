package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// workspace holds live engineering-session data for the workspace view.
type workspace struct {
	project   string
	branch    string
	commits   []string
	modified  []string
	newFiles  []string
	deleted   []string
	notifs    []string
	badge     string
	goal      string
	taskTitle string
	shown     bool
	refreshed time.Time
}

func (w *workspace) gitRefresh() {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err == nil {
		w.branch = strings.TrimSpace(string(out))
	}
	if w.branch == "" {
		out, err = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err == nil {
			w.branch = strings.TrimSpace(string(out))
		}
	}
	out, err = exec.Command("git", "status", "--short").Output()
	if err == nil {
		modified, newf, del := []string{}, []string{}, []string{}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			path := line
			if len(line) > 3 {
				path = strings.TrimSpace(line[3:])
			}
			switch {
			case strings.HasPrefix(line, "M") || strings.HasPrefix(line, "??"):
				modified = append(modified, path)
			case strings.HasPrefix(line, "A"):
				newf = append(newf, path)
			case strings.HasPrefix(line, "D"):
				del = append(del, path)
			}
		}
		w.modified, w.newFiles, w.deleted = modified, newf, del
	}
	out, err = exec.Command("git", "log", "--oneline", "-3").Output()
	if err == nil {
		w.commits = strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(w.commits) == 1 && w.commits[0] == "" {
			w.commits = nil
		}
	}
	w.refreshed = time.Now()
}

func (m *model) ws() *workspace {
	if m.wsData == nil {
		m.wsData = &workspace{project: m.projectName(), badge: "Ready"}
		m.wsData.gitRefresh()
	}
	if time.Since(m.wsData.refreshed) > 30*time.Second {
		m.wsData.gitRefresh()
	}
	return m.wsData
}

func (m *model) projectName() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		dir := strings.TrimSpace(string(out))
		if i := strings.LastIndexAny(dir, `/\`); i != -1 {
			return dir[i+1:]
		}
		return dir
	}
	return "."
}

// currentBadge maps the live state to a badge label.
func (m *model) currentBadge() string {
	if m.streaming {
		if m.lastPrompt != "" {
			return "Generating"
		}
		return "Thinking"
	}
	if len(m.entries) == 0 {
		return "Idle"
	}
	return "Ready"
}

// progressRing renders an ASCII ring at progress p (0..1).
func progressRing(p float64, width int) string {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	filled := int(float64(width) * p)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %3.0f%%", bar, p*100)
}

// goalCard renders the live goal + progress block.
func (m *model) goalCard() string {
	ws := m.ws()
	goal := ws.goal
	if goal == "" {
		goal = "Awaiting your next prompt"
	}
	p := 0.0
	if len(m.entries) > 1 {
		done := 0
		for _, e := range m.entries {
			if e.role == "assistant" || e.role == "system" {
				done++
			}
		}
		p = float64(done) / float64(len(m.entries))
	}
	status := ws.badge
	ring := progressRing(p, 26)

	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" GOAL "),
		" " + goal,
		"",
		fmt.Sprintf(" Status: %s", m.t.badge.Render(" "+status+" ")),
		fmt.Sprintf(" %s", ring),
		fmt.Sprintf(" ETA: %s   Elapsed: %s", m.etaText(), m.durationText()),
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m *model) etaText() string {
	elapsed := time.Since(m.startTime)
	if elapsed < 30*time.Second {
		return "estimating…"
	}
	if len(m.entries) < 2 {
		return "estimating…"
	}
	return "~" + elapsed.Round(time.Second).String()
}

func (m *model) durationText() string {
	return time.Since(m.startTime).Round(time.Second).String()
}

// taskCards renders live task status derived from the session.
func (m *model) taskCards() string {
	ws := m.ws()
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" TASKS "))
	if m.streaming {
		lines = append(lines, fmt.Sprintf("  ⟳ Running  %s", truncateStr(m.lastPrompt, 40)))
	} else {
		lines = append(lines, "  – Idle")
	}
	for _, n := range ws.notifs {
		lines = append(lines, "  • "+truncateStr(n, 60))
	}
	return style.Render(strings.Join(lines, "\n"))
}

// fileTree renders modified/new/deleted files.
func (m *model) fileTree() string {
	ws := m.ws()
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" FILES "))
	lines = append(lines, fmt.Sprintf(" %d modified    %d new    %d deleted",
		len(ws.modified), len(ws.newFiles), len(ws.deleted)))
	shown := 0
	for _, f := range ws.modified {
		if shown >= 8 {
			lines = append(lines, "  …")
			break
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(" M ")+" "+f)
		shown++
	}
	for _, f := range ws.newFiles {
		if shown >= 8 {
			lines = append(lines, "  …")
			break
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("43")).Render(" A ")+" "+f)
		shown++
	}
	if shown == 0 {
		lines = append(lines, "  (clean)")
	}
	return style.Render(strings.Join(lines, "\n"))
}

// gitCard renders branch + recent commits.
func (m *model) gitCard() string {
	ws := m.ws()
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" GIT "))
	lines = append(lines, fmt.Sprintf(" branch: %s", lipgloss.NewStyle().Bold(true).Render(ws.branch)))
	for i, c := range ws.commits {
		if i >= 3 {
			break
		}
		lines = append(lines, "  "+truncateStr(c, 56))
	}
	return style.Render(strings.Join(lines, "\n"))
}

// meterCard renders token + cost meters.
func (m *model) meterCard() string {
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" METERS "))
	tokP := 0.0
	if m.tok > 0 {
		tokP = float64(m.tok) / 100000.0
		if tokP > 1 {
			tokP = 1
		}
	}
	costP := 0.0
	if m.cost > 0 {
		costP = m.cost / 1.0
		if costP > 1 {
			costP = 1
		}
	}
	lines = append(lines, fmt.Sprintf(" tokens %s", progressRing(tokP, 22)))
	lines = append(lines, fmt.Sprintf(" cost   %s $%.4f", progressRing(costP, 22), m.cost))
	return style.Render(strings.Join(lines, "\n"))
}

// timeline renders the session timeline.
func (m *model) timeline() string {
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" TIMELINE "))
	if len(m.entries) == 0 {
		lines = append(lines, "  (empty)")
		return style.Render(strings.Join(lines, "\n"))
	}
	start := len(m.entries) - 12
	if start < 0 {
		start = 0
	}
	for _, e := range m.entries[start:] {
		ts := e.ts.Format("15:04:05")
		var mark string
		switch e.role {
		case "user":
			mark = "You"
		case "assistant":
			mark = "Delta"
		case "error":
			mark = "Error"
		default:
			mark = "Event"
		}
		content := truncateStr(strings.ReplaceAll(e.content, "\n", " "), 44)
		if e.role == "assistant" || e.role == "user" {
			lines = append(lines, fmt.Sprintf(" %s %s %s", m.t.dim.Render(ts), mark, content))
		}
	}
	return style.Render(strings.Join(lines, "\n"))
}

// activityFeed renders the scrolling event stream.
func (m *model) activityFeed() string {
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" ACTIVITY "))
	if len(m.entries) == 0 {
		lines = append(lines, "  waiting for activity…")
	}
	start := len(m.entries) - 8
	if start < 0 {
		start = 0
	}
	for _, e := range m.entries[start:] {
		mark := "•"
		switch e.role {
		case "error":
			mark = "✗"
		case "assistant":
			mark = "Δ"
		case "user":
			mark = "›"
		}
		lines = append(lines, fmt.Sprintf(" %s %s", m.t.dim.Render(mark), truncateStr(strings.ReplaceAll(e.content, "\n", " "), 58)))
	}
	return style.Render(strings.Join(lines, "\n"))
}

// notifications renders recent system notifications.
func (m *model) notifications() string {
	ws := m.ws()
	style := m.t.brd.Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.w - 4)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" NOTIFICATIONS "))
	if len(ws.notifs) == 0 {
		lines = append(lines, "  none")
	}
	for _, n := range ws.notifs {
		lines = append(lines, "  • "+truncateStr(n, 60))
	}
	return style.Render(strings.Join(lines, "\n"))
}

// workspaceView renders the full-screen engineering workspace.
func (m *model) workspaceView() string {
	w := m.ws()
	w.badge = m.currentBadge()
	w.goal = m.lastPrompt
	if w.goal == "" {
		w.goal = "Interactive session"
	}

	var header []string
	header = append(header,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" Δ WORKSPACE "),
		fmt.Sprintf(" %s • %s ", w.project, w.branch),
		m.t.dim.Render("provider: "+m.provName),
		m.t.dim.Render("model: "+m.modelName),
		m.t.dim.Render("status: "+w.badge),
		m.t.dim.Render("session: "+m.durationText()),
	)

	cols := lipgloss.JoinVertical(lipgloss.Left,
		m.goalCard(),
		"",
		m.taskCards(),
		"",
		m.meterCard(),
	)
	right := lipgloss.JoinVertical(lipgloss.Left,
		m.fileTree(),
		"",
		m.gitCard(),
	)
	bottom := lipgloss.JoinVertical(lipgloss.Left,
		m.timeline(),
		"",
		m.activityFeed(),
		"",
		m.notifications(),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, header[0], header[1], header[2], header[3], header[4], header[5]),
		m.t.sep.Render("─"+strings.Repeat("─", max(m.w-2, 0))),
		lipgloss.JoinHorizontal(lipgloss.Top, cols, right),
		"",
		bottom,
	)
}

// sessionSummary renders the end-of-session report.
func (m *model) sessionSummary() string {
	ws := m.ws()
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(" ── SESSION SUMMARY ── "))
	lines = append(lines, fmt.Sprintf(" project:      %s (%s)", ws.project, ws.branch))
	lines = append(lines, fmt.Sprintf(" duration:     %s", m.durationText()))
	lines = append(lines, fmt.Sprintf(" exchanges:    %d", m.exchanges))
	lines = append(lines, fmt.Sprintf(" messages:     %d", len(m.entries)))
	lines = append(lines, fmt.Sprintf(" tokens:       %d", m.tok))
	lines = append(lines, fmt.Sprintf(" cost:         $%.4f", m.cost))
	lines = append(lines, fmt.Sprintf(" files touched: %d modified, %d new, %d deleted",
		len(ws.modified), len(ws.newFiles), len(ws.deleted)))
	if len(m.inputHistory) > 0 {
		lines = append(lines, fmt.Sprintf(" history:      %d prompts saved", len(m.inputHistory)))
	}
	return strings.Join(lines, "\n")
}

func (m *model) notify(text string) {
	ws := m.ws()
	ws.notifs = append(ws.notifs, text)
	if len(ws.notifs) > 6 {
		ws.notifs = ws.notifs[len(ws.notifs)-6:]
	}
}

func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
