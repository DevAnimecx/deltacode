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

type chatMessage struct {
	role     string
	content  string
	duration time.Duration
	tokens   int
	cost     float64
}

type st struct {
	header     lipgloss.Style
	subh      lipgloss.Style
	status     lipgloss.Style
	userMsg    lipgloss.Style
	userPr     lipgloss.Style
	asstMsg    lipgloss.Style
	asstPr     lipgloss.Style
	sysMsg     lipgloss.Style
	sep        lipgloss.Style
	fk         lipgloss.Style
	fd         lipgloss.Style
	errMsg     lipgloss.Style
	dim        lipgloss.Style
	brd        lipgloss.Style
	scroll     lipgloss.Style
	meta       lipgloss.Style
	logo       lipgloss.Style
}

func newStyle() st {
	c := lipgloss.Color("43")
	t := lipgloss.Color("37")
	sub := lipgloss.Color("244")
	g := lipgloss.Color("240")
	return st{
		header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Padding(0, 1),
		subh:  lipgloss.NewStyle().Foreground(sub).Padding(0, 1),
		status: lipgloss.NewStyle().Foreground(sub),
		userMsg: lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Padding(0, 2),
		userPr: lipgloss.NewStyle().Bold(true).Foreground(c).SetString("┃ You "),
		asstMsg: lipgloss.NewStyle().Padding(0, 2),
		asstPr: lipgloss.NewStyle().Bold(true).Foreground(t).SetString("Δ "),
		sysMsg: lipgloss.NewStyle().Foreground(sub).Italic(true).Padding(0, 2),
		sep: lipgloss.NewStyle().Foreground(g),
		fk: lipgloss.NewStyle().Foreground(c).Bold(true),
		fd: lipgloss.NewStyle().Foreground(sub),
		errMsg: lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Padding(0, 2),
		dim: lipgloss.NewStyle().Foreground(sub),
		brd: lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		scroll: lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("214")).Padding(0, 1),
		meta: lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Padding(0, 2),
		logo: lipgloss.NewStyle().Foreground(lipgloss.Color("43")),
	}
}

type streamMsg struct {
	content string
	done    bool
	err     error
}

type tickMsg struct{}

type chatModel struct {
	ready       bool
	vp          viewport.Model
	ta          textarea.Model
	sp          spinner.Model
	st          st
	cfg         *config.Manager
	ctxEng      *context.Engine
	mem         *memory.ProjectMemory
	vector      *memory.VectorMemory
	skills      *skill.Engine
	router      *router.Router

	msgs        []chatMessage
	w           int
	h           int
	statusText  string
	modelName   string
	provName    string
	streaming   bool
	sb          strings.Builder
	streamCh    chan streamMsg
	cost        float64
	tok         int
	sid         int64
	lastPrompt  string
	startTime   time.Time
	glam        *glamour.TermRenderer

	dotTick     int
	scrollTop   int
	atBottom    bool
}

func NewChatModel(cfg *config.Manager) chatModel {
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

	m := chatModel{
		vp:        vp,
		ta:        ta,
		sp:        s,
		st:        newStyle(),
		cfg:       cfg,
		statusText: "Ready",
		modelName: cfg.GetConfig().DefaultModel,
		provName:  cfg.GetConfig().DefaultProvider,
		ready:     true,
		w:         80,
		h:         24,
		glam:      g,
		atBottom:  true,
	}

	m.vp.Width = 78
	m.vp.Height = 15
	m.ta.SetWidth(74)
	m.ta.SetHeight(3)
	m.renderMsgs()

	m.initSubsystems()
	m.msgs = append(m.msgs, chatMessage{role: "system", content: ""})
	m.msgs = append(m.msgs, chatMessage{role: "system", content: m.st.logo.Render(
		"      ██████╗ ███████╗██╗  ████████╗ █████╗")})
	m.msgs = append(m.msgs, chatMessage{role: "system", content: m.st.logo.Render(
		"      ██╔══██╗██╔════╝██║  ╚══██╔══╝██╔══██╗")})
	m.msgs = append(m.msgs, chatMessage{role: "system", content: m.st.logo.Render(
		"      ██║  ██║█████╗  ██║     ██║   ███████║")})
	m.msgs = append(m.msgs, chatMessage{role: "system", content: m.st.logo.Render(
		"      ██║  ██║██╔══╝  ██║     ██║   ██╔══██║")})
	m.msgs = append(m.msgs, chatMessage{role: "system", content: m.st.logo.Render(
		"      ██████╔╝███████╗███████╗██║   ██║  ██║")})
	m.msgs = append(m.msgs, chatMessage{role: "system", content: m.st.logo.Render(
		"      ╚═════╝ ╚══════╝╚══════╝╚═╝   ╚═╝  ╚═╝")})
	m.msgs = append(m.msgs, chatMessage{role: "system", content: ""})
	m.appendSys(fmt.Sprintf("Model: %s  •  Provider: %s", m.modelName, m.provName))
	m.appendSys("Type your prompt below and press Enter to begin.")

	return m
}

