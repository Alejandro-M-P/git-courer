// Package components provides reusable TUI components for the git-courer wizard.
package components

import (
	"strings"

	"github.com/Alejandro-M-P/git-courer/tui/styles"
)

// RenderProgress renders a step-based progress indicator for multi-step TUI wizards.
// The current step is rendered with SelectedStyle and bracketed: [StepName]
// Completed steps (index < current) are rendered with SuccessStyle and checkmark: ✓ StepName
// Future steps (index > current) are rendered with SubtextStyle with no prefix.
// Steps are joined with " → " separator.
// Returns empty string for empty steps list.
func RenderProgress(steps []string, current int) string {
	if len(steps) == 0 {
		return ""
	}

	var result []string
	for i, step := range steps {
		if i == current {
			result = append(result, styles.SelectedStyle.Render("["+step+"]"))
		} else if i < current {
			result = append(result, styles.SuccessStyle.Render("✓ "+step))
		} else {
			result = append(result, styles.SubtextStyle.Render(step))
		}
	}
	return strings.Join(result, " → ")
}
