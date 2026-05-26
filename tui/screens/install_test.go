package screens

import (
	"strings"
	"testing"
)

// TestInstallScreen_UsesRenderProgress verifies that InstallScreen's View
// renders the progress bar via components.RenderProgress, not inline progressIndicator().
func TestInstallScreen_UsesRenderProgress(t *testing.T) {
	m := NewInstallScreen(80, nil)

	view := m.View()

	// Must contain bracketed current step (step 0 = Welcome)
	if !strings.Contains(view, "[Welcome]") {
		t.Errorf("InstallScreen View should show bracketed current step; got:\n%s", view)
	}
	// Must contain future step names
	if !strings.Contains(view, "MCP Config") {
		t.Errorf("InstallScreen View should show future step 'MCP Config'; got:\n%s", view)
	}
	// Must contain the expected separator
	if !strings.Contains(view, " → ") {
		t.Errorf("InstallScreen View should contain ' → ' separator; got:\n%s", view)
	}
}

// TestInstallScreen_ProgressAtDifferentSteps verifies progress rendering at different steps.
func TestInstallScreen_ProgressAtDifferentSteps(t *testing.T) {
	m := NewInstallScreen(80, nil)
	m.step = 2 // LLM Config step

	view := m.View()
	if !strings.Contains(view, "[LLM Config]") {
		t.Errorf("At step 2, should bracket 'LLM Config'; got:\n%s", view)
	}
	if !strings.Contains(view, "✓ Welcome") {
		t.Errorf("At step 2, 'Welcome' should be completed; got:\n%s", view)
	}
	if !strings.Contains(view, "✓ MCP Config") {
		t.Errorf("At step 2, 'MCP Config' should be completed; got:\n%s", view)
	}
}