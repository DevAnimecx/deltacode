package tui

import "strings"

type diffStyle string

const (
	diffStyleAuto    diffStyle = "auto"
	diffStyleStacked diffStyle = "stacked"
)

func (m *model) renderDiff(oldText, newText string) string {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")
	var out []string
	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}
	for i := 0; i < max; i++ {
		var prefix, line string
		if i < len(oldLines) && i < len(newLines) {
			if oldLines[i] == newLines[i] {
				prefix = "  "
				line = oldLines[i]
			} else {
				prefix = "- "
				line = oldLines[i]
				out = append(out, m.t.dim.Render("  "+prefix+line))
				prefix = "+ "
				line = newLines[i]
			}
		} else if i >= len(newLines) {
			prefix = "- "
			line = oldLines[i]
		} else {
			prefix = "+ "
			line = newLines[i]
		}
		if prefix == "- " {
			out = append(out, m.t.errM.Render("  "+prefix+line))
		} else if prefix == "+ " {
			out = append(out, m.t.badge.Render("  "+prefix+line))
		} else {
			out = append(out, m.t.dim.Render("  "+prefix+line))
		}
	}
	return strings.Join(out, "\n")
}
