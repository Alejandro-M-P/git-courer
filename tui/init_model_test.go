package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	tea "github.com/charmbracelet/bubbletea"
)

// mockLLMClient is a test double for LLMInitClient interface.
type mockLLMClient struct {
	available    bool
	projectInit  *domain.ProjectConfig
	projectError error
}
func (m *mockLLMClient) IsAvailable() bool { return m.available }

func (m *mockLLMClient) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) {
	if m.projectError != nil {
		return nil, m.projectError
	}
	return m.projectInit, nil
}

// --- Test: View renders description + areas in review state ---

func TestInitModel_ViewRendersDescriptionAndAreas(t *testing.T) {
	projectConfig := &domain.ProjectConfig{
		Description: "A git helper project",
		Areas: map[string][]string{
			"core":     {"internal/core/"},
			"adapters": {"internal/adapters/"},
		},
	}

	m := NewInitModel(80, 24, ".", projectConfig, nil)
	// When config is provided, model starts in review state

	view := m.View()

	if !strings.Contains(view, "A git helper project") {
		t.Errorf("View should contain description; got:\n%s", view)
	}
	if !strings.Contains(view, "core") {
		t.Errorf("View should contain area 'core'; got:\n%s", view)
	}
	if !strings.Contains(view, "internal/core/") {
		t.Errorf("View should contain area path 'internal/core/'; got:\n%s", view)
	}
	if !strings.Contains(view, "adapters") {
		t.Errorf("View should contain area 'adapters'; got:\n%s", view)
	}
}

// --- Test: View renders empty form when no config (LLM unavailable) ---

func TestInitModel_ViewRendersEmptyFormWithoutConfig(t *testing.T) {
	m := NewInitModel(80, 24, ".", nil, &mockLLMClient{available: false})
	// When no config and LLM unavailable, model starts in review mode with empty fields

	view := m.View()

	if !strings.Contains(view, "Description") {
		t.Errorf("View should show Description field for manual entry; got:\n%s", view)
	}
}

// --- Test: Confirm saves config ---

func TestInitModel_ConfirmSavesConfig(t *testing.T) {
	tmpDir := t.TempDir()

	projectConfig := &domain.ProjectConfig{
		Description: "Test project for confirm",
		Areas: map[string][]string{
			"auth": {"internal/auth/"},
		},
	}

	m := NewInitModel(80, 24, tmpDir, projectConfig, nil)
	m.setState(stateInitConfirm)

	// Press 'y' to confirm and save
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	// Verify config was saved to disk
	configPath := filepath.Join(tmpDir, ".git-courer", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file should exist after confirm")
	}

	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "Test project for confirm" {
		t.Errorf("Description = %q, want %q", loaded.Description, "Test project for confirm")
	}
	if len(loaded.Areas) != 1 || len(loaded.Areas["auth"]) != 1 {
		t.Errorf("Areas[auth] = %v, want [internal/auth/]", loaded.Areas["auth"])
	}

	// Model should be done
	initM := updatedModel.(InitModel)
	if !initM.done {
		t.Error("Model should be done after confirm+save")
	}
}

// --- Test: Cancel from review does not save config ---

func TestInitModel_CancelDoesNotSave(t *testing.T) {
	tmpDir := t.TempDir()

	projectConfig := &domain.ProjectConfig{
		Description: "Should not be saved",
		Areas:       map[string][]string{},
	}

	m := NewInitModel(80, 24, tmpDir, projectConfig, nil)
	m.setState(stateInitReview)

	// Press esc to cancel
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})

	// Verify config was NOT saved
	configPath := filepath.Join(tmpDir, ".git-courer", "config.json")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file should NOT exist after cancel")
	}

	initM := updatedModel.(InitModel)
	if !initM.done {
		t.Error("Model should be done after cancel")
	}
}

// --- Test: 'n' on confirm state cancels without saving ---

func TestInitModel_ConfirmDeclineDoesNotSave(t *testing.T) {
	tmpDir := t.TempDir()

	projectConfig := &domain.ProjectConfig{
		Description: "Should not be saved either",
		Areas:       map[string][]string{},
	}

	m := NewInitModel(80, 24, tmpDir, projectConfig, nil)
	m.setState(stateInitConfirm)

	// Press 'n' to decline on confirm screen
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	// Verify config was NOT saved
	configPath := filepath.Join(tmpDir, ".git-courer", "config.json")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file should NOT exist after declining confirm")
	}

	initM := updatedModel.(InitModel)
	if !initM.done {
		t.Error("Model should be done after declining confirm")
	}
}

// --- Test: Loading state shows spinner message ---

