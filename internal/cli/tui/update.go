package tui

import (
	"fmt"
	"path/filepath"
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
		if m.dd.visible() {
			vpH -= 8
		}
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
		return m, m.onKey(v)

	case spinner.TickMsg:
		m.sp, _ = m.sp.Update(msg)
		return m, m.sp.Tick

	case chunk:
		return m, m.onChunk(v)

	case tick:
		m.dotTick = (m.dotTick + 1) % 4
		if m.toast != nil && time.Now().After(m.toast.until) {
			m.toast = nil
		}
		if m.streaming && time.Since(m.lastStreamTime) >= time.Second {
			delta := m.sb.Len() - m.lastStreamLen
			if delta > 0 {
				m.tokSpeed = float64(delta) / time.Since(m.lastStreamTime).Seconds() / 5
			}
			m.lastStreamLen = m.sb.Len()
			m.lastStreamTime = time.Now()
		}
		if m.autoRunning {
			m.pollAutoEvents()
		}
		m.render()
		if m.streaming || m.toast != nil || m.autoRunning {
			return m, m.tick()
		}
		return m, nil
	}

	return m, nil
}

func (m *model) onKey(msg tea.KeyMsg) tea.Cmd {
	if m.dd.visible() {
		return m.onDropdownKey(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		if m.streaming {
			m.cancelStream()
			m.addSys("Cancelled.")
			return nil
		}
		if m.quitConfirm {
			m.saveSession()
			m.saveHistory()
			m.addSys("")
			m.addSys(m.sessionSummary())
			m.addSys("Session saved. Goodbye.")
			return tea.Quit
		}
		m.quitConfirm = true
		m.addSys("Press Ctrl+C again to quit.")
		return nil
	}
	m.quitConfirm = false

	if m.streaming {
		switch msg.String() {
		case "esc":
			m.stopStream()
			m.addSys("Stopped - partial response kept.")
		case "ctrl+k", "ctrl+m", "ctrl+p":
			m.addSys("Wait for the response to finish.")
		}
		return nil
	}

	switch msg.String() {
	case "ctrl+k":
		m.openDropdown(ddCommand)
		return nil
	case "ctrl+m":
		m.openDropdown(ddModel)
		return nil
	case "ctrl+p":
		m.openDropdown(ddProvider)
		return nil
	case "ctrl+n":
		return m.newSession(true)
	case "ctrl+s":
		m.saveSession()
		m.toastNow("Session saved")
		return nil
	case "ctrl+h":
		m.helpShown = !m.helpShown
		return nil
	}

	return m.onInputKey(msg)
}

func (m *model) onInputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		if m.dd.kind == ddSlash && m.dd.open && len(m.dd.filtered) > 0 {
			m.applySlashItem(m.dd.filtered[m.dd.cursor].value)
			return nil
		}
		t := strings.TrimSpace(m.ta.Value())
		if t == "" {
			return nil
		}
		m.inputHistory = append(m.inputHistory, t)
		m.historyIdx = -1
		m.saveHistory()
		m.ta.Reset()
		if strings.HasPrefix(t, "/") {
			m.dd.setOpen(false)
			return m.slash(t)
		}
		m.ta.Blur()
		return m.submit(t)

	case "shift+enter":
		if m.ta.Focused() {
			m.ta.InsertString("\n")
			return nil
		}
		return m.submit(strings.TrimSpace(m.ta.Value()))

	case "up":
		if m.ta.Focused() && len(m.inputHistory) > 0 {
			if m.historyIdx == -1 {
				m.historyIdx = len(m.inputHistory) - 1
			} else if m.historyIdx > 0 {
				m.historyIdx--
			}
			m.ta.SetValue(m.inputHistory[m.historyIdx])
			m.maybeSlashComplete()
			return nil
		}
		if !m.ta.Focused() {
			m.vp.LineUp(1)
			return nil
		}

	case "down":
		if m.ta.Focused() && m.historyIdx >= 0 {
			m.historyIdx++
			if m.historyIdx >= len(m.inputHistory) {
				m.historyIdx = -1
				m.ta.Reset()
				m.maybeSlashComplete()
				return nil
			}
			m.ta.SetValue(m.inputHistory[m.historyIdx])
			m.maybeSlashComplete()
			return nil
		}
		if !m.ta.Focused() {
			m.vp.LineDown(1)
			return nil
		}

	case "tab":
		if m.ta.Focused() {
			m.ta.Blur()
		} else {
			m.ta.Focus()
		}
		return nil
	case "shift+tab":
		if m.ta.Focused() {
			m.ta.Blur()
		} else {
			m.ta.Focus()
		}
		return nil

	case "ctrl+l":
		if len(m.entries) > 5 && !m.confirmed {
			m.confirmed = true
			m.addSys("Clear " + fmt.Sprintf("%d messages? ", len(m.entries)) + "Press Ctrl+L again to confirm.")
			m.confirmAction = "clear"
			return nil
		}
		m.confirmed = false
		m.confirmAction = ""
		m.entries = nil
		m.messages = nil
		m.render()
		return nil

	case "home":
		if m.ta.Focused() {
			m.ta.SetCursor(0)
			return nil
		}
		m.vp.GotoTop()
		return nil
	case "end":
		if m.ta.Focused() {
			m.ta.SetCursor(len(m.ta.Value()))
			return nil
		}
		m.vp.GotoBottom()
		return nil

	case "pgup":
		if !m.ta.Focused() {
			m.vp.HalfViewUp()
			return nil
		}
	case "pgdown":
		if !m.ta.Focused() {
			m.vp.HalfViewDown()
			return nil
		}

	case " ":
		if !m.ta.Focused() {
			m.scrollLocked = !m.scrollLocked
			if m.scrollLocked {
				m.toastNow("Scroll locked - press Space to follow")
			} else {
				m.vp.GotoBottom()
				m.toastNow("Following output")
			}
			return nil
		}

	case "g", "G":
		if !m.ta.Focused() {
			m.vp.GotoBottom()
			m.scrollLocked = false
			return nil
		}

	case "ctrl+w":
		if !m.ta.Focused() {
			if m.wsData == nil {
				m.wsData = &workspace{project: m.projectName(), badge: "Ready"}
				m.wsData.gitRefresh()
			}
			m.wsData.shown = !m.wsData.shown
			m.wsData.gitRefresh()
			if m.wsData.shown {
				m.notify("workspace view opened")
			}
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
	case "u":
		if !m.ta.Focused() {
			return m.undoLast()
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

	if !m.ta.Focused() && len(msg.String()) == 2 {
		switch {
		case msg.String()[0] == 'y' && msg.String()[1] >= '1' && msg.String()[1] <= '9':
			return m.copyBlock(int(msg.String()[1] - '1'))
		case msg.String()[0] == 's' && msg.String()[1] >= '1' && msg.String()[1] <= '9':
			return m.saveBlock(int(msg.String()[1] - '1'))
		}
	}

	if !m.ta.Focused() {
		m.ta.Focus()
	}
	m.ta, _ = m.ta.Update(msg)
	m.maybeSlashComplete()
	return nil
}

func (m *model) maybeSlashComplete() {
	val := m.ta.Value()
	if strings.HasPrefix(val, "/") && !strings.Contains(val, " ") {
		m.refreshSlashDropdown(val)
		m.dd.setOpen(len(m.dd.filtered) > 0)
		return
	}
	if m.dd.kind == ddSlash {
		m.dd.setOpen(false)
	}
}

func (m *model) refreshSlashDropdown(query string) {
	all := slashItems()
	var filtered []ddItem
	q := strings.ToLower(query)
	for _, it := range all {
		if strings.HasPrefix(strings.ToLower(it.label), q) {
			filtered = append(filtered, it)
		}
	}
	m.dd.kind = ddSlash
	m.dd.items = all
	m.dd.filtered = filtered
	m.dd.title = "Commands"
	if m.dd.cursor >= len(filtered) {
		m.dd.cursor = 0
	}
}

func (m *model) applySlashItem(value string) tea.Cmd {
	m.dd.setOpen(false)
	m.ta.SetValue("")
	return m.slash(value)
}

func (m *model) findLastIdx(role string) int {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == role {
			return i
		}
	}
	return -1
}

// syncMessages rebuilds the request context from the visible entries so that
// edits, deletes and undos never send stale history to the provider.
func (m *model) syncMessages() {
	m.messages = m.messages[:0]
	for _, e := range m.entries {
		switch e.role {
		case "user":
			m.messages = append(m.messages, models.Message{Role: models.RoleUser, Content: e.content})
		case "assistant":
			m.messages = append(m.messages, models.Message{Role: models.RoleAssistant, Content: e.content})
		}
	}
}

func (m *model) editLastUser() tea.Cmd {
	idx := m.findLastIdx("user")
	if idx < 0 {
		return nil
	}
	m.ta.SetValue(m.entries[idx].content)
	m.entries = m.entries[:idx]
	m.syncMessages()
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
	m.syncMessages()
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
	m.syncMessages()
	m.render()
	return nil
}

func (m *model) undoLast() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == "user" {
			m.entries = m.entries[:i]
			m.syncMessages()
			m.render()
			m.toastNow("Last exchange undone")
			return nil
		}
		if m.entries[i].role == "assistant" {
			continue
		}
	}
	return nil
}

