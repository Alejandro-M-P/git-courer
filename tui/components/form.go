// Package components provides reusable TUI components for the git-courer wizard.
package components

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// FieldType represents the type of a form field.
type FieldType int

const (
	FieldText FieldType = iota
	FieldSelect
	FieldStepper
	FieldToggle
)

// FormField represents a single form field.
type FormField struct {
	ID        string
	Name      string
	Type      FieldType
	Value     *string
	Options   []string // for select/stepper
	StepMin   int      // for stepper
	StepMax   int      // for stepper
	StepValue int      // for stepper display
	TextInput textinput.Model
}

// FormModel is a form for editing git-courer configuration.
type FormModel struct {
	fields          []FormField
	cursor          int
	width           int
	cfg             *config.Config
	ResolveContext  func() tea.Cmd // Optional: called on Ctrl+R when context_window is focused
}

// NewFormModel creates a new form model from a config.
func NewFormModel(cfg *config.Config, width int) FormModel {
	if cfg == nil {
		var err error
		cfg, err = config.Load()
		if err != nil {
			cfg = config.Default()
		}
	}

	fields := []FormField{
		{
			ID:    "provider",
			Name:  "Provider",
			Type:  FieldSelect,
			Value: &cfg.LLM.Provider,
			Options: []string{
				"ollama",
				"openai-compatible",
			},
		},
		{
			ID:      "model",
			Name:    "Model",
			Type:    FieldSelect,
			Value:   &cfg.LLM.Model,
			Options: fetchModelOptions(cfg),
		},
		{
			ID:    "url",
			Name:  "Base URL",
			Type:  FieldText,
			Value: &cfg.LLM.BaseURL,
		},
		{
			ID:        "parallel",
			Name:      "Parallel",
			Type:      FieldStepper,
			Value:     nil,
			StepMin:   1,
			StepMax:   32,
			StepValue: cfg.LLM.NumParallel,
		},
		{
			ID:      "release_type",
			Name:    "Release Type",
			Type:    FieldSelect,
			Value:   &cfg.Release.Type,
			Options: []string{"tag", "github"},
		},
		{
			ID:    "context_window",
			Name:  "Context Window",
			Type:  FieldText,
			Value: &cfg.LLM.ContextWindowStr,
		},
	}

	for i := range fields {
		if fields[i].Type == FieldText {
			t := textinput.New()
			if fields[i].ID == "context_window" {
				t.Placeholder = "auto-detect..."
			} else {
				t.Placeholder = "Empty"
			}
			if fields[i].Value != nil {
				t.SetValue(*fields[i].Value)
			}
			t.Width = 30
			fields[i].TextInput = t
		}
	}

	return FormModel{
		fields: fields,
		cursor: 0,
		width:  width,
		cfg:    cfg,
	}
}

