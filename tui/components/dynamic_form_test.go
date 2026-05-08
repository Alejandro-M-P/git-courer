package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Spec Scenario: Create form with text and select fields ---
func TestNewDynamicFormModel_TextAndSelect(t *testing.T) {
	fields := []DynamicField{
		{ID: "name", Name: "Name", Type: DynFieldText, Placeholder: "enter name"},
		{ID: "mode", Name: "Mode", Type: DynFieldSelect, Options: []string{"a", "b"}},
	}
	m := NewDynamicFormModel(fields, 60)

	if len(m.fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(m.fields))
	}
	if m.cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", m.cursor)
	}

 vals := m.Values()
	if vals["name"] != "" {
		t.Errorf("Text field 'name' should start empty; got %q", vals["name"])
	}
	if vals["mode"] != "a" {
		t.Errorf("Select field 'mode' should default to first option; got %q", vals["mode"])
	}
}

// --- Spec Scenario: Navigate between fields ---
func TestDynamicFormModel_NavigateDown(t *testing.T) {
	fields := []DynamicField{
		{ID: "f1", Name: "F1", Type: DynFieldText},
		{ID: "f2", Name: "F2", Type: DynFieldText},
		{ID: "f3", Name: "F3", Type: DynFieldText},
	}
	m := NewDynamicFormModel(fields, 60)
	if m.cursor != 0 {
		t.Fatalf("Start cursor should be 0; got %d", m.cursor)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	dm := updated.(DynamicFormModel)
	if dm.cursor != 1 {
		t.Errorf("After down, cursor should be 1; got %d", dm.cursor)
	}
}

func TestDynamicFormModel_NavigateUp(t *testing.T) {
	fields := []DynamicField{
		{ID: "f1", Name: "F1", Type: DynFieldText},
		{ID: "f2", Name: "F2", Type: DynFieldText},
	}
	m := NewDynamicFormModel(fields, 60)
	m.cursor = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	dm := updated.(DynamicFormModel)
	if dm.cursor != 0 {
		t.Errorf("After up, cursor should be 0; got %d", dm.cursor)
	}
}

// --- Spec Scenario: Submit values after user input ---
func TestDynamicFormModel_TextEntryValues(t *testing.T) {
	fields := []DynamicField{
		{ID: "project", Name: "Project", Type: DynFieldText, Placeholder: "my-app"},
	}
	m := NewDynamicFormModel(fields, 60)

	// Type "my-app" into the focused text input
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my-app")})
	dm := updated.(DynamicFormModel)

	vals := dm.Values()
	if vals["project"] != "my-app" {
		t.Errorf("Expected project='my-app', got %q", vals["project"])
	}
}

// --- Spec Scenario: Empty field list ---
func TestNewDynamicFormModel_EmptyFields(t *testing.T) {
	m := NewDynamicFormModel([]DynamicField{}, 60)
	if len(m.fields) != 0 {
		t.Fatalf("Expected 0 fields, got %d", len(m.fields))
	}

 view := m.View()
	if view != "" {
		t.Errorf("Empty form View should return empty string; got %q", view)
	}

 vals := m.Values()
	if len(vals) != 0 {
		t.Errorf("Empty form Values should return empty map; got %v", vals)
	}
}

// --- Spec Scenario: Toggle field in DynamicForm ---
func TestDynamicFormModel_Toggle(t *testing.T) {
	fields := []DynamicField{
		{ID: "preview", Name: "Preview", Type: DynFieldToggle, Value: "false"},
	}
	m := NewDynamicFormModel(fields, 60)

	// Press space to toggle
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	dm := updated.(DynamicFormModel)

	vals := dm.Values()
	if vals["preview"] != "true" {
		t.Errorf("After toggle, preview should be 'true'; got %q", vals["preview"])
	}

	// Toggle again
	updated2, _ := dm.Update(tea.KeyMsg{Type: tea.KeySpace})
	dm2 := updated2.(DynamicFormModel)
	vals2 := dm2.Values()
	if vals2["preview"] != "false" {
		t.Errorf("After second toggle, preview should be 'false'; got %q", vals2["preview"])
	}
}

// Triangulation: Select field cycles options
func TestDynamicFormModel_SelectCycle(t *testing.T) {
	fields := []DynamicField{
		{ID: "provider", Name: "Provider", Type: DynFieldSelect, Options: []string{"ollama", "openai"}},
	}
	m := NewDynamicFormModel(fields, 60)
	// Default should be first option
	vals := m.Values()
	if vals["provider"] != "ollama" {
		t.Errorf("Default select value should be first option; got %q", vals["provider"])
	}

	// Press space to cycle to next
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	dm := updated.(DynamicFormModel)
	vals = dm.Values()
	if vals["provider"] != "openai" {
		t.Errorf("After cycle, provider should be 'openai'; got %q", vals["provider"])
	}
}

// Triangulation: Stepper field increments/decrements
func TestDynamicFormModel_Stepper(t *testing.T) {
	fields := []DynamicField{
		{ID: "parallel", Name: "Parallel", Type: DynFieldStepper, Value: "2", StepMin: 1, StepMax: 32},
	}
	m := NewDynamicFormModel(fields, 60)
	vals := m.Values()
	if vals["parallel"] != "2" {
		t.Errorf("Initial stepper value should be '2'; got %q", vals["parallel"])
	}

	// Press right to increment
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	dm := updated.(DynamicFormModel)
	vals = dm.Values()
	if vals["parallel"] != "3" {
		t.Errorf("After increment, should be '3'; got %q", vals["parallel"])
	}

	// Press left to decrement
	updated2, _ := dm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	dm2 := updated2.(DynamicFormModel)
	vals = dm2.Values()
	if vals["parallel"] != "2" {
		t.Errorf("After decrement, should be '2'; got %q", vals["parallel"])
	}
}

// Triangulation: Stepper respects min/max bounds
func TestDynamicFormModel_StepperBounds(t *testing.T) {
	fields := []DynamicField{
		{ID: "parallel", Name: "Parallel", Type: DynFieldStepper, Value: "1", StepMin: 1, StepMax: 4},
	}
	m := NewDynamicFormModel(fields, 60)

	// Try to decrement below min
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	dm := updated.(DynamicFormModel)
	vals := dm.Values()
	if vals["parallel"] != "1" {
		t.Errorf("Should not decrement below min; got %q", vals["parallel"])
	}

	// Set to max and try to increment
	dm.fields[0].Value = "4"
	updated2, _ := dm.Update(tea.KeyMsg{Type: tea.KeyRight})
	dm2 := updated2.(DynamicFormModel)
	vals = dm2.Values()
	if vals["parallel"] != "4" {
		t.Errorf("Should not increment above max; got %q", vals["parallel"])
	}
}

// Triangulation: View renders field names
func TestDynamicFormModel_ViewRendersFieldNames(t *testing.T) {
	fields := []DynamicField{
		{ID: "name", Name: "Project Name", Type: DynFieldText},
		{ID: "mode", Name: "Mode", Type: DynFieldSelect, Options: []string{"a", "b"}},
	}
	m := NewDynamicFormModel(fields, 60)
	view := m.View()

	if !strings.Contains(view, "Project Name") {
		t.Errorf("View should contain field name 'Project Name'; got:\n%s", view)
	}
	if !strings.Contains(view, "Mode") {
		t.Errorf("View should contain field name 'Mode'; got:\n%s", view)
	}
}

// Triangulation: Tab navigation works like down
func TestDynamicFormModel_TabNavigation(t *testing.T) {
	fields := []DynamicField{
		{ID: "f1", Name: "F1", Type: DynFieldText},
		{ID: "f2", Name: "F2", Type: DynFieldSelect, Options: []string{"a"}},
		{ID: "f3", Name: "F3", Type: DynFieldToggle, Value: "false"},
	}
	m := NewDynamicFormModel(fields, 60)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	dm := updated.(DynamicFormModel)
	if dm.cursor != 1 {
		t.Errorf("After tab, cursor should be 1; got %d", dm.cursor)
	}
}

// Triangulation: Select with 'l' key cycles forward
func TestDynamicFormModel_SelectCyclesWithL(t *testing.T) {
	fields := []DynamicField{
		{ID: "provider", Name: "Provider", Type: DynFieldSelect, Options: []string{"ollama", "openai", "anthropic"}},
	}
	m := NewDynamicFormModel(fields, 60)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	dm := updated.(DynamicFormModel)
	vals := dm.Values()
	if vals["provider"] != "openai" {
		t.Errorf("After 'l', should cycle to next; got %q", vals["provider"])
	}
}

// Triangulation: Select with 'h' key cycles backward
func TestDynamicFormModel_SelectCyclesBackwardWithH(t *testing.T) {
	fields := []DynamicField{
		{ID: "provider", Name: "Provider", Type: DynFieldSelect, Options: []string{"ollama", "openai", "anthropic"}},
	}
	// Start at second option
	m := NewDynamicFormModel(fields, 60)
	m.fields[0].Value = "openai"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	dm := updated.(DynamicFormModel)
	vals := dm.Values()
	if vals["provider"] != "ollama" {
		t.Errorf("After 'h', should cycle to previous; got %q", vals["provider"])
	}
}