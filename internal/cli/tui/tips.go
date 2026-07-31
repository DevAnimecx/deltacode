package tui

import (
	"fmt"
	"math/rand"
)

var tips = []string{
	"Tip: Press Ctrl+K for the command palette.",
	"Tip: Press Ctrl+M to switch models without typing.",
	"Tip: Type / then see live command suggestions.",
	"Tip: Press y to copy the last code block, y2 for the second one.",
	"Tip: Press w to save the last code block to a file.",
	"Tip: Press Space (chat unfocused) to freeze scrolling.",
	"Tip: Press U to undo the last exchange.",
	"Tip: Press R to resend and edit your last prompt with E.",
	"Tip: /export saves your transcript as Markdown.",
	"Tip: Press Ctrl+N twice to start a fresh session.",
	"Tip: /theme cycles through color schemes.",
	"Tip: /stats shows tokens, cost, and code blocks.",
	"Tip: /sessions lists saved chats — press 1-9 to resume.",
	"Tip: /search finds keywords in the conversation.",
	"Tip: Shift+Enter inserts a newline in the input box.",
	"Tip: Press T to expand or collapse reasoning traces.",
	"Tip: /wrap toggles word wrap for long answers.",
	"Tip: /minimal hides decorations for distraction-free work.",
	"Tip: Ctrl+U clears the current input line.",
	"Tip: Ctrl+W deletes the word before the cursor.",
}

func (m *model) showTip() {
	m.addSys(tips[rand.Intn(len(tips))])
}

func (m *model) showTips() {
	for _, t := range tips {
		m.addSys(t)
	}
	m.addSys(fmt.Sprintf("━━━ %d tips ━━━", len(tips)))
}
