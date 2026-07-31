package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/router"
	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// fakeSSE returns a server that answers /chat/completions with a streaming
// OpenAI-style SSE response: two content deltas, a usage/finish chunk and
// [DONE]. It records the number of requests received.
func fakeSSE(t *testing.T, content []string) (string, *int) {
	t.Helper()
	reqs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs++
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		var body struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fl := w.(http.Flusher)
		for _, c := range content {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q},\"index\":0}]}\n\n", c)
			fl.Flush()
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\",\"index\":0}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3,\"total_tokens\":8}}\n\n")
		fl.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &reqs
}

// testCfg builds an isolated config manager with a provider pointing at the
// fake server.
func testCfg(t *testing.T, baseURL string) *config.Manager {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManagerAt(filepath.Join(t.TempDir(), ".delta"))
	if err != nil {
		t.Fatalf("config.NewManagerAt: %v", err)
	}
	if err := cfg.AddProvider(models.ProviderConfig{
		Name:    "test-prov",
		Type:    models.ProviderOpenAI,
		BaseURL: baseURL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}); err != nil {
		t.Fatalf("AddProvider: %v", err)
	}
	if err := cfg.SetDefault("test-prov"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	return cfg
}

// testModel builds a headless TUI model (no terminal, no memory engines).
func testModel(t *testing.T, cfg *config.Manager) *model {
	t.Helper()
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("43"))
	ta := textarea.New()
	ta.Placeholder = " test "
	ta.Prompt = "┃ "
	ta.CharLimit = 0
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.Focus()
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
		w:          80, h: 24, glam: g, atBottom: true,
		inputHistory:     []string{},
		historyIdx:       -1,
		reasoningVisible: true,
		collapseLong:     true,
		wrapEnabled:      true,
	}
	m.rtr = router.NewRouter(m.provName, m.modelName)
	m.vp.Width = 78
	m.vp.Height = 14
	m.ta.SetWidth(74)
	m.ta.SetHeight(3)
	return m
}

// drainResponse feeds every streamed chunk back through Update until done,
// simulating the bubbletea event loop. It returns the concatenated content.
func drainResponse(t *testing.T, m *model) string {
	t.Helper()
	var got strings.Builder
	timeout := time.After(10 * time.Second)
	for {
		select {
		case c, ok := <-m.streamCh:
			if !ok {
				m.Update(chunk{done: true})
				return got.String()
			}
			got.WriteString(c.content)
			m.Update(c)
			if c.done {
				return got.String()
			}
		case <-timeout:
			t.Fatalf("timed out waiting for stream")
		}
	}
}

// pump submits a prompt and drains the response.
func pump(t *testing.T, m *model, prompt string) string {
	t.Helper()
	m.submit(prompt)
	return drainResponse(t, m)
}

// drain consumes the remaining buffered chunks after a cancel/stop.
func drain(t *testing.T, m *model) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-m.streamCh:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("timed out draining stream")
		}
	}
}

func TestSendReceivesStreamedResponse(t *testing.T) {
	url, reqs := fakeSSE(t, []string{"Hello", ", ", "world!"})
	cfg := testCfg(t, url)
	m := testModel(t, cfg)

	got := pump(t, m, "hello")
	if got != "Hello, world!" {
		t.Fatalf("content = %q, want %q", got, "Hello, world!")
	}
	if *reqs != 1 {
		t.Fatalf("requests = %d, want 1", *reqs)
	}
	if m.streaming {
		t.Fatal("still streaming after done")
	}
	if m.exchanges != 1 {
		t.Fatalf("exchanges = %d, want 1", m.exchanges)
	}
	if len(m.messages) != 2 {
		t.Fatalf("messages = %d, want 2 (user + assistant)", len(m.messages))
	}
	last := lastEntry(m, "assistant")
	if last == nil || last.content != "Hello, world!" {
		t.Fatalf("last assistant content = %q, want %q", last.content, "Hello, world!")
	}
	if m.tok <= 0 {
		t.Fatalf("tok = %d, want > 0", m.tok)
	}
	if m.sb.Len() != 0 || m.rb.Len() != 0 {
		t.Fatal("streaming buffers not reset after finish")
	}
}

