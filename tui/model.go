// Package tui provides the git-courer interactive TUI.
package tui

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/installer"
	"github.com/Alejandro-M-P/git-courer/tui/components"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
)

// AppState represents the current state of the TUI.
type AppState int

const (
	stateInstall AppState = iota
	stateUninstall
	stateMCPSetup
	stateConfigEdit
	stateDone
)

// String returns a human-readable name for the state.
func (s AppState) String() string {
	switch s {
	case stateInstall:
		return "Install"
	case stateUninstall:
		return "Uninstall"
	case stateMCPSetup:
		return "MCP Setup"
	case stateConfigEdit:
		return "Config"
	case stateDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// AppModel is the main TUI model.
type AppModel struct {
	state          AppState
	checkbox       components.CheckboxModel
	form           components.FormModel
	width          int
	height         int
	started        bool
	hasConfig      bool
	err            error
	selectedClients []string
}

// NewAppModel creates a new app model.
func NewAppModel(width, height int) AppModel {
	cfg := config.Default()

	// Check if global config exists
	if _, err := config.Load(); err == nil {
		cfg, _ = config.Load()
	}

	// Initialize checkbox with MCP clients
	clients := installer.DetectClients()
	checkboxItems := make([]components.CheckboxItem, len(clients))
	for i, client := range clients {
		checkboxItems[i] = components.CheckboxItem{
			Name:     client.Name,
			Selected: true,  // default to selected
			Detected: true,  // if in list, it's detected
		}
	}
	if len(checkboxItems) == 0 {
		checkboxItems = []components.CheckboxItem{
			{Name: "No MCP clients detected", Selected: false, Detected: false},
		}
	}

	checkbox := components.NewCheckbox(checkboxItems, width)
	form := components.NewFormModel(cfg, width)

	return AppModel{
		state:          stateInstall,
		checkbox:       checkbox,
		form:           form,
		width:          width,
		height:         height,
		started:        false,
		hasConfig:      false,
		selectedClients: []string{},
	}
}

// Init initializes the TUI.
func (m AppModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the app.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.started {
		m.checkbox.Start()
		m.form.Start()
		m.started = true
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "j", "down", "k", "up", "h", "left", "l", "right", " ":
			return m.handleNavigation(msg.String())
		}
	}

	return m, nil
}

// handleEnter handles the enter key based on current state.
func (m AppModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case stateInstall:
		// Move to uninstall (if clients detected) or config edit
		if len(m.checkbox.Items()) > 0 && m.checkbox.Items()[0].Name != "No MCP clients detected" {
			m.state = stateUninstall
		} else {
			m.state = stateConfigEdit
		}

	case stateUninstall:
		// Collect selected clients
		for _, item := range m.checkbox.Items() {
			if item.Selected && item.Name != "No MCP clients detected" {
				m.selectedClients = append(m.selectedClients, item.Name)
			}
		}
		m.state = stateConfigEdit

	case stateConfigEdit:
		m.state = stateMCPSetup

	case stateMCPSetup:
		// Save config and finish
		if err := m.form.Config().SaveGlobal(); err != nil {
			m.err = err
		}
		m.state = stateDone
	}

	return m, nil
}

// handleNavigation handles navigation keys.
func (m AppModel) handleNavigation(key string) (tea.Model, tea.Cmd) {
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch m.state {
	case stateInstall, stateUninstall:
		m.checkbox.Update(keyMsg)

	case stateConfigEdit, stateMCPSetup:
		m.form.Update(keyMsg)
	}

	return m, nil
}

// View renders the TUI.
func (m AppModel) View() string {
	var s strings.Builder

	// Header
	s.WriteString(styles.TitleStyle.Render("git-courer TUI Installer") + "\n")
	s.WriteString(styles.SubtextStyle.Render("Interactive setup wizard\n\n"))

	// Error if any
	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	// State-specific content
	switch m.state {
	case stateInstall:
		s.WriteString(m.renderInstallScreen())
	case stateUninstall:
		s.WriteString(m.renderUninstallScreen())
	case stateConfigEdit:
		s.WriteString(m.renderConfigScreen())
	case stateMCPSetup:
		s.WriteString(m.renderMCPSetupScreen())
	case stateDone:
		s.WriteString(m.renderDoneScreen())
	}

	// Footer
	s.WriteString("\n" + m.renderHelp())

	return s.String()
}

// renderInstallScreen shows the welcome/install screen.
func (m AppModel) renderInstallScreen() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render("Welcome to git-courer!\n\n"))
	s.WriteString("This wizard will help you set up git-courer on your system.\n\n")
	s.WriteString(styles.SelectedStyle.Render("Press ENTER to start →\n"))
	return s.String()
}

// renderUninstallScreen shows the MCP client uninstall selection.
func (m AppModel) renderUninstallScreen() string {
	var s strings.Builder
	s.WriteString(styles.TitleStyle.Render("Uninstall MCP Clients\n\n"))
	s.WriteString("Select clients to uninstall git-courer from:\n\n")
	s.WriteString(m.checkbox.View())
	s.WriteString("\n")
	s.WriteString(styles.HelpStyle.Render("j/k: navigate  space: toggle  enter: confirm\n"))
	return s.String()
}

// renderConfigScreen shows the configuration form.
func (m AppModel) renderConfigScreen() string {
	var s strings.Builder
	s.WriteString(styles.TitleStyle.Render("Configuration\n\n"))
	s.WriteString("Edit your git-courer settings:\n\n")
	s.WriteString(m.form.View())
	s.WriteString("\n")
	s.WriteString(styles.HelpStyle.Render("j/k: navigate  h/l: change value  enter: next\n"))
	return s.String()
}

// renderMCPSetupScreen shows MCP setup options.
func (m AppModel) renderMCPSetupScreen() string {
	var s strings.Builder
	s.WriteString(styles.TitleStyle.Render("MCP Setup\n\n"))
	s.WriteString("Configure MCP clients for git-courer:\n\n")
	s.WriteString(m.checkbox.View())
	s.WriteString("\n")
	s.WriteString(styles.HelpStyle.Render("j/k: navigate  space: toggle  enter: save & finish\n"))
	return s.String()
}

// renderDoneScreen shows the completion screen.
func (m AppModel) renderDoneScreen() string {
	var s strings.Builder
	s.WriteString(styles.SuccessStyle.Render("✓ git-courer setup complete!\n\n"))
	if len(m.selectedClients) > 0 {
		s.WriteString(fmt.Sprintf("Configured for: %v\n", m.selectedClients))
	}
	s.WriteString("\n")
	s.WriteString(styles.SubtextStyle.Render("Run 'git-courer mcp' to start the server.\n"))
	return s.String()
}

// renderHelp renders contextual help.
func (m AppModel) renderHelp() string {
	return styles.HelpStyle.Render("ctrl+c: quit")
}

// Run creates and executes the TUI program.
func Run(width, height int) error {
	p := tea.NewProgram(
		NewAppModel(width, height),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}