func TestInitModel_LoadingStateShowsAnalyzing(t *testing.T) {
	// Create model with an available LLM but no config — stays in loading state
	m := NewInitModel(80, 24, ".", nil, &mockLLMClient{available: true})
	// State should be loading since no config was provided and LLM is available

	view := m.View()
	if !strings.Contains(view, "Analyzing") {
		t.Errorf("Loading state should show analyzing message; got:\n%s", view)
	}
}

// --- Test: No LLM, no config → stays in review (manual entry) ---

func TestInitModel_NoLLMNoConfigStartsInReview(t *testing.T) {
	m := NewInitModel(80, 24, ".", nil, &mockLLMClient{available: false})
	if m.state != stateInitReview {
		t.Errorf("State should be stateInitReview when LLM unavailable and no config; got %d", m.state)
	}
}

// --- Test: LLM result transitions to review state ---

func TestInitModel_LLMResultTransitionsToReview(t *testing.T) {
	projectConfig := &domain.ProjectConfig{
		Description: "LLM generated project",
		Areas: map[string][]string{
			"api": {"internal/api/"},
		},
	}

	m := NewInitModel(80, 24, ".", nil, &mockLLMClient{available: true})

	// Simulate receiving LLM result
	updatedModel, _ := m.Update(projectInitResultMsg{config: projectConfig})

	initM := updatedModel.(InitModel)
	if initM.state != stateInitReview {
		t.Errorf("State should be stateInitReview after LLM result; got %d", initM.state)
	}
	if initM.config.Description != "LLM generated project" {
		t.Errorf("Config description should be set from LLM result; got %q", initM.config.Description)
	}
}

// --- Test: LLM error transitions to review with empty form ---

func TestInitModel_LLMErrorTransitionsToReview(t *testing.T) {
	m := NewInitModel(80, 24, ".", nil, &mockLLMClient{available: true})

	// Simulate LLM failure
	updatedModel, _ := m.Update(projectInitErrorMsg{err: os.ErrDeadlineExceeded})

	initM := updatedModel.(InitModel)
	if initM.state != stateInitReview {
		t.Errorf("State should be stateInitReview after LLM error; got %d", initM.state)
	}
	if initM.err == nil {
		t.Error("Error should be set after LLM failure")
	}
}

// --- Test: Description editing in review state ---

func TestInitModel_DescriptionEditing(t *testing.T) {
	projectConfig := &domain.ProjectConfig{
		Description: "Original description",
		Areas:       map[string][]string{},
	}

	m := NewInitModel(80, 24, ".", projectConfig, nil)
	// Model already in review state with description pre-populated
	m.descriptionInput.Focus()

	// Type characters to update description
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("edited")})

	initM := updatedModel.(InitModel)
	if initM.descriptionInput.Value() != "Original descriptionedited" {
		t.Errorf("description input should append typed chars; got %q", initM.descriptionInput.Value())
	}
}

// --- Test: Enter on confirm state saves via handleEnter ---

func TestInitModel_EnterOnConfirmSaves(t *testing.T) {
	tmpDir := t.TempDir()

	projectConfig := &domain.ProjectConfig{
		Description: "Enter confirm test",
		Areas:       map[string][]string{},
	}

	m := NewInitModel(80, 24, tmpDir, projectConfig, nil)
	m.setState(stateInitConfirm)

	// Press enter on confirm screen
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify config was saved
	configPath := filepath.Join(tmpDir, ".git-courer", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file should exist after enter on confirm")
	}

	initM := updatedModel.(InitModel)
	if !initM.done {
		t.Error("Model should be done after enter on confirm")
	}
}

// --- Test: Review to confirm transition syncs inputs ---

func TestInitModel_ReviewEnterMovesToConfirm(t *testing.T) {
	projectConfig := &domain.ProjectConfig{
		Description: "Review test",
		Areas:       map[string][]string{},
	}

	m := NewInitModel(80, 24, ".", projectConfig, nil)
	// Model should be in review state

	// Press enter to move from review to confirm
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	initM := updatedModel.(InitModel)
	if initM.state != stateInitConfirm {
		t.Errorf("State should be stateInitConfirm after enter in review; got %d", initM.state)
	}
	if initM.config.Description != "Review test" {
		t.Errorf("Config should be synced from inputs; got description %q", initM.config.Description)
	}
}

// --- Triangulation: Multiple areas render correctly ---