// Init initializes the form.
func (m *FormModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the form component.
func (m *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "tab":
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % len(m.fields)
			m.focusCurrent()
		case "up", "shift+tab":
			m.blurCurrent()
			m.cursor = (m.cursor - 1)
			if m.cursor < 0 {
				m.cursor = len(m.fields) - 1
			}
			m.focusCurrent()
		case "h", "left":
			if m.fields[m.cursor].Type != FieldText {
				m.handleStep(-1)
				m.cycleOption(-1)
			}
		case "l", "right":
			if m.fields[m.cursor].Type != FieldText {
				m.handleStep(1)
				m.cycleOption(1)
			}
		case " ":
			if m.fields[m.cursor].Type == FieldToggle {
				m.toggleCurrent()
			} else if m.fields[m.cursor].Type == FieldSelect {
				m.cycleOption(1)
			}
		case "enter":
			if m.fields[m.cursor].Type == FieldToggle {
				m.toggleCurrent()
			} else if m.fields[m.cursor].Type == FieldSelect {
				m.cycleOption(1)
			} else if m.fields[m.cursor].Type == FieldText {
				m.blurCurrent()
				m.cursor = (m.cursor + 1) % len(m.fields)
				m.focusCurrent()
			}
			_ = m.cfg.SaveGlobal()
		case "ctrl+r":
			// Resolve context window when context_window field is focused
			if m.fields[m.cursor].ID == "context_window" && m.ResolveContext != nil {
				return m, m.ResolveContext()
			}
		}
	}

	for i := range m.fields {
		if m.fields[i].Type == FieldText {
			var cmd tea.Cmd
			m.fields[i].TextInput, cmd = m.fields[i].TextInput.Update(msg)
			cmds = append(cmds, cmd)
			if m.fields[i].Value != nil {
				*m.fields[i].Value = m.fields[i].TextInput.Value()
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *FormModel) toggleCurrent() {
	// No toggles remain in the form
}

func (m *FormModel) blurCurrent() {
	if m.fields[m.cursor].Type == FieldText {
		m.fields[m.cursor].TextInput.Blur()
	}
}

func (m *FormModel) focusCurrent() {
	if m.fields[m.cursor].Type == FieldText {
		m.fields[m.cursor].TextInput.Focus()
	}
}

func (m *FormModel) handleStep(delta int) {
	f := &m.fields[m.cursor]
	if f.Type != FieldStepper {
		return
	}
	newVal := f.StepValue + delta
	if newVal >= f.StepMin && newVal <= f.StepMax {
		f.StepValue = newVal
		m.cfg.LLM.NumParallel = newVal
	}
}

func (m *FormModel) cycleOption(delta int) {
	f := &m.fields[m.cursor]
	if f.Type != FieldSelect || len(f.Options) == 0 {
		return
	}

	curIdx := 0
	for i, opt := range f.Options {
		if *f.Value == opt {
			curIdx = i
			break
		}
	}

	newIdx := (curIdx + delta) % len(f.Options)
	if newIdx < 0 {
		newIdx = len(f.Options) - 1
	}
	*f.Value = f.Options[newIdx]
}

// fetchModelOptions returns available models from the LLM endpoint,
// with the currently configured model first.
func fetchModelOptions(cfg *config.Config) []string {
	if cfg.LLM.BaseURL == "" {
		return []string{cfg.LLM.Model}
	}
	models, err := openai_standard.ListModels(cfg.LLM.BaseURL)
	if err != nil || len(models) == 0 {
		return []string{cfg.LLM.Model}
	}
	// Put current model first, then the rest
	result := []string{cfg.LLM.Model}
	for _, m := range models {
		if m != cfg.LLM.Model {
			result = append(result, m)
		}
	}
	return result
}

// View renders the form.
func (m FormModel) View() string {
	var s strings.Builder

	for i, f := range m.fields {
		cursor := "  "
		if i == m.cursor {
			cursor = styles.Cursor.Render()
		}

		nameStyle := lipgloss.NewStyle()
		if i == m.cursor {
			nameStyle = nameStyle.Foreground(styles.Cyan).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(styles.Gray)
		}

		valueStr := m.getValueString(i, i == m.cursor)
		line := cursor + nameStyle.Render(f.Name) + ": " + valueStr + "\n"
		s.WriteString(line)
	}

	return s.String()
}

func (m FormModel) getValueString(idx int, isFocused bool) string {
	f := m.fields[idx]
	switch f.Type {
	case FieldText:
		return f.TextInput.View()
	case FieldSelect:
		if f.Value == nil {
			return ""
		}
		val := *f.Value
		if isFocused {
			arrowStyle := lipgloss.NewStyle().Foreground(styles.Gray)
			valueStyle := lipgloss.NewStyle().Foreground(styles.White)
			return fmt.Sprintf("[%s %s %s]", arrowStyle.Render("◀"), valueStyle.Render(val), arrowStyle.Render("▶"))
		}
		return fmt.Sprintf("[%s]", val)
	case FieldStepper:
		return fmt.Sprintf("%d", f.StepValue)
	default:
		return ""
	}
}

func (m FormModel) Cursor() int {
	return m.cursor
}

func (m FormModel) Config() *config.Config {
	return m.cfg
}
