package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Spec Scenario: InitScreen uses RenderProgress ---
func TestInitScreen_UsesRenderProgress(t *testing.T) {
	m := NewInitScreen(80, ".", nil)

	view := m.View()

	// Must contain bracketed current step (step 0 = Welcome)
	if !strings.Contains(view, "[Welcome]") {
		t.Errorf("InitScreen View should show bracketed current step; got:\n%s", view)
	}
	// Must contain future step names
	if !strings.Contains(view, "Description") {
		t.Errorf("InitScreen View should show future step 'Description'; got:\n%s", view)
	}
	// Must contain the expected separator
	if !strings.Contains(view, " → ") {
		t.Errorf("InitScreen View should contain ' → ' separator; got:\n%s", view)
	}
}

// --- Spec Scenario: InitScreen uses DynamicFormModel for description ---
func TestInitScreen_UsesDynamicFormForDescription(t *testing.T) {
	m := NewInitScreen(80, ".", nil)
	m.step = stepDescription

	view := m.View()

	// Should render field name from DynamicFormModel
	if !strings.Contains(view, "Description") {
		t.Errorf("InitScreen at stepDescription should render 'Description' field; got:\n%s", view)
	}

	// Should contain a text input placeholder
	if !strings.Contains(view, "Enter a one-sentence description") {
		t.Errorf("Should contain DynamicForm text placeholder; got:\n%s", view)
	}
}

// --- Spec Scenario: InitScreen step content wrapped in BoxStyle ---
func TestInitScreen_BoxStyleWrapping(t *testing.T) {
	m := NewInitScreen(80, ".", nil)
	m.step = stepDescription

	view := m.View()

	// BoxStyle adds rounded borders. We verify description content still appears
	if !strings.Contains(view, "Description") {
		t.Errorf("BoxStyle-wrapped content should still show field name; got:\n%s", view)
	}
}

// --- Spec Scenario: InitScreen saves ProjectConfig from DynamicForm values ---
func TestInitScreen_SavesFromDynamicForm(t *testing.T) {
	tmpDir := t.TempDir()

	// Create model and advance to description step
	m := NewInitScreen(80, tmpDir, nil)
	m.step = stepDescription

	// Type a description into the DynamicFormModel
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("My test project")})
	m = *updated.(*InitScreen)

	// Verify DynamicFormModel captured the value
	vals := m.descForm.Values()
	if vals["description"] != "My test project" {
		t.Errorf("DynamicForm should have captured description; got %q", vals["description"])
	}

	// Advance through base branch and grammars to review
	m.step = stepBaseBranch
	m.step = stepGrammars
	m.downloading = false
	m.step = stepReview

	// Save
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)

	if !m.confirmed {
		t.Fatalf("Save should be confirmed; err=%v", m.err)
	}

	// Verify config was saved to disk
	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "My test project" {
		t.Errorf("Saved description = %q, want %q", loaded.Description, "My test project")
	}
}

// Triangulation: InitScreen progress at different steps
func TestInitScreen_ProgressAtDescriptionStep(t *testing.T) {
	m := NewInitScreen(80, ".", nil)
	m.step = stepDescription

	view := m.View()
	if !strings.Contains(view, "[Description]") {
		t.Errorf("At step 1, should bracket 'Description'; got:\n%s", view)
	}
	if !strings.Contains(view, "✓ Welcome") {
		t.Errorf("At step 1, 'Welcome' should be completed; got:\n%s", view)
	}
}

// Triangulation: InitScreen progress matches components.RenderProgress
func TestInitScreen_RenderProgressMatchesComponent(t *testing.T) {
	steps := progressStepsList()

	m := NewInitScreen(80, ".", nil)
	for step := 0; step <= 6; step++ {
		m.step = step
		view := m.View()
		expected := components.RenderProgress(steps, step)

		if !strings.Contains(view, expected) {
			t.Errorf("Step %d: View should contain RenderProgress output;\nview:\n%s\nexpected progress:\n%s", step, view, expected)
		}
	}
}

