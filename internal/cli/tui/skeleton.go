package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Style struct {
	Header    lipgloss.Style
	StatusBar lipgloss.Style
	Progress  lipgloss.Style
	Model     lipgloss.Style
	Content   lipgloss.Style
	Separator lipgloss.Style
	Meter     lipgloss.Style
	Highlight lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Error     lipgloss.Style
}

func DefaultStyle() Style {
	return Style{
		Header:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Padding(0, 1),
		StatusBar: lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Padding(0, 1),
		Progress:  lipgloss.NewStyle().Foreground(lipgloss.Color("76")),
		Model:     lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Content:   lipgloss.NewStyle().Padding(0, 1),
		Separator: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Meter:     lipgloss.NewStyle().Foreground(lipgloss.Color("141")),
		Highlight: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.Color("76")),
		Warning:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	}
}

type DashboardModel struct {
	ready       bool
	viewport    viewport.Model
	spinner     spinner.Model
	style       Style
	statusText  string
	modelName   string
	provider    string
	taskType    string
	content     strings.Builder
	logs        []string
	width       int
	height      int
	cost        float64
	tokens      int
	files       int
	linesAdded  int
	linesRemoved int
	progress    float64
	progressLabel string
	showGitDiff bool
	showCost    bool
}

func NewModel() DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	style := DefaultStyle()

	return DashboardModel{
		spinner:    s,
		style:      style,
		statusText: "Ready",
		modelName:  "No model",
		provider:   "—",
		taskType:   "—",
		logs:       []string{},
		showCost:   true,
	}
}

func (m DashboardModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-14)
			m.viewport.YPosition = 8
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 14
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "d":
			m.showGitDiff = !m.showGitDiff
		case "c":
			m.showCost = !m.showCost
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m DashboardModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	style := m.style
	w := m.width

	// Header
	header := style.Header.Render(fmt.Sprintf(" Δ Delta Code"))

	// Dashboard stats row
	stats := lipgloss.JoinHorizontal(lipgloss.Top,
		style.StatusBar.Render(fmt.Sprintf(" Model: %s", style.Model.Render(m.modelName))),
		style.StatusBar.Render(fmt.Sprintf(" Provider: %s", m.provider)),
		style.StatusBar.Render(fmt.Sprintf(" Task: %s", m.taskType)),
	)

	// Cost & Token meter
	meterRow := ""
	if m.showCost {
		costStr := fmt.Sprintf("$%.4f", m.cost)
		meterRow = style.Meter.Render(fmt.Sprintf(" Cost: %s  |  Tokens: %d  |  Files: %d  |  +%d/-%d",
			costStr, m.tokens, m.files, m.linesAdded, m.linesRemoved))
	}

	// Progress bar
	bar := m.renderProgressBar(m.progress, 40)

	// Status + spinner
	statusLine := style.StatusBar.Render(fmt.Sprintf(" %s %s  %s", m.spinner.View(), m.statusText, bar))

	// Separator
	sep := strings.Repeat("─", max(w-2, 0))
	sepLine := style.Separator.Render("─" + sep + "─")

	// Content area
	content := style.Content.Render(m.viewport.View())

	// Footer
	footer := style.StatusBar.Render(
		" [q] quit  |  [d] toggle diff  |  [c] toggle cost  |  ↑↓ scroll")

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s\n%s\n%s\n\n%s\n%s\n",
		header, sepLine, stats, meterRow, statusLine, sepLine, sepLine, content, footer,
	)
}

func (m *DashboardModel) renderProgressBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	label := fmt.Sprintf(" %s %d%%", m.progressLabel, int(pct*100))
	return m.style.Progress.Render(bar + label)
}

func (m *DashboardModel) SetStatus(status string) {
	m.statusText = status
}

func (m *DashboardModel) SetModel(name string) {
	m.modelName = name
}

func (m *DashboardModel) SetProvider(name string) {
	m.provider = name
}

func (m *DashboardModel) SetTaskType(ttype string) {
	m.taskType = ttype
}

func (m *DashboardModel) SetCost(c float64) {
	m.cost = c
}

func (m *DashboardModel) SetTokens(t int) {
	m.tokens = t
}

func (m *DashboardModel) SetFiles(f, added, removed int) {
	m.files = f
	m.linesAdded = added
	m.linesRemoved = removed
}

func (m *DashboardModel) SetProgress(pct float64, label string) {
	m.progress = pct
	m.progressLabel = label
}

func (m *DashboardModel) AppendLog(line string) {
	m.logs = append(m.logs, line)
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m *DashboardModel) RefreshGitDiff() {
	out, err := exec.Command("git", "diff", "--stat").Output()
	if err == nil && len(out) > 0 {
		m.AppendLog("--- Git Diff ---\n" + string(out))
	}
}