func (m *model) copyMsg() tea.Cmd {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == "assistant" || m.entries[i].role == "user" {
			content := m.entries[i].content
			if err := clipboardWrite(content); err != nil {
				m.addSys("Copy failed: " + err.Error())
			} else {
				m.toastNow("Copied to clipboard")
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
		m.messages = nil
		m.render()
	case "/help":
		m.addSys("----- Commands -----")
		m.addSys("/clear      Clear all messages")
		m.addSys("/copy       Copy entire transcript")
		m.addSys("/cost       Show session cost & tokens")
		m.addSys("/export     Save transcript to file")
		m.addSys("/help       Show this help")
		m.addSys("/minimal    Toggle minimal mode")
		m.addSys("/model      Switch model")
		m.addSys("/new        Start a new session")
		m.addSys("/provider   Switch provider")
		m.addSys("/search     Search conversation")
		m.addSys("/sessions   List & resume sessions")
		m.addSys("/stats      Session statistics")
		m.addSys("/theme      Cycle color themes")
		m.addSys("/think      Toggle reasoning display")
		m.addSys("/tips       Show usage tips")
		m.addSys("/undo       Remove last exchange")
		m.addSys("/wrap       Toggle word wrap")
		m.addSys("/agent      Toggle agent task mode")
		m.addSys("/agent-task Set agent task type")
		m.addSys("/auto-fix   Run autonomous fix loop")
		m.addSys("/checkpoint Save checkpoint")
		m.addSys("/checkpoints List checkpoints")
		m.addSys("/restore    Restore checkpoint")
		m.addSys("/run        Run last code block")
		m.addSys("")
		m.addSys("----- Keys -----")
		m.addSys("Enter       Send message")
		m.addSys("Shift+Enter New line")
		m.addSys("Esc         Stop streaming")
		m.addSys("Ctrl+K      Command palette")
		m.addSys("Ctrl+M      Model dropdown")
		m.addSys("Ctrl+P      Provider dropdown")
		m.addSys("Ctrl+N      New session")
		m.addSys("Ctrl+S      Save session")
		m.addSys("Tab         Focus chat/input")
		m.addSys("Up/Down     Input history (when focused)")
		m.addSys("j/k         Scroll chat (when unfocused)")
		m.addSys("Space       Toggle scroll lock")
		m.addSys("g/G         Scroll to top/bottom")
		m.addSys("PgUp/PgDn   Page scroll")
		m.addSys("E           Edit last message")
		m.addSys("R           Resend last message")
		m.addSys("D           Delete last user message")
		m.addSys("U           Undo last exchange")
		m.addSys("C           Copy last message")
		m.addSys("y / y1-9    Copy code block")
		m.addSys("w / s1-9    Save code block")
		m.addSys("T           Toggle reasoning")
		m.addSys("Ctrl+L      Clear all")
		m.addSys("Ctrl+C      Quit")
	case "/cost":
		m.addSys(fmt.Sprintf("Cost: $%.4f  |  Tokens: %d", m.cost, m.tok))
	case "/model":
		if len(p) > 1 {
			m.modelName = p[1]
			m.addSys("Switched to: " + p[1])
		} else {
			m.openDropdown(ddModel)
		}
	case "/provider":
		if len(p) > 1 {
			m.provName = p[1]
			m.addSys("Switched to: " + p[1])
		} else {
			m.openDropdown(ddProvider)
		}
	case "/think":
		m.reasoningVisible = !m.reasoningVisible
		m.addSys(fmt.Sprintf("Reasoning: %v", m.reasoningVisible))
	case "/theme":
		m.cycleTheme()
	case "/minimal":
		m.minimal = !m.minimal
		m.addSys(fmt.Sprintf("Minimal mode: %v", m.minimal))
	case "/wrap":
		m.wrapEnabled = !m.wrapEnabled
		m.addSys(fmt.Sprintf("Word wrap: %v", m.wrapEnabled))
	case "/stats":
		m.showStats()
	case "/export":
		return m.exportTranscript()
	case "/copy":
		return m.copyTranscript()
	case "/search":
		if len(p) > 1 {
			m.searchChat(strings.Join(p[1:], " "))
		} else {
			m.addSys("Usage: /search <query>")
		}
	case "/sessions":
		m.showSessions()
	case "/new":
		return m.newSession(false)
	case "/undo":
		return m.undoLast()
	case "/tips":
		m.showTips()
	case "/workspace":
		if m.wsData == nil {
			m.wsData = &workspace{project: m.projectName(), badge: "Ready"}
		}
		m.wsData.shown = true
		m.wsData.gitRefresh()
	case "/keys":
		m.helpShown = !m.helpShown
	case "/summary":
		m.addSys(m.sessionSummary())
	case "/export-json":
		path := filepath.Join(sessionsDir(), "export-"+time.Now().Format("20060102-150405")+".json")
		if err := m.exportJSON(path); err != nil {
			m.addErr(err.Error())
		} else {
			m.addSys("Exported JSON: " + path)
		}
	case "/agent":
		m.toggleAgentMode()
	case "/agent-task":
		if len(p) > 1 {
			m.setTaskType(strings.ToLower(p[1]))
		} else {
			m.addSys("Usage: /agent-task <general|coding|refactor|debug|test>")
		}
	case "/auto-fix":
		goal := strings.Join(p[1:], " ")
		if goal == "" {
			goal = m.lastPrompt
		}
		m.startAutoFix(goal)
	case "/checkpoint":
		m.saveCheckpoint("")
	case "/checkpoints":
		cps := m.listCheckpoints()
		if len(cps) == 0 {
			m.addSys("No checkpoints yet.")
		} else {
			for _, cp := range cps {
				m.addSys(fmt.Sprintf("  %s  %s  %s", cp.ID[:8], cp.Timestamp.Format("15:04:05"), truncateStr(cp.Label, 32)))
			}
		}
	case "/restore":
		if len(p) > 1 {
			m.restoreCheckpoint(p[1])
		} else {
			cps := m.listCheckpoints()
			if len(cps) == 0 {
				m.addSys("No checkpoints to restore.")
			} else {
				m.addSys("Latest: " + cps[0].ID)
				m.restoreCheckpoint(cps[0].ID)
			}
		}
	case "/run":
		m.runLastCodeBlock()
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
	m.exchanges++
	if m.wsData == nil {
		m.wsData = &workspace{project: m.projectName(), badge: "Generating"}
	}
	m.wsData.badge = "Generating"
	m.wsData.goal = prompt
	m.notify("started: " + truncateStr(prompt, 48))
	m.stopOnce = &stopOnce{ch: make(chan struct{})}
	m.stopCh = m.stopOnce.ch
	m.sb.Reset()
	m.rb.Reset()
	m.tokSpeed = 0
	m.lastStreamLen = 0
	m.lastStreamTime = time.Now()
	m.render()
	m.vp.GotoBottom()
	m.atBottom = true
	m.scrollLocked = false

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
		ctxPrompt = m.ctxEng.CachedPrompt(prompt)
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
	stop := m.stopOnce

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
				m.toastNow("Copied " + lang + " code")
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
				m.toastNow("Created " + path)
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
	m.reasoningVisible = !m.reasoningVisible
	m.addSys(fmt.Sprintf("Reasoning display: %v", m.reasoningVisible))
}

func (m *model) cancelStream() {
	m.streaming = false
	m.statusText = "Cancelled"
	m.ta.Focus()
	if m.stopOnce != nil {
		m.stopOnce.close()
		m.stopOnce = nil
	}
	m.stopCh = nil
	m.finalizePartial("(cancelled)")
	m.saveSession()
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
