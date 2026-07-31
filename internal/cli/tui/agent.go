package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/autonomous"
	"github.com/DevAnimecx/deltacode/internal/sandbox"
	"github.com/DevAnimecx/deltacode/internal/timemachine"
)

// --- Agent Task Mode (feature 1) ---

func (m *model) toggleAgentMode() {
	m.agentMode = !m.agentMode
	if m.agentMode && m.taskType == "" {
		m.taskType = "general"
	}
	m.addSys(fmt.Sprintf("Agent mode: %v (task: %s)", m.agentMode, m.taskType))
}

func (m *model) setTaskType(t string) {
	m.taskType = t
	m.addSys(fmt.Sprintf("Task type: %s", t))
}

// --- Autonomous Fix Loop (feature 2) ---

func (m *model) startAutoFix(goal string) {
	if m.autoRunning {
		m.addSys("Autonomous loop already running.")
		return
	}
	if m.autoEng == nil {
		m.autoEng = autonomous.NewEngine(m.cfg)
	}
	m.autoRunning = true
	m.streaming = true
	m.statusText = "Auto-fix"
	m.startTime = time.Now()
	m.tokSpeed = 0
	m.lastStreamLen = 0
	m.lastStreamTime = time.Now()
	home, _ := os.UserHomeDir()
	m.autoEventsPath = ""
	if home != "" {
		m.autoEventsPath = filepath.Join(home, ".delta", "session", "events.jsonl")
	}
	m.autoEventsPos = 0
	m.addSys(fmt.Sprintf("Running autonomous fix loop: %s", goal))
	go func() {
		err := m.autoEng.Execute(goal)
		m.autoRunning = false
		if err != nil {
			m.addSys(fmt.Sprintf("Auto-fix error: %v", err))
		} else {
			m.addSys("Auto-fix complete.")
		}
	}()
}

func (m *model) pollAutoEvents() {
	if !m.autoRunning || m.autoEventsPath == "" {
		return
	}
	f, err := os.Open(m.autoEventsPath)
	if err != nil {
		return
	}
	defer f.Close()
	stat, _ := f.Stat()
	if stat.Size() < m.autoEventsPos {
		m.autoEventsPos = 0
	}
	if _, err := f.Seek(m.autoEventsPos, 0); err != nil {
		return
	}
	dec := json.NewDecoder(f)
	for dec.More() {
		var ev map[string]any
		if err := dec.Decode(&ev); err != nil {
			break
		}
		m.autoEventsPos, _ = f.Seek(0, 1)
		kind, _ := ev["kind"].(string)
		detail, _ := ev["detail"].(string)
		switch kind {
		case "task_start", "task_done":
			m.addSys(fmt.Sprintf("  %s: %s", kind, detail))
		case "retry", "validation_failed", "task_failed":
			m.addSys(fmt.Sprintf("  ⚠ %s: %s", kind, detail))
		case "quality_loop", "quality_fix":
			m.addSys(fmt.Sprintf("  ↻ %s: %s", kind, detail))
		}
	}
}

// --- Time-Machine Checkpoints (feature 3) ---

func (m *model) saveCheckpoint(label string) {
	if m.tm == nil {
		m.addSys("Timemachine not available.")
		return
	}
	if label == "" {
		label = "before-" + m.lastPrompt
		if len(label) > 40 {
			label = label[:40]
		}
	}
	lastResp := ""
	last := lastEntry(m, "assistant")
	if last != nil {
		lastResp = last.content
	}
	cp, err := m.tm.Save(label, m.lastPrompt, lastResp, m.modelName, m.provName)
	if err != nil {
		m.addSys("Checkpoint save failed: " + err.Error())
		return
	}
	m.addSys(fmt.Sprintf("Checkpoint saved: %s (%s)", cp.ID, label))
}

func (m *model) listCheckpoints() []timemachine.Checkpoint {
	if m.tm == nil {
		return nil
	}
	cps, _ := m.tm.List(20)
	return cps
}

func (m *model) restoreCheckpoint(id string) {
	if m.tm == nil {
		m.addSys("Timemachine not available.")
		return
	}
	if err := m.tm.Undo(id); err != nil {
		m.addSys("Checkpoint restore failed: " + err.Error())
		return
	}
	m.addSys("Restored checkpoint " + id)
	m.wsData = &workspace{project: m.projectName(), badge: "Restored"}
	m.wsData.gitRefresh()
	m.render()
}

// --- Sandboxed Run (feature 4) ---

func extractCodeBlock(content string) (string, string, string) {
	blocks := extractCodeBlocks(content)
	if len(blocks) == 0 {
		return "", "", ""
	}
	b := blocks[len(blocks)-1]
	return b.code, b.language, b.filename
}

func (m *model) runLastCodeBlock() {
	if m.sbx == nil {
		m.addSys("Sandbox not available.")
		return
	}
	last := lastEntry(m, "assistant")
	if last == nil || last.content == "" {
		m.addSys("No code to run.")
		return
	}
	code, lang, filename := extractCodeBlock(last.content)
	if code == "" {
		m.addSys("No code block found in last response.")
		return
	}
	m.addSys(fmt.Sprintf("Running %s in sandbox...", lang))
	go func() {
		if filename != "" {
			if err := m.sbx.WriteFile(filename, code); err != nil {
				m.addSys("Sandbox write failed: " + err.Error())
				return
			}
		}
		var res *sandbox.Result
		var err error
		if lang != "" {
			res, err = m.sbx.RunCommand(lang, filename)
		} else {
			res, err = m.sbx.RunShell(code)
		}
		if err != nil {
			m.addSys("Sandbox run error: " + err.Error())
			return
		}
		if res.Stdout != "" {
			m.addSys("stdout:\n" + res.Stdout)
		}
		if res.Stderr != "" {
			m.addErr("stderr: " + res.Stderr)
		}
		m.addSys(fmt.Sprintf("exit %d", res.ExitCode))
	}()
}

// --- Task card helpers (feature 1) ---

func lastEntry(m *model, role string) *entry {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == role {
			return &m.entries[i]
		}
	}
	return nil
}

func (m *model) taskCardText() string {
	if !m.agentMode || m.taskType == "" {
		return ""
	}
	return fmt.Sprintf("Task: %s — %s", strings.ToUpper(m.taskType), truncateStr(m.lastPrompt, 50))
}