func TestInitModel_ViewRendersMultipleAreas(t *testing.T) {
	projectConfig := &domain.ProjectConfig{
		Description: "Multi-area project",
		Areas: map[string][]string{
			"alpha": {"src/alpha/"},
			"beta":  {"src/beta/", "pkg/beta/"},
			"gamma": {"src/gamma/"},
		},
	}

	m := NewInitModel(80, 24, ".", projectConfig, nil)
	view := m.View()

	if !strings.Contains(view, "alpha") {
		t.Error("View should contain area 'alpha'")
	}
	if !strings.Contains(view, "beta") {
		t.Error("View should contain area 'beta'")
	}
	if !strings.Contains(view, "gamma") {
		t.Error("View should contain area 'gamma'")
	}
	if !strings.Contains(view, "src/beta/") {
		t.Error("View should contain path 'src/beta/'")
	}
	if !strings.Contains(view, "pkg/beta/") {
		t.Error("View should contain path 'pkg/beta/'")
	}
}

// --- Triangulation: Confirm with areas saves them correctly ---

func TestInitModel_ConfirmSavesWithMultipleAreas(t *testing.T) {
	tmpDir := t.TempDir()

	projectConfig := &domain.ProjectConfig{
		Description: "Multi-area save test",
		Areas: map[string][]string{
			"core": {"internal/core/"},
			"api":  {"internal/api/", "pkg/api/"},
		},
	}

	m := NewInitModel(80, 24, tmpDir, projectConfig, nil)
	m.setState(stateInitConfirm)

	// Press 'y' to confirm
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if len(loaded.Areas) != 2 {
		t.Errorf("Areas count = %d, want 2", len(loaded.Areas))
	}
	if len(loaded.Areas["api"]) != 2 {
		t.Errorf("Areas[api] should have 2 paths; got %v", loaded.Areas["api"])
	}

	initM := updatedModel.(InitModel)
	if !initM.done {
		t.Error("Model should be done after confirm")
	}
}

// --- Triangulation: Config collision from TUI shows error ---

func TestInitModel_SaveFailureSetsError(t *testing.T) {
	// Use a read-only directory to force save failure
	tmpDir := t.TempDir()
	// Create .git-courer dir but make it read-only
	configDir := filepath.Join(tmpDir, ".git-courer")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Make dir read-only after creating it
	if err := os.Chmod(configDir, 0555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(configDir, 0755) // restore for cleanup

	projectConfig := &domain.ProjectConfig{
		Description: "Save failure test",
		Areas:       map[string][]string{},
	}

	m := NewInitModel(80, 24, tmpDir, projectConfig, nil)
	m.setState(stateInitConfirm)

	// Press 'y' to confirm — should fail to save
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	initM := updatedModel.(InitModel)
	if initM.err == nil {
		t.Error("Error should be set when save fails")
	}
	if initM.done {
		t.Error("Model should NOT be done when save fails")
	}
}

// --- Triangulation: Tab navigation cycles through fields ---

func TestInitModel_TabNavigation(t *testing.T) {
	projectConfig := &domain.ProjectConfig{
		Description: "Tab test",
		Areas: map[string][]string{
			"auth": {"internal/auth/"},
		},
	}

	m := NewInitModel(80, 24, ".", projectConfig, nil)
	// Starts with focus on description

	// Tab to area name
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	initM := updatedModel.(InitModel)
	// focusIndex 0=description, 1=area[0].name, 2=area[0].paths, 3=save, 4=cancel
	expectedFocus := focusTarget(1)
	if initM.focusIndex != expectedFocus {
		t.Errorf("After first tab, focusIndex = %d, want %d", initM.focusIndex, expectedFocus)
	}

	// Tab to area paths
	updatedModel, _ = initM.Update(tea.KeyMsg{Type: tea.KeyTab})
	initM = updatedModel.(InitModel)
	if initM.focusIndex != focusTarget(2) {
		t.Errorf("After second tab, focusIndex = %d, want 2", initM.focusIndex)
	}
}

// --- Triangulation: Manual description entry flows through to save ---

func TestInitModel_ManualDescriptionEntrySaves(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewInitModel(80, 24, tmpDir, nil, &mockLLMClient{available: false})
	// Model starts in review mode with empty fields

	// Enter a description directly
	m.descriptionInput.SetValue("Manual entry project")

	// Use handleEnter to properly sync inputs and move to confirm
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	initM := updatedModel.(InitModel)
	if initM.state != stateInitConfirm {
		t.Fatalf("State should be confirm; got %d", initM.state)
	}

	// Confirm with 'y'
	updatedModel, _ = initM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	initM = updatedModel.(InitModel)
	if !initM.done {
		t.Error("Model should be done after confirm")
	}

	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "Manual entry project" {
		t.Errorf("Description = %q, want %q", loaded.Description, "Manual entry project")
	}
}