// lastEntry returns the most recent entry with the given role, or nil.
func lastEntry(m *model, role string) *entry {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == role {
			return &m.entries[i]
		}
	}
	return nil
}

func TestCancelKeepsPartial(t *testing.T) {
	url, _ := fakeSSE(t, []string{"partial", " text"})
	cfg := testCfg(t, url)
	m := testModel(t, cfg)

	m.submit("hello")
	c := <-m.streamCh
	m.Update(c) // "partial"
	if m.entries[len(m.entries)-1].role != "streaming" {
		t.Fatal("expected a streaming entry")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}) // cancel

	if m.streaming {
		t.Fatal("still streaming after cancel")
	}
	if m.statusText != "Cancelled" {
		t.Fatalf("statusText = %q, want Cancelled", m.statusText)
	}
	last := lastEntry(m, "assistant")
	if last == nil {
		t.Fatal("no assistant entry (partial not kept)")
	}
	if !strings.HasPrefix(last.content, "partial") || !strings.Contains(last.content, "(cancelled)") {
		t.Fatalf("partial not kept: %q", last.content)
	}
	if len(m.messages) != 2 {
		t.Fatalf("messages = %d, want 2 (user + partial assistant)", len(m.messages))
	}
	if m.sb.Len() != 0 || m.rb.Len() != 0 {
		t.Fatal("streaming buffers not reset after cancel")
	}

	drain(t, m)
	m.Update(chunk{done: true}) // trailing done must be a no-op
	if m.statusText != "Cancelled" {
		t.Fatalf("trailing done clobbered status: %q", m.statusText)
	}
	if len(m.entries) != 3 {
		t.Fatalf("trailing done appended entries: %d (user, kept partial, cancelled note)", len(m.entries))
	}
}

func TestStaleChunkIgnoredAfterCancel(t *testing.T) {
	url, _ := fakeSSE(t, []string{"a", "b"})
	cfg := testCfg(t, url)
	m := testModel(t, cfg)

	m.submit("hello")
	c := <-m.streamCh
	m.Update(c)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	entries := len(m.entries)

	m.Update(chunk{content: "stale", done: false})
	if len(m.entries) != entries {
		t.Fatalf("stale chunk created entries: %d -> %d", entries, len(m.entries))
	}
	if m.sb.Len() != 0 {
		t.Fatalf("stale chunk wrote buffer: %q", m.sb.String())
	}
	drain(t, m)
}

func TestToastExpiresAndKeepsTicking(t *testing.T) {
	cfg := testCfg(t, "http://127.0.0.1:1")
	m := testModel(t, cfg)

	m.toastNow("hello")
	if m.toast == nil {
		t.Fatal("toast not set")
	}
	_, cmd := m.Update(tick{})
	if cmd == nil {
		t.Fatal("expected tick to continue while toast active")
	}
	m.toast.until = time.Now().Add(-time.Millisecond)
	_, cmd = m.Update(tick{})
	if m.toast != nil {
		t.Fatal("toast should have expired")
	}
	if cmd != nil {
		t.Fatal("expected no tick after toast expiry (not streaming)")
	}
}

func TestUndoRemovesExchangeAndSyncsMessages(t *testing.T) {
	url, _ := fakeSSE(t, []string{"answer"})
	cfg := testCfg(t, url)
	m := testModel(t, cfg)

	pump(t, m, "question")
	if len(m.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(m.messages))
	}
	m.undoLast()
	if len(m.entries) != 0 || len(m.messages) != 0 {
		t.Fatalf("undo left entries=%d messages=%d, want 0/0", len(m.entries), len(m.messages))
	}
}