func (m *chatModel) initSubsystems() {
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
	m.router = router.NewRouter(conf.DefaultProvider, conf.DefaultModel)
}

func (m chatModel) Init() tea.Cmd {
	return tea.Batch(m.sp.Tick, textarea.Blink)
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleResize(msg)
		return &m, nil
	case tea.KeyMsg:
		if m.streaming {
			return &m, nil
		}
		return m.handleKey(msg)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return &m, cmd
	case streamMsg:
		return m.handleStream(msg)
	case tickMsg:
		m.dotTick = (m.dotTick + 1) % 4
		m.renderMsgs()
		if m.streaming {
			return &m, m.tick()
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return &m, cmd
}

func (m *chatModel) handleResize(msg tea.WindowSizeMsg) {
	m.w = msg.Width
	m.h = msg.Height
	vpH := m.h - 9
	if vpH < 5 {
		vpH = 5
	}
	m.vp.Width = msg.Width - 2
	m.vp.Height = vpH
	m.ta.SetWidth(msg.Width - 6)
	m.ta.SetHeight(3)
	if !m.ready {
		m.ready = true
	}
	m.renderMsgs()
}

func (m *chatModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		t := strings.TrimSpace(m.ta.Value())
		if t == "" {
			return m, nil
		}
		if strings.HasPrefix(t, "/") {
			return m, m.handleSlash(t)
		}
		m.ta.Reset()
		m.ta.Blur()
		return m, m.submitPrompt(t)
	case "tab":
		if m.ta.Focused() {
			m.ta.Blur()
		} else {
			m.ta.Focus()
		}
	case "ctrl+l":
		m.msgs = nil
		m.renderMsgs()
	case "ctrl+w":
		m.ta.Reset()
	case "up", "down", "pgup", "pgdown":
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	if !m.ta.Focused() {
		m.ta.Focus()
	}
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	if m.vp.ScrollPercent() >= 0.99 {
		m.atBottom = true
	} else {
		m.atBottom = false
	}

	return m, cmd
}

func (m *chatModel) handleSlash(cmd string) tea.Cmd {
	parts := strings.Fields(strings.ToLower(cmd))
	switch parts[0] {
	case "/clear":
		m.msgs = nil
		m.renderMsgs()
		m.ta.Reset()
		return nil
	case "/help":
		m.appendSys("Available: /clear  /model <name>  /cost  /help")
		return nil
	case "/cost":
		m.appendSys(fmt.Sprintf("Session cost: $%.4f  |  Tokens: %d", m.cost, m.tok))
		return nil
	case "/model":
		if len(parts) > 1 {
			conf := m.cfg.GetConfig()
			conf.DefaultModel = parts[1]
			m.modelName = parts[1]
			m.appendSys(fmt.Sprintf("Switched to model: %s", parts[1]))
		} else {
			m.appendSys(fmt.Sprintf("Current model: %s", m.modelName))
		}
		return nil
	}
	m.appendSys(fmt.Sprintf("Unknown command: %s  (/help for list)", cmd))
	return nil
}

func (m *chatModel) submitPrompt(prompt string) tea.Cmd {
	m.lastPrompt = prompt
	m.appendUser(prompt)
	m.statusText = "Thinking"
	m.startTime = time.Now()
	m.renderMsgs()
	m.vp.GotoBottom()
	m.atBottom = true

	conf := m.cfg.GetConfig()
	pName, modelName := m.router.Route(prompt, conf.Providers)
	m.modelName = modelName
	m.provName = pName
	m.streaming = true
	m.sb.Reset()

	provCfg, err := m.cfg.GetProvider(pName)
	if err != nil {
		provCfg, err = m.cfg.GetProvider(conf.DefaultProvider)
		if err != nil {
			m.streaming = false
			m.statusText = "Error"
			m.appendErr("No provider configured. Use `delta provider add`")
			m.vp.GotoBottom()
			return nil
		}
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		m.streaming = false
		m.statusText = "Error"
		m.appendErr(err.Error())
		m.vp.GotoBottom()
		return nil
	}

	ctxPrompt := prompt
	if m.ctxEng != nil {
		ctxPrompt = m.ctxEng.BuildPrompt(prompt)
	}

	msgs := []models.Message{
		{Role: models.RoleSystem, Content: "You are Delta Code, an expert software engineer. Write production-ready code. Be concise and precise. Use markdown formatting with code blocks."},
	}
	if m.vector != nil {
		results := m.vector.Search(prompt, 3)
		if len(results) > 0 {
			var sb strings.Builder
			sb.WriteString("Relevant context from previous sessions:\n")
			for i, r := range results {
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

	ch := make(chan streamMsg, 50)
	m.streamCh = ch

	go func() {
		defer close(ch)
		stream, err := p.ChatStream(req)
		if err != nil {
			ch <- streamMsg{err: err}
			return
		}
		for chunk := range stream {
			if chunk.Done {
				break
			}
			ch <- streamMsg{content: chunk.Content}
		}
		ch <- streamMsg{done: true}
	}()

	return tea.Batch(m.readStream(), m.tick())
}

func (m *chatModel) readStream() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.streamCh
		if !ok {
			return streamMsg{done: true}
		}
		return msg
	}
}

func (m *chatModel) tick() tea.Cmd {
	return tea.Tick(time.Millisecond*400, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *chatModel) handleStream(msg streamMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.streaming = false
		m.statusText = "Error"
		m.appendErr(msg.err.Error())
		m.vp.GotoBottom()
		m.streamCh = nil
		return m, nil
	}
	if msg.done {
		full := m.sb.String()
		m.streaming = false
		m.statusText = "Ready"
		if full != "" {
			m.appendAssistant(full)
		}
		m.vp.GotoBottom()
		m.storeMemory(m.lastPrompt, full)
		m.streamCh = nil
		m.ta.Focus()
		return m, nil
	}
	m.sb.WriteString(msg.content)
	m.appendStream(msg.content)
	if m.atBottom {
		m.vp.GotoBottom()
	}
	return m, m.readStream()
}

func (m *chatModel) storeMemory(prompt, response string) {
	if m.mem != nil {
		m.mem.AddEntry(m.sid, "user", prompt, nil)
		m.mem.AddEntry(m.sid, "assistant", response, map[string]any{"source": "tui"})
	}
	if m.vector != nil {
		m.vector.Store(fmt.Sprintf("Q: %s\nA: %s", prompt, response), extractTags(prompt))
	}
	if m.skills != nil && len(response) > 50 && strings.Contains(strings.ToLower(prompt), "create") {
		m.skills.Save(generateSkillName(prompt), prompt, response, extractTags(prompt))
	}
}

func extractTags(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var tags []string
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) > 3 && !seen[w] {
			tags = append(tags, w)
			seen[w] = true
		}
		if len(tags) >= 5 {
			break
		}
	}
	return tags
}

