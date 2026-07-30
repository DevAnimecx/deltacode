package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/context"
	"github.com/DevAnimecx/deltacode/internal/memory"
	"github.com/DevAnimecx/deltacode/internal/router"
	"github.com/DevAnimecx/deltacode/internal/skill"
	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type entry struct {
	role           string
	content        string
	reasoning      string
	duration       time.Duration
	tokens         int
	cost           float64
	showReasoning  bool
	showMore       bool
	collapsed      bool
}

type theme struct {
	head   lipgloss.Style
	subh   lipgloss.Style
	stat   lipgloss.Style
	uM     lipgloss.Style
	uP     lipgloss.Style
	aM     lipgloss.Style
	aP     lipgloss.Style
	sysM   lipgloss.Style
	sep    lipgloss.Style
	fk     lipgloss.Style
	fd     lipgloss.Style
	errM   lipgloss.Style
	dim    lipgloss.Style
	brd    lipgloss.Style
	scr    lipgloss.Style
	meta   lipgloss.Style
	logo   lipgloss.Style
	codeBg lipgloss.Style
	codeL  lipgloss.Style
	badge  lipgloss.Style
	link   lipgloss.Style
}

func defTheme() theme {
	c := lipgloss.Color("43")
	t := lipgloss.Color("37")
	s := lipgloss.Color("244")
	return theme{
		head: lipgloss.NewStyle().Bold(true).Foreground(c).Padding(0, 1),
		subh: lipgloss.NewStyle().Foreground(s).Padding(0, 1),
		stat: lipgloss.NewStyle().Foreground(s),
		uM:   lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Padding(0, 2),
		uP:   lipgloss.NewStyle().Bold(true).Foreground(c).SetString("┃ You "),
		aM:   lipgloss.NewStyle().Padding(0, 2),
		aP:   lipgloss.NewStyle().Bold(true).Foreground(t).SetString("Δ "),
		sysM: lipgloss.NewStyle().Foreground(s).Italic(true).Padding(0, 2),
		sep:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		fk:   lipgloss.NewStyle().Foreground(c).Bold(true),
		fd:   lipgloss.NewStyle().Foreground(s),
		errM: lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Padding(0, 2),
		dim:  lipgloss.NewStyle().Foreground(s),
		brd:  lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		scr:  lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("214")).Padding(0, 1),
		meta: lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Padding(0, 2),
		logo: lipgloss.NewStyle().Foreground(lipgloss.Color("43")),
		codeBg: lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252")).Padding(0, 1),
		codeL: lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(s).Padding(0, 1),
		badge: lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("43")).Padding(0, 1),
		link:  lipgloss.NewStyle().Foreground(lipgloss.Color("43")).Underline(true),
	}
}

type chunk struct {
	content   string
	reasoning string
	done      bool
	err       error
}

type tick struct{}

type model struct {
	vp       viewport.Model
	ta       textarea.Model
	sp       spinner.Model
	t        theme
	cfg      *config.Manager
	ctxEng   *context.Engine
	mem      *memory.ProjectMemory
	vector   *memory.VectorMemory
	skills   *skill.Engine
	rtr      *router.Router

	entries    []entry
	messages   []models.Message
	w, h       int
	statusText string
	modelName  string
	provName   string
	streaming  bool
	sb         strings.Builder
	rb         strings.Builder
	streamCh   chan chunk
	cost       float64
	tok        int
	sid        int64
	lastPrompt string
	startTime  time.Time
	glam       *glamour.TermRenderer

	inputHistory []string
	historyIdx   int
	dotTick      int
	atBottom     bool
	stopCh       chan struct{}
	quitConfirm  bool
}

func NewChatModel(cfg *config.Manager) *model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("43"))

	ta := textarea.New()
	ta.Placeholder = "  Ask Delta to build anything..."
	ta.Prompt = "┃ "
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)

	g, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(78),
	)

	m := &model{
		vp: vp, ta: ta, sp: s, t: defTheme(), cfg: cfg,
		statusText:   "Ready",
		modelName:    cfg.GetConfig().DefaultModel,
		provName:     cfg.GetConfig().DefaultProvider,
		w: 80, h: 24, glam: g, atBottom: true,
		inputHistory: []string{},
		historyIdx:   -1,
	}

	m.vp.Width = 78
	m.vp.Height = 14
	m.ta.SetWidth(74)
	m.ta.SetHeight(3)

	m.init()
	m.splash()
	m.render()
	return m
}

func (m *model) init() {
	m.ctxEng, _ = context.NewEngine()
	if mem, err := memory.NewProjectMemory(); err == nil {
		m.mem = mem
		m.sid, _ = mem.CreateSession("tui-session")
	}
	if v, err := memory.NewVectorMemory(); err == nil {
		m.vector = v
	}
	if s, err := skill.NewEngine(); err == nil {
		m.skills = s
	}
	conf := m.cfg.GetConfig()
	m.rtr = router.NewRouter(conf.DefaultProvider, conf.DefaultModel)
	m.loadSession()
	if len(m.messages) > 0 {
		for _, msg := range m.messages {
			if msg.Role == models.RoleUser {
				m.entries = append(m.entries, entry{role: "user", content: msg.Content})
			} else if msg.Role == models.RoleAssistant {
				m.entries = append(m.entries, entry{role: "assistant", content: msg.Content})
			}
		}
	}
}

type sessionData struct {
	Messages []models.Message `json:"messages"`
	Cost     float64         `json:"cost"`
	Tokens   int             `json:"tokens"`
}

func sessionPath() string {
	conf := filepath.Join(os.Getenv("HOME"), ".delta", "sessions")
	os.MkdirAll(conf, 0755)
	return filepath.Join(conf, "last.json")
}

func (m *model) saveSession() {
	data := sessionData{
		Messages: m.messages,
		Cost:     m.cost,
		Tokens:   m.tok,
	}
	if b, err := json.Marshal(data); err == nil {
		os.WriteFile(sessionPath(), b, 0644)
	}
}

func (m *model) loadSession() {
	if b, err := os.ReadFile(sessionPath()); err == nil {
		var data sessionData
		if json.Unmarshal(b, &data) == nil && len(data.Messages) > 0 {
			m.messages = data.Messages
			m.cost = data.Cost
			m.tok = data.Tokens
		}
	}
}

func (m *model) splash() {
	for _, s := range []string{
		"      ██████╗ ███████╗██╗  ████████╗ █████╗",
		"      ██╔══██╗██╔════╝██║  ╚══██╔══╝██╔══██╗",
		"      ██║  ██║█████╗  ██║     ██║   ███████║",
		"      ██║  ██║██╔══╝  ██║     ██║   ██╔══██║",
		"      ██████╔╝███████╗███████╗██║   ██║  ██║",
		"      ╚═════╝ ╚══════╝╚══════╝╚═╝   ╚═╝  ╚═╝",
	} {
		m.entries = append(m.entries, entry{role: "system", content: m.t.logo.Render(s)})
	}
	m.addSys("v0.2.1  |  Model: " + m.modelName + "  |  Provider: " + m.provName)
	m.addSys("Type a prompt and press Enter — or type /help for commands.")
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.sp.Tick, textarea.Blink)
}

func InitProgram(cfg *config.Manager) *tea.Program {
	return tea.NewProgram(NewChatModel(cfg), tea.WithMouseCellMotion())
}
