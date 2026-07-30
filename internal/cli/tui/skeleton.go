package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/context"
	"github.com/DevAnimecx/deltacode/internal/memory"
	"github.com/DevAnimecx/deltacode/internal/provider"
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
	role     string
	content  string
	duration time.Duration
	tokens   int
	cost     float64
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
}

func defTheme() theme {
	c := lipgloss.Color("43")
	t := lipgloss.Color("37")
	s := lipgloss.Color("244")
	return theme{
		head: lipgloss.NewStyle().Bold(true).Foreground(c).Padding(0, 1),
		subh:   lipgloss.NewStyle().Foreground(s).Padding(0, 1),
		stat:   lipgloss.NewStyle().Foreground(s),
		uM: lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Padding(0, 2),
		uP: lipgloss.NewStyle().Bold(true).Foreground(c).SetString("┃ You "),
		aM: lipgloss.NewStyle().Padding(0, 2),
		aP: lipgloss.NewStyle().Bold(true).Foreground(t).SetString("Δ "),
		sysM:   lipgloss.NewStyle().Foreground(s).Italic(true).Padding(0, 2),
		sep:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		fk: lipgloss.NewStyle().Foreground(c).Bold(true),
		fd: lipgloss.NewStyle().Foreground(s),
		errM:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Padding(0, 2),
		dim:    lipgloss.NewStyle().Foreground(s),
		brd:    lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		scr:    lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("214")).Padding(0, 1),
		meta:   lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Padding(0, 2),
		logo:   lipgloss.NewStyle().Foreground(lipgloss.Color("43")),
		codeBg: lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252")).Padding(0, 1),
		codeL: lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(s).Padding(0, 1),
	}
}

type chunk struct {
	content string
	done    bool
	err     error
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

	entries     []entry
	w, h        int
	statusText  string
	modelName   string
	provName    string
	streaming   bool
	sb          strings.Builder
	streamCh    chan chunk
	cost        float64
	tok         int
	sid         int64
	lastPrompt  string
	startTime   time.Time
	glam        *glamour.TermRenderer

	dotTick  int
	atBottom bool
	stopCh   chan struct{}
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
		statusText: "Ready",
		modelName:  cfg.GetConfig().DefaultModel,
		provName:   cfg.GetConfig().DefaultProvider,
		w: 80, h: 24, glam: g, atBottom: true,
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
	m.addSys("Model: " + m.modelName + "  |  Provider: " + m.provName)
	m.addSys("Type a prompt and press Enter - or type /help for commands.")
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.sp.Tick, textarea.Blink)
}

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

	case tea.KeyMsg:
		if v.String() == "ctrl+c" {
			if m.streaming {
				m.cancelStream()
				m.addSys("Cancelled.")
				return m, nil
			}
			return m, tea.Quit
		}
		if m.streaming {
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

func (m *model) onKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		t := strings.TrimSpace(m.ta.Value())
		if t == "" {
			return nil
		}
		m.ta.Reset()
		if strings.HasPrefix(t, "/") {
			return m.slash(t)
		}
		m.ta.Blur()
		return m.submit(t)

	case "tab":
		if m.ta.Focused() {
			m.ta.Blur()
		} else {
			m.ta.Focus()
		}

	case "up", "down", "pgup", "pgdown":
		m.vp, _ = m.vp.Update(msg)

	case "ctrl+l":
		m.entries = nil
		m.render()
	}

	if !m.ta.Focused() {
		m.ta.Focus()
	}
	m.ta, _ = m.ta.Update(msg)
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
		m.addSys("/clear    Clear all messages")
		m.addSys("/cost     Show session cost & tokens")
		m.addSys("/model N  Switch model")
		m.addSys("/help     Show this help")
	case "/cost":
		m.addSys(fmt.Sprintf("Cost: $%.4f  |  Tokens: %d", m.cost, m.tok))
	case "/model":
		if len(p) > 1 {
			m.modelName = p[1]
			m.addSys("Switched to: " + p[1])
		} else {
			m.addSys("Current: " + m.modelName)
		}
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
	m.stopCh = make(chan struct{})
	m.sb.Reset()
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
		if raw.Done {
			break
		}
		select {
		case ch <- chunk{content: raw.Content}:
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
	m.sb.WriteString(c.content)
	m.addStream(c.content)
	if m.atBottom {
		m.vp.GotoBottom()
	}
	return m.nextChunk()
}

func (m *model) finishStream() {
	full := m.sb.String()
	m.streaming = false
	m.statusText = "Ready"
	if full != "" {
		m.addAsst(full)
		m.storeMemory(m.lastPrompt, full)
	}
	m.streamCh = nil
	m.ta.Focus()
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
	l := m.t.stat.Render(fmt.Sprintf(" %s%s", p, m.statusText))
	r := m.t.dim.Render(fmt.Sprintf("msgs:%d  $%.4f  %dtok", len(m.entries), m.cost, m.tok))
	sp := m.w - lipgloss.Width(l) - lipgloss.Width(r) - 2
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, l, strings.Repeat(" ", sp), r)
}

func (m *model) footer() string {
	b := []struct{ k, d string }{
		{"Enter", "send"}, {"Tab", "focus"}, {"↑↓", "scroll"},
		{"/help", "cmds"}, {"Ctrl+L", "clear"}, {"Ctrl+C", "quit"},
	}
	var p []string
	for _, x := range b {
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
			out = append(out, m.renderMD(e.content))
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
		dots := []string{"  ", " · ", " ·· ", " ···"}
		out = append(out, m.t.dim.Render("  thinking" + dots[m.dotTick%len(dots)]))
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
	return r
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
	m.entries = append(m.entries, entry{role: "user", content: c})
}
func (m *model) addAsst(c string) {
	m.entries = append(m.entries, entry{role: "assistant", content: c, duration: time.Since(m.startTime), tokens: m.tok, cost: m.cost})
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
