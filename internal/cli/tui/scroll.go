package tui

import (
	"time"
)

const scrollAccelerationEnabled = true
const scrollBaseSpeed = 1
const scrollMaxSpeed = 10
const scrollAccelWindow = 500 * time.Millisecond

type scrollState struct {
	lastTime time.Time
	count    int
	speed    int
}

func (m *model) scrollUp(n int) {
	if scrollAccelerationEnabled {
		m.scrollAccel(n, true)
	} else {
		m.vp.LineUp(n)
	}
}

func (m *model) scrollDown(n int) {
	if scrollAccelerationEnabled {
		m.scrollAccel(n, false)
	} else {
		m.vp.LineDown(n)
	}
}

func (m *model) scrollAccel(n int, up bool) {
	now := time.Now()
	if now.Sub(m.scrollState.lastTime) > scrollAccelWindow {
		m.scrollState.count = 0
		m.scrollState.speed = scrollBaseSpeed
	}
	m.scrollState.lastTime = now
	m.scrollState.count++
	if m.scrollState.count > 5 && m.scrollState.speed < scrollMaxSpeed {
		m.scrollState.speed++
	}
	for i := 0; i < m.scrollState.speed; i++ {
		if up {
			m.vp.LineUp(1)
		} else {
			m.vp.LineDown(1)
		}
	}
}

func (m *model) halfPageUp() {
	m.vp.HalfViewUp()
}

func (m *model) halfPageDown() {
	m.vp.HalfViewDown()
}

func (m *model) pageUp() {
	m.vp.PageUp()
}

func (m *model) pageDown() {
	m.vp.PageDown()
}

func (m *model) gotoTop() {
	m.vp.GotoTop()
	m.scrollLocked = false
}

func (m *model) gotoBottom() {
	m.vp.GotoBottom()
	m.scrollLocked = false
}