func generateSkillName(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) > 6 {
		return strings.Join(words[:6], "-")
	}
	return strings.Join(words, "-")
}

func (m chatModel) View() string {
	if !m.ready {
		return m.starting()
	}
	m.renderMsgs()
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHdr(m.w),
		m.renderSep(m.w),
		m.vp.View(),
		m.renderInput(),
		m.renderStatus(m.w),
		m.renderFooter(m.w),
	)
}

func (m chatModel) starting() string {
	return fmt.Sprintf("%s\n%s\n  %s initializing...",
		m.st.header.Render(" Δ Delta Code"),
		m.st.sep.Render(strings.Repeat("─", 40)),
		m.sp.View(),
	)
}

func (m chatModel) renderHdr(w int) string {
	if w <= 0 {
		w = 80
	}
	logo := m.st.header.Render(" Δ ")
	info := m.st.subh.Render(fmt.Sprintf(" %s • %s ", m.modelName, m.provName))
	ts := m.st.status.Render(time.Now().Format("15:04:05"))
	sp := w - lipgloss.Width(logo) - lipgloss.Width(info) - lipgloss.Width(ts) - 4
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, logo, strings.Repeat(" ", sp), info, ts)
}

func (m chatModel) renderSep(w int) string {
	if w <= 0 {
		w = 80
	}
	return m.st.sep.Render("─" + strings.Repeat("─", max(w-2, 0)) + "─")
}

func (m chatModel) renderStatus(w int) string {
	prefix := ""
	if m.streaming {
		prefix = m.sp.View() + " "
	}
	left := m.st.status.Render(fmt.Sprintf(" %s%s", prefix, m.statusText))
	right := m.st.dim.Render(fmt.Sprintf("msgs:%d  $%.4f", len(m.msgs), m.cost))
	sp := w - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", sp), right)
}

func (m chatModel) renderFooter(w int) string {
	bindings := []struct{ k, d string }{
		{"Enter", "send"}, {"Tab", "focus"}, {"↑↓", "scroll"},
		{"/help", "cmds"}, {"Ctrl+L", "clear"}, {"Ctrl+C", "quit"},
	}
	var parts []string
	for _, b := range bindings {
		parts = append(parts, m.st.fk.Render(b.k)+" "+m.st.fd.Render(b.d))
	}
	line := strings.Join(parts, "  ")
	return fmt.Sprintf("%s\n %s", m.st.brd.Render(strings.Repeat("─", max(0, w-2))), line)
}

