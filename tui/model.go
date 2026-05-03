// Package tui provides the git-courer interactive TUI.
package tui

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/tui/screens"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppState represents the current state of the TUI.
type AppState int

const (
	stateMain AppState = iota
	stateInstall
	stateUninstall
	stateMCPSetup
	stateDone
)

// MenuState represents the main menu selection.
type MenuState int

const (
	menuInstall MenuState = iota
	menuUninstall
	menuMCPSetup
	menuQuit
)

// String returns a human-readable name for the state.
func (s AppState) String() string {
	switch s {
	case stateMain:
		return "Main"
	case stateInstall:
		return "Install"
	case stateUninstall:
		return "Uninstall"
	case stateMCPSetup:
		return "MCP Setup"
	case stateDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// AppModel is the main TUI model.
type AppModel struct {
	state     AppState
	menuState MenuState
	width     int
	height    int
	install   screens.InstallScreen
	uninstall screens.UninstallScreen
	mcpSetup  screens.MCPSetupScreen
	err       error
}

// NewAppModel creates a new app model.
func NewAppModel(width, height int) AppModel {
	cfg := config.Default()

	// Check if global config exists
	if _, err := config.Load(); err == nil {
		cfg, _ = config.Load()
	}

	return AppModel{
		state:     stateMain,
		menuState: menuInstall,
		width:     width,
		height:    height,
		install:   screens.NewInstallScreen(width, cfg),
		uninstall: screens.NewUninstallScreen(width),
		mcpSetup:  screens.NewMCPSetupScreen(width),
	}
}

// Init initializes the TUI.
func (m AppModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the app.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		case "j", "down":
			return m.handleNav(1)
		case "k", "up":
			return m.handleNav(-1)

		case "h", "left", "l", "right", " ":
			// Pass to current screen
			return m.handleScreenInput(msg.String())
		}
	}

	return m, nil
}

// handleNav handles menu navigation.
func (m AppModel) handleNav(delta int) (tea.Model, tea.Cmd) {
	if m.state != stateMain {
		return m, nil
	}

	newMenu := int(m.menuState) + delta
	if newMenu < 0 {
		newMenu = int(menuQuit)
	} else if newMenu > int(menuQuit) {
		newMenu = int(menuInstall)
	}
	m.menuState = MenuState(newMenu)
	return m, nil
}

// handleScreenInput passes input to the current screen.
func (m AppModel) handleScreenInput(key string) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateInstall:
		m.install.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	case stateUninstall:
		m.uninstall.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	case stateMCPSetup:
		m.mcpSetup.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}
	return m, nil
}

// handleEnter handles the enter key based on current state.
func (m AppModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case stateMain:
		return m.handleMenuSelect()

	case stateInstall:
		m.install.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if m.install.IsConfirmed() {
			m.state = stateDone
		}
		// Move to next step
		m.install.NextStep()

	case stateUninstall:
		m.uninstall.Update(tea.KeyMsg{Type: tea.KeyEnter})

	case stateMCPSetup:
		m.mcpSetup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}

	return m, nil
}

// handleMenuSelect handles menu selection.
func (m AppModel) handleMenuSelect() (tea.Model, tea.Cmd) {
	switch m.menuState {
	case menuInstall:
		m.state = stateInstall
	case menuUninstall:
		m.state = stateUninstall
	case menuMCPSetup:
		m.state = stateMCPSetup
	case menuQuit:
		return m, tea.Quit
	}
	return m, nil
}

// View renders the TUI.
func (m AppModel) View() string {
	var s strings.Builder

	// Error if any
	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	// State-specific content
	switch m.state {
	case stateMain:
		s.WriteString(m.renderMainMenu())
	case stateInstall:
		s.WriteString(m.install.View())
	case stateUninstall:
		s.WriteString(m.uninstall.View())
	case stateMCPSetup:
		s.WriteString(m.mcpSetup.View())
	case stateDone:
		s.WriteString(m.renderDoneScreen())
	}

	return s.String()
}

// renderMainMenu shows the main menu.
func (m AppModel) renderMainMenu() string {
	var s strings.Builder

	s.WriteString(styles.TitleStyle.Render("git-courer") + "\n")
	s.WriteString(styles.SubtextStyle.Render("Interactive TUI Installer\n\n"))

	menuItems := []string{
		"Install / Configure",
		"Uninstall",
		"MCP Setup",
		"Quit",
	}

	for i, item := range menuItems {
		cursor := "  "
		if i == int(m.menuState) {
			cursor = "▸ "
		}

		style := lipgloss.NewStyle().Foreground(styles.Gray)
		if i == int(m.menuState) {
			style = lipgloss.NewStyle().Foreground(styles.CyanBold).Bold(true)
		}

		s.WriteString(cursor + style.Render(item) + "\n")
	}

	s.WriteString("\n" + styles.HelpStyle.Render("j/k: navigate  enter: select  ctrl+c: quit"))
	return s.String()
}

// renderDoneScreen shows the completion screen.
func (m AppModel) renderDoneScreen() string {
	var s strings.Builder
	s.WriteString(styles.SuccessStyle.Render("✓ git-courer setup complete!\n\n"))
	s.WriteString("Run 'git-courer mcp' to start the server.\n")
	return s.String()
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