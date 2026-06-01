// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/installer"
	"github.com/Alejandro-M-P/git-courer/tui/components"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
)

var (
	getMCPClients  = installer.MCPClients
	configureMCP   = installer.ConfigureMCP
	findBinaryPath = installer.FindBinaryPath
)

// MCPSetupScreen represents the MCP client selection and setup model.
type MCPSetupScreen struct {
	width           int
	checkbox        components.CheckboxModel
	step            int // 0=selection, 1=progress, 2=done
	selectedClients []string
	configuredCount int
	err             error
}

// NewMCPSetupScreen creates a new MCP setup screen.
func NewMCPSetupScreen(width int) MCPSetupScreen {
	// Detect all clients and show with detection status
	allClients := getMCPClients()
	var checkboxItems []components.CheckboxItem
	seen := make(map[string]int) // maps Name to index in checkboxItems

	for _, client := range allClients {
		detected := client.Detect()
		if idx, exists := seen[client.Name]; exists {
			// Aggregate detection status: detected if ANY detects
			if detected {
				checkboxItems[idx].Detected = true
				checkboxItems[idx].Selected = true // Default selected if detected
			}
		} else {
			checkboxItems = append(checkboxItems, components.CheckboxItem{
				Name:     client.Name,
				Selected: detected,
				Detected: detected,
			})
			seen[client.Name] = len(checkboxItems) - 1
		}
	}

	return MCPSetupScreen{
		width:    width,
		checkbox: components.NewCheckbox(checkboxItems, width),
		step:     0,
	}
}

// Init initializes the MCP setup screen.
func (m MCPSetupScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages for the MCP setup screen.
func (m *MCPSetupScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "j", "down", "k", "up", " ", "h", "left", "l", "right":
			newCheckbox, cmd := m.checkbox.Update(msg)
			m.checkbox = *(newCheckbox.(*components.CheckboxModel))
			return m, cmd
		}
	}

	return m, nil
}

// handleEnter handles the enter key based on current step.
func (m *MCPSetupScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // Selection - collect selected clients and proceed
		m.selectedClients = []string{}
		for _, item := range m.checkbox.Items() {
			if item.Selected {
				m.selectedClients = append(m.selectedClients, item.Name)
			}
		}
		if len(m.selectedClients) == 0 {
			// No clients selected - skip to done
			m.step = 2
		} else {
			m.step = 1
			m.configureClients()
		}

	case 1: // Progress - should not happen (handled in step 0)
		m.step = 2

	case 2: // Done
		return m, nil
	}
	return m, nil
}

// Done returns true if the MCP setup process is complete.
func (m MCPSetupScreen) Done() bool {
	return m.step == 2
}

// configureClients runs MCP configuration for all selected clients.
func (m *MCPSetupScreen) configureClients() {
	// Find binary path
	binPath, err := findBinaryPath()
	if err != nil {
		binPath = "git-courer" // Fallback to assuming it's in PATH
	}

	// Configure each selected client
	allClients := getMCPClients()
	m.configuredCount = 0

	for _, client := range allClients {
		for _, selectedName := range m.selectedClients {
			if client.Name == selectedName {
				if client.Detect() {
					if err := configureMCP(client, binPath); err != nil {
						m.err = err
						continue
					}
					m.configuredCount++
				}
				break
			}
		}
	}
}

// View renders the MCP setup screen.
func (m MCPSetupScreen) View() string {
	var s strings.Builder

	header := styles.BoxHeaderStyle.Render("MCP CLIENT SETUP") + "\n\n"

	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	var content string
	switch m.step {
	case 0:
		content = m.renderSelection()
	case 1:
		content = m.renderProgress()
	case 2:
		content = m.renderDone()
	}

	s.WriteString(content)

	return styles.BoxStyle.Render(header + s.String())
}

func (m MCPSetupScreen) renderSelection() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render("Select MCP clients to configure:\n\n"))
	s.WriteString("  " + styles.SuccessStyle.Render("✓") + " = detected on system\n")
	s.WriteString("  " + styles.ErrorStyle.Render("✗") + " = not detected\n\n")

	s.WriteString(m.checkbox.View())
	return s.String()
}

func (m MCPSetupScreen) renderProgress() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Configuring MCP clients...\n\n"))

	// Show what was configured (one line per unique selected name)
	for _, selectedName := range m.selectedClients {
		s.WriteString(styles.SuccessStyle.Render("✓ ") + selectedName + "\n")
	}

	return s.String()
}

func (m MCPSetupScreen) renderDone() string {
	var s strings.Builder
	s.WriteString(styles.SuccessStyle.Render("✓ MCP setup complete!\n\n"))

	if m.configuredCount > 0 {
		s.WriteString(fmt.Sprintf("Configured %d MCP client(s) for git-courer.\n", m.configuredCount))
	} else if len(m.selectedClients) > 0 {
		s.WriteString(fmt.Sprintf("Configured %d MCP client(s).\n", len(m.selectedClients)))
	} else {
		s.WriteString("No MCP clients were selected.\n")
	}

	s.WriteString("\n")
	s.WriteString(styles.SubtextStyle.Render("Run 'git-courer mcp' to start the MCP server.\n"))
	return s.String()
}

// Items returns the checkbox items for external access.
func (m MCPSetupScreen) Items() []components.CheckboxItem {
	return m.checkbox.Items()
}

// Cursor returns the current cursor position.
func (m MCPSetupScreen) Cursor() int {
	return m.checkbox.Cursor()
}
