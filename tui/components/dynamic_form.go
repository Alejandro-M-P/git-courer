package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blak0p/git-courer/tui/styles"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DynFieldType represents the type of a dynamic form field.
type DynFieldType int

const (
	DynFieldText DynFieldType = iota
	DynFieldSelect
	DynFieldStepper
	DynFieldToggle
)

// DynamicField defines a single editable field in a DynamicFormModel.
// It has NO dependency on config.Config — value types only.
type DynamicField struct {
	ID          string
	Name        string
	Type        DynFieldType
	Value       string   // copied value — not a pointer to external state
	Options     []string // for select/stepper
	StepMin     int      // for stepper
	StepMax     int      // for stepper
	Placeholder string

	textInput textinput.Model // internal, only for DynFieldText
}

// DynamicFormModel is a generic form model that accepts arbitrary field definitions.
// It implements tea.Model and has NO dependency on config or domain packages.
type DynamicFormModel struct {
	fields []DynamicField
	cursor int
	width  int
}

// NewDynamicFormModel creates a new DynamicFormModel with the given fields and width.
func NewDynamicFormModel(fields []DynamicField, width int) DynamicFormModel {
	// Initialize text inputs for text fields
	for i := range fields {
		if fields[i].Type == DynFieldText {
			ti := textinput.New()
			ti.Placeholder = fields[i].Placeholder
			ti.CharLimit = 200
			ti.Width = width / 3
			if fields[i].Value != "" {
				ti.SetValue(fields[i].Value)
			}
			fields[i].textInput = ti
		}
		// Select fields default to first option if Value is empty
		if fields[i].Type == DynFieldSelect && fields[i].Value == "" && len(fields[i].Options) > 0 {
			fields[i].Value = fields[i].Options[0]
		}
	}

	m := DynamicFormModel{
		fields: fields,
		cursor: 0,
		width:  width,
	}

	// Focus first text field if any
	if len(fields) > 0 && fields[0].Type == DynFieldText {
		m.fields[0].textInput.Focus()
	}

	return m
}

// Init implements tea.Model.
func (m DynamicFormModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m DynamicFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.fields) == 0 {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "tab":
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % len(m.fields)
			m.focusCurrent()
		case "up", "shift+tab":
			m.blurCurrent()
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.fields) - 1
			}
			m.focusCurrent()
		case " ":
			f := &m.fields[m.cursor]
			if f.Type == DynFieldToggle {
				m.toggleCurrent()
			} else if f.Type == DynFieldSelect {
				m.cycleOption(1)
			}
		case "l", "right":
			f := &m.fields[m.cursor]
			if f.Type == DynFieldSelect {
				m.cycleOption(1)
			} else if f.Type == DynFieldStepper {
				m.stepCurrent(1)
			}
		case "h", "left":
			f := &m.fields[m.cursor]
			if f.Type == DynFieldSelect {
				m.cycleOption(-1)
			} else if f.Type == DynFieldStepper {
				m.stepCurrent(-1)
			}
		case "enter":
			f := &m.fields[m.cursor]
			if f.Type == DynFieldToggle {
				m.toggleCurrent()
			} else if f.Type == DynFieldSelect {
				m.cycleOption(1)
			} else if f.Type == DynFieldText {
				m.blurCurrent()
				m.cursor = (m.cursor + 1) % len(m.fields)
				m.focusCurrent()
			}
		}
	}

	// Forward messages to focused text input
	if len(m.fields) > 0 && m.fields[m.cursor].Type == DynFieldText {
		var cmd tea.Cmd
		m.fields[m.cursor].textInput, cmd = m.fields[m.cursor].textInput.Update(msg)
		// Sync value back
		m.fields[m.cursor].Value = m.fields[m.cursor].textInput.Value()
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m DynamicFormModel) View() string {
	if len(m.fields) == 0 {
		return ""
	}

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

// Values returns a map of field IDs to their current string values.
func (m DynamicFormModel) Values() map[string]string {
	result := make(map[string]string, len(m.fields))
	for _, f := range m.fields {
		result[f.ID] = f.Value
	}
	return result
}

func (m *DynamicFormModel) blurCurrent() {
	if len(m.fields) == 0 {
		return
	}
	if m.fields[m.cursor].Type == DynFieldText {
		m.fields[m.cursor].textInput.Blur()
	}
}

func (m *DynamicFormModel) focusCurrent() {
	if len(m.fields) == 0 {
		return
	}
	if m.fields[m.cursor].Type == DynFieldText {
		m.fields[m.cursor].textInput.Focus()
	}
}

func (m *DynamicFormModel) toggleCurrent() {
	f := &m.fields[m.cursor]
	if f.Value == "true" {
		f.Value = "false"
	} else {
		f.Value = "true"
	}
}

func (m *DynamicFormModel) cycleOption(delta int) {
	f := &m.fields[m.cursor]
	if f.Type != DynFieldSelect || len(f.Options) == 0 {
		return
	}

	curIdx := 0
	for i, opt := range f.Options {
		if f.Value == opt {
			curIdx = i
			break
		}
	}

	newIdx := (curIdx + delta) % len(f.Options)
	if newIdx < 0 {
		newIdx = len(f.Options) - 1
	}
	f.Value = f.Options[newIdx]
}

func (m *DynamicFormModel) stepCurrent(delta int) {
	f := &m.fields[m.cursor]
	if f.Type != DynFieldStepper {
		return
	}

	val, err := strconv.Atoi(f.Value)
	if err != nil {
		val = f.StepMin
	}

	newVal := val + delta
	if newVal < f.StepMin {
		newVal = f.StepMin
	}
	if newVal > f.StepMax {
		newVal = f.StepMax
	}
	f.Value = strconv.Itoa(newVal)
}

func (m DynamicFormModel) getValueString(idx int, isFocused bool) string {
	f := m.fields[idx]
	switch f.Type {
	case DynFieldText:
		return f.textInput.View()
	case DynFieldSelect:
		if isFocused {
			arrowStyle := lipgloss.NewStyle().Foreground(styles.Gray)
			valueStyle := lipgloss.NewStyle().Foreground(styles.White)
			return fmt.Sprintf("[%s %s %s]", arrowStyle.Render("◀"), valueStyle.Render(f.Value), arrowStyle.Render("▶"))
		}
		return fmt.Sprintf("[%s]", f.Value)
	case DynFieldStepper:
		if isFocused {
			arrowStyle := lipgloss.NewStyle().Foreground(styles.Gray)
			valueStyle := lipgloss.NewStyle().Foreground(styles.White)
			return fmt.Sprintf("%s %s %s", arrowStyle.Render("◀"), valueStyle.Render(f.Value), arrowStyle.Render("▶"))
		}
		return f.Value
	case DynFieldToggle:
		if f.Value == "true" {
			return styles.SuccessStyle.Render("true")
		}
		return styles.ErrorStyle.Render("false")
	default:
		return ""
	}
}
