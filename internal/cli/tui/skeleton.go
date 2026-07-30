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
	"github.com/charmbracelet/lipgloss"
)

type chatMessage struct {
	role    string
	content string
}

type style struct {
	header     lipgloss.Style
	subheader  lipgloss.Style
	status     lipgloss.Style
	userMsg    lipgloss.Style
	userPrefix lipgloss.Style
	asstMsg    lipgloss.Style
	asstPrefix lipgloss.Style
	codeBg     lipgloss.Style
	codeLabel  lipgloss.Style
	system     lipgloss.Style
	separator  lipgloss.Style
	footKey    lipgloss.Style
	footDesc   lipgloss.Style
	errMsg     lipgloss.Style
	dim        lipgloss.Style
	border     lipgloss.Style
	meter      lipgloss.Style
}

func defaultStyle() style {
	cyan := lipgloss.Color("43")
	teal := lipgloss.Color("37")
	sub := lipgloss.Color("244")
	gray := lipgloss.Color("240")
	dk := lipgloss.Color("236")

	return style{
		header:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Padding(0, 1),
		subheader:  lipgloss.NewStyle().Foreground(sub).Padding(0, 1),
		status:     lipgloss.NewStyle().Foreground(sub),
		userMsg:    lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Padding(0, 2),
		userPrefix: lipgloss.NewStyle().Bold(true).Foreground(cyan).SetString("┃ You "),
		asstMsg:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 2),
		asstPrefix: lipgloss.NewStyle().Bold(true).Foreground(teal).SetString("Δ "),
		codeBg:     lipgloss.NewStyle().Background(dk).Foreground(lipgloss.Color("252")).Padding(0, 1),
		codeLabel:  lipgloss.NewStyle().Background(dk).Foreground(sub).Padding(0, 1),
		system:     lipgloss.NewStyle().Foreground(sub).Italic(true).Padding(0, 2),
		separator:  lipgloss.NewStyle().Foreground(gray),
		footKey:    lipgloss.NewStyle().Foreground(cyan).Bold(true),
		footDesc:   lipgloss.NewStyle().Foreground(sub),
		errMsg:     lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Padding(0, 2),
		dim:        lipgloss.NewStyle().Foreground(sub),
		border:     lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		meter:      lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Padding(0, 1),
	}
}

type streamMsg struct {
	content string
	done    bool
	err     error
}

type chatModel struct {
	ready        bool
	viewport     viewport.Model
	textarea     textarea.Model
	spinner      spinner.Model
	st           style
	cfg          *config.Manager
	ctxEng       *context.Engine
	mem          *memory.ProjectMemory
	vector       *memory.VectorMemory
	skills       *skill.Engine
	router       *router.Router

	messages     []chatMessage
	width        int
	height       int
	statusText   string
	modelName    string
	provName     string
	isStreaming  bool
	streamBuf    strings.Builder
	streamCh     chan streamMsg
	cost         float64
	tokens       int
	sessionID    int64
	lastPrompt   string
}

func NewChatModel(cfg *config.Manager) chatModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("43"))

	ta := textarea.New()
	ta.Placeholder = "Ask Delta to build anything...  (Shift+Enter for newline, Enter to send)"
	ta.Prompt = "┃ "
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)

	st := defaultStyle()

	m := chatModel{
		viewport:   vp,
		textarea:   ta,
		spinner:    s,
		st:         st,
		cfg:        cfg,
		statusText: "Ready",
		modelName:  cfg.GetConfig().DefaultModel,
		provName:   cfg.GetConfig().DefaultProvider,
		ready:      true,
		width:      80,
		height:     24,
	}

	m.viewport.Width = 78
	m.viewport.Height = 14
	m.textarea.SetWidth(74)
	m.textarea.SetHeight(3)

	m.initSubsystems()
	m.appendSystem("Δ Delta Code — The Self-Evolving BYOK Coding Agent")
	m.appendSystem(fmt.Sprintf("Model: %s  |  Provider: %s", m.modelName, m.provName))
	m.appendSystem("Type your prompt below and press Enter to begin.")

	return m
}