func (m chatModel) renderInput() string {
	return m.ta.View()
}

func (m *chatModel) renderMsgs() {
	var out []string
	s := m.st

	needsScroll := m.vp.ScrollPercent() > 0 && !m.atBottom
	if needsScroll {
		remain := m.vp.TotalLineCount() - m.vp.YOffset
		out = append(out, s.scroll.Render(fmt.Sprintf(" ↑ %d more lines", remain)))
		out = append(out, "")
	}

	for _, msg := range m.msgs {
		switch msg.role {
		case "user":
			out = append(out, s.userPr.String())
			for _, line := range strings.Split(msg.content, "\n") {
				out = append(out, s.userMsg.Render("  "+line))
			}
		case "assistant":
			out = append(out, s.asstPr.String())
			rendered := m.renderMarkdown(msg.content)
			out = append(out, rendered)
			if msg.duration > 0 || msg.tokens > 0 {
				meta := ""
				if msg.duration > 0 {
					meta += fmt.Sprintf("%.1fs", msg.duration.Seconds())
				}
				if msg.tokens > 0 {
					if meta != "" {
						meta += " · "
					}
					meta += fmt.Sprintf("%d tok", msg.tokens)
				}
				if msg.cost > 0 {
					if meta != "" {
						meta += " · "
					}
					meta += fmt.Sprintf("$%.4f", msg.cost)
				}
				out = append(out, s.meta.Render("  "+meta))
			}
		case "streaming":
			out = append(out, s.asstPr.String())
			rendered := m.renderStreamContent(msg.content)
			out = append(out, rendered)
		case "system":
			out = append(out, s.sysMsg.Render("   "+msg.content))
		case "error":
			out = append(out, s.errMsg.Render("  ✗ "+msg.content))
		}
		out = append(out, "")
	}

	if m.streaming {
		dots := []string{"  ", " · ", " ·· ", " ···"}
		d := dots[m.dotTick%len(dots)]
		out = append(out, s.dim.Render("  thinking"+d))
	}

	m.vp.SetContent(strings.Join(out, "\n"))
}

func (m chatModel) renderMarkdown(content string) string {
	if m.glam == nil {
		return sAsstFallback(content)
	}
	out, err := m.glam.Render(content)
	if err != nil || out == "" {
		return sAsstFallback(content)
	}
	return out
}

func sAsstFallback(content string) string {
	s := lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("252"))
	var result []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "```") {
			continue
		}
		result = append(result, s.Render("  "+line))
	}
	return strings.Join(result, "\n")
}

func (m chatModel) renderStreamContent(content string) string {
	s := m.st
	var result []string
	lines := strings.Split(content, "\n")
	inCode := false
	var cb []string
	cl := ""

	flush := func() {
		if len(cb) == 0 {
			return
		}
		l := " code "
		if cl != "" {
			l = " " + cl + " "
		}
		result = append(result, s.dim.Render("  │ "+l))
		for _, ln := range cb {
			result = append(result, s.dim.Render("  │ "+ln))
		}
		cb = nil
		cl = ""
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			flush()
			inCode = !inCode
			if inCode {
				cl = strings.TrimSpace(strings.TrimPrefix(line, "```"))
			}
			continue
		}
		if inCode {
			cb = append(cb, line)
			continue
		}
		flush()
		if line == "" {
			result = append(result, "")
		} else {
			result = append(result, s.dim.Render("  "+line))
		}
	}
	flush()

	return strings.Join(result, "\n")
}

func (m *chatModel) appendUser(c string) {
	m.msgs = append(m.msgs, chatMessage{role: "user", content: c})
	m.renderMsgs()
}

func (m *chatModel) appendAssistant(c string) {
	dur := time.Since(m.startTime)
	m.msgs = append(m.msgs, chatMessage{role: "assistant", content: c, duration: dur, tokens: m.tok, cost: m.cost})
	m.renderMsgs()
}

func (m *chatModel) appendStream(c string) {
	if len(m.msgs) > 0 && m.msgs[len(m.msgs)-1].role == "streaming" {
		m.msgs[len(m.msgs)-1].content += c
	} else {
		m.msgs = append(m.msgs, chatMessage{role: "streaming", content: c})
	}
	m.renderMsgs()
}

func (m *chatModel) appendSys(c string) {
	m.msgs = append(m.msgs, chatMessage{role: "system", content: c})
	m.renderMsgs()
}

func (m *chatModel) appendErr(c string) {
	m.msgs = append(m.msgs, chatMessage{role: "error", content: c})
	m.renderMsgs()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