// Triangulation: InitScreen wizard flow from start to finish
func TestInitScreen_WizardFlow(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewInitScreen(80, tmpDir, nil)

	// Step 0: Welcome
	if m.step != stepWelcome {
		t.Fatalf("Initial step should be Welcome; got %d", m.step)
	}

	// Advance to Description
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepDescription {
		t.Fatalf("After enter on welcome, should be at Description; got %d", m.step)
	}

	// Type a description
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Test project")})
	m = *updated.(*InitScreen)

	// Advance to Base Branch
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepBaseBranch {
		t.Fatalf("After enter on description, should be at BaseBranch; got %d", m.step)
	}

	// Advance to Test Command (accept default base branch)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepTestCommand {
		t.Fatalf("After enter on base branch, should be at TestCommand; got %d", m.step)
	}

	// Advance to Grammars (accept default test command)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepGrammars {
		t.Fatalf("After enter on test command, should be at Grammars; got %d", m.step)
	}

	m.downloading = false

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepReview {
		t.Fatalf("After enter on grammars, should be at Review; got %d", m.step)
	}

	// Verify review shows our description
	view := m.View()
	if !strings.Contains(view, "Test project") {
		t.Errorf("Review should show description 'Test project'; got:\n%s", view)
	}

	// Save on Review
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if !m.confirmed {
		t.Fatalf("Save should be confirmed; err=%v", m.err)
	}
	if m.step != stepFinish {
		t.Fatalf("After save, should be at Finish; got %d", m.step)
	}

	// Verify saved config
	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "Test project" {
		t.Errorf("Saved description = %q, want %q", loaded.Description, "Test project")
	}
}

// --- Spec Scenario: Init wizard completes without stepAreas (freeform categories) ---
func TestInitScreen_CompletesWithoutAreas(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewInitScreen(80, tmpDir, nil)

	// Step 0: Welcome
	if m.step != stepWelcome {
		t.Fatalf("Initial step should be Welcome; got %d", m.step)
	}

	// Advance to Description
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepDescription {
		t.Fatalf("After enter on welcome, should be at Description; got %d", m.step)
	}

	// Type a description
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Freeform project")})
	m = *updated.(*InitScreen)

	// Advance to Base Branch
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepBaseBranch {
		t.Fatalf("After enter on description, should be at BaseBranch; got %d", m.step)
	}

	// Advance to Test Command (NO Areas step in freeform mode)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepTestCommand {
		t.Fatalf("After enter on base branch, should be at TestCommand (skipping Areas); got %d", m.step)
	}

	// Advance to Grammars (accept default test command)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepGrammars {
		t.Fatalf("After enter on test command, should be at Grammars; got %d", m.step)
	}

	// Complete grammars step
	m.downloading = false
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.step != stepReview {
		t.Fatalf("After enter on grammars, should be at Review; got %d", m.step)
	}

	// Verify review shows description but NO areas section
	view := m.View()
	if !strings.Contains(view, "Freeform project") {
		t.Errorf("Review should show description 'Freeform project'; got:\n%s", view)
	}

	// Save on Review
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if !m.confirmed {
		t.Fatalf("Save should be confirmed; err=%v", m.err)
	}
	if m.step != stepFinish {
		t.Fatalf("After save, should be at Finish; got %d", m.step)
	}

	// Verify saved config
	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "Freeform project" {
		t.Errorf("Saved description = %q, want %q", loaded.Description, "Freeform project")
	}
}

// Triangulation: InitScreen description entered via DynamicForm reflects in review
func TestInitScreen_DescriptionFlowsToReview(t *testing.T) {
	m := NewInitScreen(80, ".", nil)
	m.step = stepDescription

	// Type description into DynamicForm
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Review flow test")})
	m = *updated.(*InitScreen)

	// Jump to review step
	m.step = stepReview

	view := m.View()
	if !strings.Contains(view, "Review flow test") {
		t.Errorf("Review should show description from DynamicForm; got:\n%s", view)
	}
}

// Triangulation: Config save failure sets error
func TestInitScreen_SaveFailureSetsError(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, domain.MetadataDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(configDir, 0555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(configDir, 0755)

	m := NewInitScreen(80, tmpDir, nil)
	m.step = stepReview

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *updated.(*InitScreen)
	if m.err == nil {
		t.Error("Error should be set when save fails")
	}
	if m.confirmed {
		t.Error("Should not be confirmed when save fails")
	}
}

// Suppress unused import warning
var _ = components.DynFieldText
