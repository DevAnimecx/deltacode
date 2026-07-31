package tui

import (
	"os/exec"
	"runtime"
)

type attentionConfig struct {
	Enabled   bool
	Notify    bool
	Sound     bool
	Volume    float64
	SoundPack string
	Sounds    map[string]string
}

func defaultAttention() attentionConfig {
	return attentionConfig{
		Enabled:   false,
		Notify:    true,
		Sound:     true,
		Volume:    0.4,
		SoundPack: "opencode.default",
	}
}

func (m *model) loadAttention() attentionConfig {
	return defaultAttention()
}

func (m *model) attentionNotify(event string) {
	cfg := m.loadAttention()
	if !cfg.Enabled || !cfg.Notify {
		return
	}
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("powershell", "-Command", "Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show('Delta Code: "+event+"', 'Delta Code', 'OK', 'Information')").Start()
	case "darwin":
		_ = exec.Command("osascript", "-e", "display notification \""+event+"\" with title \"Delta Code\"").Start()
	case "linux":
		_ = exec.Command("notify-send", "Delta Code", event).Start()
	}
}

func (m *model) attentionSound(event string) {
	cfg := m.loadAttention()
	if !cfg.Enabled || !cfg.Sound {
		return
	}
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("powershell", "-Command", "[Console]::Beep(800, 200)").Start()
	case "darwin":
		_ = exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Start()
	case "linux":
		_ = exec.Command("paplay", "/usr/share/sounds/freedesktop/stereo/complete.oga").Start()
	}
}

func (m *model) notifyAttention(event string) {
	m.attentionNotify(event)
	m.attentionSound(event)
}
