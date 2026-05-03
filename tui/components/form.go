// Package components provides reusable TUI components for the git-courer wizard.
package components

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// modelsFetchedMsg is sent when models are successfully retrieved from a provider.
type modelsFetchedMsg struct {
	provider string
	models   []string
}

// fetchErrorMsg is sent when fetching models fails.
type fetchErrorMsg struct {
	err error
}

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
	Loading   bool
}

// FormModel is a form for editing git-courer configuration.
type FormModel struct {
	fields []FormField
	cursor int
	width  int
	cfg    *config.Config
}

// NewFormModel creates a new form model from a config.
func NewFormModel(cfg *config.Config, width int) FormModel {
	fields := []FormField{
		{
			ID:    "provider",
			Name:  "Provider",
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
			ID:    "model",
			Name:  "Model",
			Type:  FieldSelect, // Changed to Select for dynamic list
			Value: &cfg.LLM.Model,
			Options: []string{cfg.LLM.Model}, // Initial value as only option
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
			Value:     nil, // special handling
			StepMin:   1,
			StepMax:   32,
			StepValue: cfg.LLM.NumParallel,
		},
		{
			ID:    "preview",
			Name:  "Preview",
			Type:  FieldToggle,
			Value: nil, // special handling
		},
		{
			ID:    "workdir",
			Name:  "Git WorkDir",
			Type:  FieldText,
			Value: &cfg.Git.WorkDir,
		},
		{
			ID:    "project",
			Name:  "Context Project",
			Type:  FieldText,
			Value: &cfg.Context.Project,
		},
		{
			ID:    "style",
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

	for i := range fields {
		if fields[i].Type == FieldText {
			t := textinput.New()
			t.Placeholder = "Empty"
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
	return m.FetchModelsCmd()
}

// Update handles messages for the form component.
func (m *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case modelsFetchedMsg:
		for i := range m.fields {
			if m.fields[i].ID == "model" {
				m.fields[i].Options = msg.models
				m.fields[i].Loading = false
				// If current value is not in options, but options exist, select first
				found := false
				for _, opt := range msg.models {
					if *m.fields[i].Value == opt {
						found = true
						break
					}
				}
				if !found && len(msg.models) > 0 {
					*m.fields[i].Value = msg.models[0]
				}
			}
		}
		return m, nil

	case fetchErrorMsg:
		for i := range m.fields {
			if m.fields[i].ID == "model" {
				m.fields[i].Loading = false
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down", "tab":
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % len(m.fields)
			m.focusCurrent()
			if m.fields[m.cursor].ID == "model" && len(m.fields[m.cursor].Options) <= 1 {
				cmds = append(cmds, m.FetchModelsCmd())
			}
		case "k", "up", "shift+tab":
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
				if m.fields[m.cursor].ID == "provider" {
					cmds = append(cmds, m.FetchModelsCmd())
				}
			}
		case "l", "right":
			if m.fields[m.cursor].Type != FieldText {
				m.handleStep(1)
				m.cycleOption(1)
				if m.fields[m.cursor].ID == "provider" {
					cmds = append(cmds, m.FetchModelsCmd())
				}
			}
		case " ":
			if m.fields[m.cursor].Type == FieldToggle {
				m.toggleCurrent()
			} else if m.fields[m.cursor].Type == FieldSelect {
				m.cycleOption(1)
				if m.fields[m.cursor].ID == "provider" {
					cmds = append(cmds, m.FetchModelsCmd())
				}
			}
		case "enter":
			if m.fields[m.cursor].Type == FieldToggle {
				m.toggleCurrent()
			} else if m.fields[m.cursor].Type == FieldSelect {
				m.cycleOption(1)
				if m.fields[m.cursor].ID == "provider" {
					cmds = append(cmds, m.FetchModelsCmd())
				}
			} else if m.fields[m.cursor].Type == FieldText {
				m.blurCurrent()
				m.cursor = (m.cursor + 1) % len(m.fields)
				m.focusCurrent()
			}
		}
	}

	// Update text inputs
	for i := range m.fields {
		if m.fields[i].Type == FieldText {
			var cmd tea.Cmd
			m.fields[i].TextInput, cmd = m.fields[i].TextInput.Update(msg)
			cmds = append(cmds, cmd)
			if m.fields[i].Value != nil {
				oldVal := *m.fields[i].Value
				newVal := m.fields[i].TextInput.Value()
				if oldVal != newVal {
					*m.fields[i].Value = newVal
					if m.fields[i].ID == "url" {
						cmds = append(cmds, m.FetchModelsCmd())
					}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *FormModel) FetchModelsCmd() tea.Cmd {
	provider := m.cfg.LLM.Provider
	url := m.cfg.LLM.BaseURL

	// Only fetch for local providers that we know how to query
	if provider != "ollama" && provider != "vllm" && provider != "localai" && provider != "lmstudio" {
		return nil
	}

	for i := range m.fields {
		if m.fields[i].ID == "model" {
			m.fields[i].Loading = true
		}
	}

	return func() tea.Msg {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url + "/v1/models")
		if err != nil {
			return fetchErrorMsg{err: err}
		}
		defer resp.Body.Close()

		var data struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return fetchErrorMsg{err: err}
		}

		var models []string
		for _, m := range data.Data {
			models = append(models, m.ID)
		}

		if len(models) == 0 {
			return fetchErrorMsg{err: fmt.Errorf("no models found")}
		}

		return modelsFetchedMsg{provider: provider, models: models}
	}
}

func (m *FormModel) toggleCurrent() {
	f := &m.fields[m.cursor]
	if f.ID == "preview" {
		m.cfg.Preview.Enabled = !m.cfg.Preview.Enabled
	}
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

// handleStep handles stepper field increments.
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

// cycleOption cycles through options for select fields.
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

// View renders the form.
func (m FormModel) View() string {
	var s strings.Builder

	for i, f := range m.fields {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}

		nameStyle := lipgloss.NewStyle()
		if i == m.cursor {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#00D4FF")).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#888888"))
		}

		valueStr := m.getValueString(i)
		whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		
		line := cursor + nameStyle.Render(f.Name) + ": " + whiteStyle.Render(valueStr) + "\n"
		s.WriteString(line)
	}

	return s.String()
}

// getValueString returns the string representation of a field's value.
func (m FormModel) getValueString(idx int) string {
	f := m.fields[idx]
	if f.Loading {
		return styles.HelpStyle.Render("Loading...")
	}
	switch f.Type {
	case FieldText:
		return f.TextInput.View()
	case FieldSelect:
		if f.Value == nil {
			return ""
		}
		return *f.Value
	case FieldStepper:
		return fmt.Sprintf("%d", f.StepValue)
	case FieldToggle:
		val := false
		if f.ID == "preview" {
			val = m.cfg.Preview.Enabled
		}
		if val {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF94")).Render("true")
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4C4C")).Render("false")
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