func TestResendLastUserRebuildsContext(t *testing.T) {
	url, _ := fakeSSE(t, []string{"answer"})
	cfg := testCfg(t, url)
	m := testModel(t, cfg)

	pump(t, m, "first")
	m.resendLastUser()
	got := drainResponse(t, m)
	if got != "answer" {
		t.Fatalf("resent response = %q, want %q", got, "answer")
	}
	if len(m.entries) != 2 {
		t.Fatalf("entries = %d, want 2 (fresh user+assistant)", len(m.entries))
	}
	if len(m.messages) != 2 {
		t.Fatalf("messages = %d, want 2 (stale context purged)", len(m.messages))
	}
	if m.entries[0].content != "first" {
		t.Fatalf("resent prompt = %q, want %q", m.entries[0].content, "first")
	}
}

func TestThemeChangeInvalidatesMDCache(t *testing.T) {
	cfg := testCfg(t, "http://127.0.0.1:1")
	m := testModel(t, cfg)
	m.entries = append(m.entries, entry{role: "assistant", content: "# Heading\n\nSome **bold** text."})

	m.render()
	if !m.entries[0].mdCached {
		t.Fatal("expected md cache to be populated after render")
	}
	m.cycleTheme()
	if m.entries[0].mdCached {
		t.Fatal("expected md cache to be invalidated by theme change")
	}
	m.render()
	if !m.entries[0].mdCached {
		t.Fatal("expected md cache repopulated after render")
	}
}

func TestSessionsDirUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".delta", "sessions")
	if got := sessionsDir(); got != want {
		t.Fatalf("sessionsDir = %q, want %q", got, want)
	}
	if got := sessionPath(); got != filepath.Join(want, "last.json") {
		t.Fatalf("sessionPath = %q", got)
	}
}

func TestNewSessionSavesBeforeClear(t *testing.T) {
	cfg := testCfg(t, "http://127.0.0.1:1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := testModel(t, cfg)
	m.messages = append(m.messages,
		models.Message{Role: models.RoleUser, Content: "keep me"},
		models.Message{Role: models.RoleAssistant, Content: "ok"})

	m.newSession(false)
	if len(m.messages) != 0 {
		t.Fatalf("messages = %d, want 0 after new session", len(m.messages))
	}
	b, err := os.ReadFile(filepath.Join(home, ".delta", "sessions", "last.json"))
	if err != nil {
		t.Fatalf("last.json not saved: %v", err)
	}
	if !strings.Contains(string(b), "keep me") {
		t.Fatalf("saved session missing prior messages: %s", b)
	}
}

func TestSlashCompletionOpensDropdown(t *testing.T) {
	cfg := testCfg(t, "http://127.0.0.1:1")
	m := testModel(t, cfg)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.dd.visible() {
		t.Fatal("slash dropdown should open after typing /")
	}
	if m.dd.kind != ddSlash {
		t.Fatalf("dropdown kind = %d, want ddSlash", m.dd.kind)
	}
	for _, r := range "search" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.dd.filtered) != 1 || m.dd.filtered[0].value != "/search" {
		t.Fatalf("filtered = %v, want [/search]", m.dd.filtered)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.dd.visible() {
		t.Fatal("dropdown should close on enter")
	}
	found := false
	for _, e := range m.entries {
		if e.role == "system" && strings.Contains(e.content, "/search") {
			found = true
		}
	}
	if !found {
		t.Fatal("slash selection should have executed /search")
	}
}

func TestStreamWorkerFallbackToNonStreaming(t *testing.T) {
	// A stream that produces no content must fall back to a plain Chat call.
	reqs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs++
		var body struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !body.Stream {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fallback answer"}}]}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
	}))
	t.Cleanup(srv.Close)

	cfg := testCfg(t, srv.URL)
	m := testModel(t, cfg)
	got := pump(t, m, "hello")
	if got != "fallback answer" {
		t.Fatalf("content = %q, want %q", got, "fallback answer")
	}
	if reqs < 2 {
		t.Fatalf("requests = %d, want >= 2 (stream + fallback)", reqs)
	}
}