func (m *chatModel) initSubsystems() {
	m.ctxEng, _ = context.NewEngine()
	if mem, err := memory.NewProjectMemory(); err == nil {
		m.mem = mem
		m.sessionID, _ = mem.CreateSession("tui-session")
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
	return tea.Batch(m.spinner.Tick, textarea.Blink)
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleResize(msg)
		return &m, nil

	case tea.KeyMsg:
		if m.isStreaming {
			return &m, nil
		}
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return &m, cmd

	case streamMsg:
		return m.handleStreamMsg(msg)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return &m, cmd
}

func (m *chatModel) handleResize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	inputH := 4
	statusH := 2
	footerH := 1
	margin := inputH + statusH + footerH + 4
	vpHeight := m.height - margin
	if vpHeight < 5 {
		vpHeight = 5
	}

	m.viewport.Width = msg.Width - 2
	m.viewport.Height = vpHeight

	m.textarea.SetWidth(msg.Width - 6)
	m.textarea.SetHeight(3)

	if !m.ready {
		m.ready = true
	}
	m.refreshViewport()
}

func (m *chatModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		text := strings.TrimSpace(m.textarea.Value())
		if text == "" {
			return m, nil
		}
		m.textarea.Reset()
		m.textarea.Blur()
		return m, m.submitPrompt(text)

	case "tab":
		if m.textarea.Focused() {
			m.textarea.Blur()
		} else {
			m.textarea.Focus()
		}

	case "ctrl+l":
		m.messages = nil
		m.refreshViewport()

	case "ctrl+w":
		m.textarea.Reset()
	}

	if !m.textarea.Focused() {
		m.textarea.Focus()
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m *chatModel) submitPrompt(prompt string) tea.Cmd {
	m.lastPrompt = prompt
	m.appendUser(prompt)
	m.statusText = "Thinking..."
	m.refreshViewport()
	m.viewport.GotoBottom()

	conf := m.cfg.GetConfig()
	pName, modelName := m.router.Route(prompt, conf.Providers)
	m.modelName = modelName
	m.provName = pName
	m.isStreaming = true
	m.streamBuf.Reset()
	m.cost = 0
	m.tokens = 0

	provCfg, err := m.cfg.GetProvider(pName)
	if err != nil {
		provCfg, err = m.cfg.GetProvider(conf.DefaultProvider)
		if err != nil {
			m.isStreaming = false
			m.statusText = "Error"
			m.appendError("no provider configured. Use `delta provider add`")
			m.viewport.GotoBottom()
			return nil
		}
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		m.isStreaming = false
		m.statusText = "Error"
		m.appendError(err.Error())
		m.viewport.GotoBottom()
		return nil
	}

	ctxPrompt := prompt
	if m.ctxEng != nil {
		ctxPrompt = m.ctxEng.BuildPrompt(prompt)
	}

	msgs := []models.Message{
		{Role: models.RoleSystem, Content: "You are Delta Code, an expert software engineer. Write production-ready code. Be concise and precise."},
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

	return m.readStreamCmd()
}

func (m *chatModel) readStreamCmd() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.streamCh
		if !ok {
			return streamMsg{done: true}
		}
		return msg
	}
}

func (m *chatModel) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.isStreaming = false
		m.statusText = "Error"
		m.appendError(msg.err.Error())
		m.viewport.GotoBottom()
		m.streamCh = nil
		return m, nil
	}

	if msg.done {
		full := m.streamBuf.String()
		m.isStreaming = false
		m.statusText = "Ready"
		if full != "" {
			m.appendAssistant(full)
		}
		m.viewport.GotoBottom()
		m.storeMemory(m.lastPrompt, full)
		m.streamCh = nil
		m.textarea.Focus()
		return m, nil
	}

	m.streamBuf.WriteString(msg.content)
	m.appendStreaming(msg.content)
	m.viewport.GotoBottom()

	return m, m.readStreamCmd()
}

func (m *chatModel) storeMemory(prompt, response string) {
	if m.mem != nil {
		m.mem.AddEntry(m.sessionID, "user", prompt, nil)
		m.mem.AddEntry(m.sessionID, "assistant", response, map[string]any{"source": "tui"})
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
		return m.renderStarting()
	}

	header := m.renderHeader(m.width)
	sep := m.renderSep(m.width)
	m.refreshViewport()
	input := m.renderInput()
	status := m.renderStatus(m.width)
	footer := m.renderFooter(m.width)

	lines := []string{header, sep, m.viewport.View(), input, status, footer}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m chatModel) renderStarting() string {
	return fmt.Sprintf("%s\n%s\n  %s initializing...\n%s",
		m.st.header.Render(" Δ Delta Code"),
		m.st.separator.Render(strings.Repeat("─", 40)),
		m.spinner.View(),
		m.st.dim.Render("Loading subsystems..."),
	)
}

