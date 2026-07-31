package tui

import "github.com/DevAnimecx/deltacode/pkg/models"

type redoStack struct {
	entries  []entry
	messages []models.Message
}

func (m *model) pushUndo(e entry, msg models.Message) {
	if m.undoStack == nil {
		m.undoStack = &redoStack{}
	}
	m.undoStack.entries = append(m.undoStack.entries, e)
	m.undoStack.messages = append(m.undoStack.messages, msg)
}

func (m *model) popUndo() (entry, models.Message, bool) {
	if m.undoStack == nil || len(m.undoStack.entries) == 0 {
		return entry{}, models.Message{}, false
	}
	n := len(m.undoStack.entries) - 1
	e := m.undoStack.entries[n]
	msg := m.undoStack.messages[n]
	m.undoStack.entries = m.undoStack.entries[:n]
	m.undoStack.messages = m.undoStack.messages[:n]
	return e, msg, true
}
