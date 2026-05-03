// Package components provides reusable TUI components for the git-courer wizard.
package components

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FieldType represents the type of a form field.
type FieldType int

const (
	FieldText FieldType = iota
	FieldSelect
	FieldStepper
)

// FormField represents a single form field.
type FormField struct {
	Name      string
	Type      FieldType
	Value     *string
	Options   []string // for select/stepper
	StepMin   int      // for stepper
	StepMax   int      // for stepper
	StepValue int      // for stepper display
}

// FormModel is a form for editing git-courer configuration.
type FormModel struct {
	fields   []FormField
	cursor   int
	width    int
	started  bool
	cfg      *config.Config
}

// NewFormModel creates a new form model from a config.
func NewFormModel(cfg *config.Config, width int) FormModel {
	fields := []FormField{
		{
			Name:  "LLM Provider",
			Type:  FieldSelect,
			Value: &cfg.LLM.Provider,
			Options: []string{
				"ollama",
				"openai-compatible",
				"lmstudio",
				"vllm",
				"localai",
			},
		},
		{
			Name:  "Model",
			Type:  FieldText,
			Value: &cfg.LLM.Model,
		},
		{
			Name:  "Base URL",
			Type:  FieldText,
			Value: &cfg.LLM.BaseURL,
		},
		{
			Name:      "Parallel Requests",
			Type:      FieldStepper,
			Value:     nil, // special handling
			StepMin:   1,
			StepMax:   32,
			StepValue: cfg.LLM.NumParallel,
		},
		{
			Name:  "Preview Enabled",
			Type:  FieldSelect,
			Value: boolToString(cfg.Preview.Enabled),
			Options: []string{
				"true",
				"false",
			},
		},
		{
			Name:  "Git WorkDir",
			Type:  FieldText,
			Value: &cfg.Git.WorkDir,
		},
		{
			Name:  "Context Project",
			Type:  FieldText,
			Value: &cfg.Context.Project,
		},
		{
			Name:  "Context Style",
			Type:  FieldSelect,
			Value: &cfg.Context.Style,
			Options: []string{
				"concise_technical",
				"detailed_technical",
				"casual",
			},
		},
	}

	return FormModel{
		fields:  fields,
		cursor: 0,
		width:  width,
		started: false,
		cfg:     cfg,
	}
}

// boolToString converts a bool to string pointer.
func boolToString(b bool) *string {
	s := fmt.Sprintf("%t", b)
	return &s
}

// Start marks the form as started.
func (m *FormModel) Start() {
	m.started = true
}

// Init initializes the form (no-op).
func (m FormModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the form component.
func (m *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.started {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.fields)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "h", "left":
			m.handleStep(-1)
		case "l", "right":
			m.handleStep(1)
		case " ":
			// For select fields, cycle to next option
			m.cycleOption(1)
		case "enter":
			// For select fields, cycle to next option
			m.cycleOption(1)
		case "esc":
			// Back — parent should handle
		}
	}
	return m, nil
}

// handleStep handles stepper field increments.
func (m *FormModel) handleStep(delta int) {
	f := &m.fields[m.cursor]
	if f.Type != FieldStepper {
		return
	}
	newVal := f.StepValue + delta
	if newVal >= f.StepMin && newVal <= f.StepMax {
		f.StepValue = newVal
		// Also update the config directly
		m.cfg.LLM.NumParallel = newVal
	}
}

// cycleOption cycles through options for select fields.
func (m *FormModel) cycleOption(delta int) {
	f := &m.fields[m.cursor]
	if f.Type != FieldSelect || len(f.Options) == 0 {
		return
	}

	// Find current index
	curIdx := 0
	for i, opt := range f.Options {
		if *f.Value == opt {
			curIdx = i
			break
		}
	}

	// Move to next/prev
	newIdx := (curIdx + delta) % len(f.Options)
	if newIdx < 0 {
		newIdx = len(f.Options) - 1
	}
	*f.Value = f.Options[newIdx]
}

// View renders the form.
func (m FormModel) View() string {
	var s strings.Builder

	for i, f := range m.fields {
		cursor := "  "
		if i == m.cursor && m.started {
			cursor = "▸ "
		}

		// Field name
		nameStyle := lipgloss.NewStyle()
		if i == m.cursor && m.started {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#00D4FF")).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#888888"))
		}

		valueStr := m.getValueString(f)
		whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		line := cursor + nameStyle.Render(f.Name) + ": " + whiteStyle.Render(valueStr) + "\n"
		s.WriteString(line)
	}

	return s.String()
}

// getValueString returns the string representation of a field's value.
func (m FormModel) getValueString(f FormField) string {
	switch f.Type {
	case FieldText:
		if f.Value == nil {
			return ""
		}
		return *f.Value
	case FieldSelect:
		if f.Value == nil {
			return ""
		}
		return *f.Value
	case FieldStepper:
		return fmt.Sprintf("%d", f.StepValue)
	default:
		return ""
	}
}

// Cursor returns the current cursor position.
func (m FormModel) Cursor() int {
	return m.cursor
}

// Config returns the updated config.
func (m FormModel) Config() *config.Config {
	return m.cfg
}