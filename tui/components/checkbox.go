// Package components provides reusable TUI components for the git-courer wizard.
package components

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CheckboxItem represents a single checkbox item.
type CheckboxItem struct {
	Name     string
	Selected bool
	Detected bool // true if item exists on system, false if not
}

// CheckboxModel is a navigable checkbox list.
type CheckboxModel struct {
	items   []CheckboxItem
	cursor  int
	width   int
	started bool
}

// NewCheckbox creates a new CheckboxModel with the given items.
func NewCheckbox(items []CheckboxItem, width int) CheckboxModel {
	return CheckboxModel{
		items:   items,
		cursor:  0,
		width:   width,
		started: false,
	}
}

// Start marks the checkbox as started (first navigation).
func (m *CheckboxModel) Start() {
	m.started = true
}

// Init initializes the checkbox component (no-op).
func (m CheckboxModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the checkbox component.
func (m *CheckboxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.started {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case " ":
			// Toggle selection
			m.items[m.cursor].Selected = !m.items[m.cursor].Selected
		case "enter":
			// Confirm — nothing to do here, parent handles it
		case "esc":
			// Back — parent should handle this
		}
	}
	return m, nil
}

// View renders the checkbox list.
func (m CheckboxModel) View() string {
	var s string
	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor && m.started {
			cursor = "▸ "
		}

		// Selection indicator
		selected := " "
		if item.Selected {
			selected = "✓"
		}

		// Detection indicator
		detected := "✗"
		if item.Detected {
			detected = "✓"
		}

		// Style based on cursor position
		nameStyle := lipgloss.NewStyle()
		if i == m.cursor && m.started {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#00D4FF")).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#888888"))
		}

		line := cursor + "[" + selected + "] " + detected + "  " + nameStyle.Render(item.Name) + "\n"
		s += line
	}
	return s
}

// Cursor returns the current cursor position.
func (m CheckboxModel) Cursor() int {
	return m.cursor
}

// Items returns the current items.
func (m CheckboxModel) Items() []CheckboxItem {
	return m.items
}

// SetItems updates the items.
func (m *CheckboxModel) SetItems(items []CheckboxItem) {
	m.items = items
	if m.cursor >= len(m.items) {
		m.cursor = 0
	}
}