func (m chatModel) renderHeader(w int) string {
	if w <= 0 {
		w = 80
	}
	logo := m.st.header.Render(" Δ Delta Code ")
	info := m.st.subheader.Render(fmt.Sprintf(" %s • %s ", m.modelName, m.provName))
	timeStr := m.st.status.Render(time.Now().Format("15:04:05"))
	sp := w - lipgloss.Width(logo) - lipgloss.Width(info) - lipgloss.Width(timeStr) - 4
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, logo, strings.Repeat(" ", sp), info, timeStr)
}

func (m chatModel) renderSep(w int) string {
	if w <= 0 {
		w = 80
	}
	return m.st.separator.Render("─" + strings.Repeat("─", max(w-2, 0)) + "─")
}

func (m chatModel) renderStatus(w int) string {
	s := m.st
	prefix := ""
	if m.isStreaming {
		prefix = m.spinner.View() + " "
	}
	left := s.status.Render(fmt.Sprintf(" %s%s", prefix, m.statusText))
	right := s.dim.Render(fmt.Sprintf("msgs: %d", len(m.messages)))
	sp := w - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if sp < 1 {
		sp = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", sp), right)
}

func (m chatModel) renderFooter(w int) string {
	bindings := []struct{ key, desc string }{
		{"Enter", "send"}, {"Tab", "focus"}, {"↑↓", "scroll"},
		{"Ctrl+L", "clear"}, {"Ctrl+C", "quit"},
	}
	var parts []string
	for _, b := range bindings {
		parts = append(parts, m.st.footKey.Render(b.key)+" "+m.st.footDesc.Render(b.desc))
	}
	line := strings.Join(parts, "   ")
	return fmt.Sprintf("%s\n %s", m.st.border.Render(strings.Repeat("─", max(0, w-2))), line)
}

func (m chatModel) renderInput() string {
	return m.textarea.View()
}

func (m *chatModel) refreshViewport() {
	var out []string
	s := m.st

	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			out = append(out, s.userPrefix.String())
			for _, line := range strings.Split(msg.content, "\n") {
				out = append(out, s.userMsg.Render("  "+line))
			}
		case "assistant":
			out = append(out, s.asstPrefix.String())
			out = append(out, m.formatContent(msg.content, false))
		case "streaming":
			out = append(out, s.asstPrefix.String())
			rendered := m.formatContent(msg.content, true)
			out = append(out, rendered)
		case "system":
			out = append(out, s.system.Render(" ◆ "+msg.content))
		case "error":
			out = append(out, s.errMsg.Render(" ✗ "+msg.content))
		}
		out = append(out, "")
	}

	m.viewport.SetContent(strings.Join(out, "\n"))
}

func (m chatModel) formatContent(content string, streaming bool) string {
	s := m.st
	var result []string
	lines := strings.Split(content, "\n")
	inCode := false
	var codeBuf []string
	codeLang := ""

	flush := func() {
		if len(codeBuf) == 0 {
			return
		}
		label := " code "
		if codeLang != "" {
			label = " " + codeLang + " "
		}
		result = append(result, s.codeLabel.Render(label))
		for _, cl := range codeBuf {
			result = append(result, s.codeBg.Render("  "+cl))
		}
		codeBuf = nil
		codeLang = ""
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			flush()
			inCode = !inCode
			if inCode {
				codeLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
			}
			continue
		}
		if inCode {
			codeBuf = append(codeBuf, line)
			continue
		}
		flush()
		if line == "" {
			result = append(result, "")
		} else {
			result = append(result, s.asstMsg.Render("  "+line))
		}
	}
	flush()

	if streaming {
		result = append(result, s.dim.Render("  ▌"))
	}

	return strings.Join(result, "\n")
}

func (m *chatModel) appendUser(content string) {
	m.messages = append(m.messages, chatMessage{role: "user", content: content})
	m.refreshViewport()
}

func (m *chatModel) appendAssistant(content string) {
	m.messages = append(m.messages, chatMessage{role: "assistant", content: content})
	m.refreshViewport()
}

func (m *chatModel) appendStreaming(chunk string) {
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].role == "streaming" {
		m.messages[len(m.messages)-1].content += chunk
	} else {
		m.messages = append(m.messages, chatMessage{role: "streaming", content: chunk})
	}
	m.refreshViewport()
}

func (m *chatModel) appendSystem(content string) {
	m.messages = append(m.messages, chatMessage{role: "system", content: content})
	m.refreshViewport()
}

func (m *chatModel) appendError(content string) {
	m.messages = append(m.messages, chatMessage{role: "error", content: content})
	m.refreshViewport()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
