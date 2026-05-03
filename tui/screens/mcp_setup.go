// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/installer"
	"github.com/Alejandro-M-P/git-courer/tui/components"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
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
	allClients := installer.MCPClients()
	checkboxItems := make([]components.CheckboxItem, len(allClients))

	for i, client := range allClients {
		detected := client.Detect()
		checkboxItems[i] = components.CheckboxItem{
			Name:     client.Name,
			Selected: detected, // Default selected if detected
			Detected: detected,
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
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "j", "down", "k", "up", " ", "h", "left", "l", "right":
			m.checkbox.Update(msg)
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
		return m, tea.Quit
	}
	return m, nil
}

// configureClients runs MCP configuration for all selected clients.
func (m *MCPSetupScreen) configureClients() {
	// Find binary path
	binPath, err := installer.FindBinaryPath()
	if err != nil {
		binPath = "git-courer" // Fallback to assuming it's in PATH
	}

	// Configure each selected client
	allClients := installer.MCPClients()
	m.configuredCount = 0

	for _, client := range allClients {
		for _, selectedName := range m.selectedClients {
			if client.Name == selectedName {
				if err := installer.ConfigureMCP(client, binPath); err != nil {
					m.err = err
					continue
				}
				m.configuredCount++
				break
			}
		}
	}
}

// View renders the MCP setup screen.
func (m MCPSetupScreen) View() string {
	var s strings.Builder

	s.WriteString(styles.TitleStyle.Render("MCP Client Setup") + "\n\n")

	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	switch m.step {
	case 0:
		s.WriteString(m.renderSelection())
	case 1:
		s.WriteString(m.renderProgress())
	case 2:
		s.WriteString(m.renderDone())
	}

	s.WriteString("\n" + m.renderHelp())
	return s.String()
}

func (m MCPSetupScreen) renderSelection() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render("Select MCP clients to configure for git-courer:\n\n"))
	s.WriteString("  " + styles.SuccessStyle.Render("✓") + " = detected on system\n")
	s.WriteString("  " + styles.WarningStyle.Render("✗") + " = not detected\n\n")

	s.WriteString(m.checkbox.View())
	return s.String()
}

func (m MCPSetupScreen) renderProgress() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Configuring MCP clients...\n\n"))

	// Show what was configured
	allClients := installer.MCPClients()
	for _, client := range allClients {
		for _, selectedName := range m.selectedClients {
			if client.Name == selectedName {
				s.WriteString(styles.SuccessStyle.Render("✓ ") + client.Name + "\n")
				break
			}
		}
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

func (m MCPSetupScreen) renderHelp() string {
	switch m.step {
	case 0:
		return styles.HelpStyle.Render("j/k: navigate  space: toggle  enter: configure  ctrl+c: quit")
	case 1:
		return styles.HelpStyle.Render("")
	case 2:
		return styles.HelpStyle.Render("press enter to exit")
	default:
		return styles.HelpStyle.Render("ctrl+c: quit")
	}
}

// Items returns the checkbox items for external access.
func (m MCPSetupScreen) Items() []components.CheckboxItem {
	return m.checkbox.Items()
}

// SelectedClients returns the list of selected client names.
func (m MCPSetupScreen) SelectedClients() []string {
	return m.selectedClients
}

// ConfiguredCount returns the number of successfully configured clients.
func (m MCPSetupScreen) ConfiguredCount() int {
	return m.configuredCount
}

// HasSelection returns true if any clients are selected.
func (m MCPSetupScreen) HasSelection() bool {
	for _, item := range m.checkbox.Items() {
		if item.Selected {
			return true
		}
	}
	return false
}

// EnsureFileExists creates the directory structure for a config file if needed.
func EnsureFileExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}