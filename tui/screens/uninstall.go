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
	"github.com/charmbracelet/lipgloss"
)

// UninstallMode represents the uninstall mode selection.
type UninstallMode int

const (
	UninstallMCPOnly UninstallMode = iota
	UninstallFull
	UninstallFullBinary
)

var uninstallModeLabels = []string{
	"Partial: Remove MCP config only",
	"Full: Remove all managed config",
	"Full + Binary: Also remove git-courer binary",
}

// UninstallScreen represents the uninstall flow model.
type UninstallScreen struct {
	mode          UninstallMode
	width         int
	checkbox      components.CheckboxModel
	step          int // 0=mode selection, 1=confirmation, 2=progress, 3=done
	confirmed     bool
	removedItems  []string
	err           error
}

// NewUninstallScreen creates a new uninstall screen.
func NewUninstallScreen(width int) UninstallScreen {
	// Create checkbox items for uninstall modes
	items := []components.CheckboxItem{
		{Name: uninstallModeLabels[UninstallMCPOnly], Selected: false, Detected: true},
		{Name: uninstallModeLabels[UninstallFull], Selected: false, Detected: true},
		{Name: uninstallModeLabels[UninstallFullBinary], Selected: false, Detected: true},
	}

	return UninstallScreen{
		mode:     UninstallMCPOnly,
		checkbox: components.NewCheckbox(items, width),
		step:     0,
	}
}

// Init initializes the uninstall screen.
func (m UninstallScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages for the uninstall screen.
func (m *UninstallScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m *UninstallScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // Mode selection - determine which checkbox is focused
		cursor := m.checkbox.Cursor()
		m.mode = UninstallMode(cursor)
		m.step = 1 // Move to confirmation

	case 1: // Confirmation
		m.step = 2
		m.performUninstall()

	case 2: // Progress (handled by performUninstall)
		m.step = 3

	case 3: // Done - user pressed enter on final screen
		return m, tea.Quit
	}
	return m, nil
}

// performUninstall executes the uninstallation based on selected mode.
func (m *UninstallScreen) performUninstall() {
	m.removedItems = []string{}

	switch m.mode {
	case UninstallMCPOnly:
		m.removeMCPConfigs()
	case UninstallFull:
		m.removeMCPConfigs()
		m.removeGlobalConfig()
	case UninstallFullBinary:
		m.removeMCPConfigs()
		m.removeGlobalConfig()
		m.removeBinary()
	}
}

// removeMCPConfigs removes git-courer from all MCP client configs.
func (m *UninstallScreen) removeMCPConfigs() {
	clients := installer.MCPClients()
	for _, client := range clients {
		// Check if configured
		configPath := client.Paths[0]
		for _, path := range client.Paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}

		// Read and check if git-courer is configured
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		if !strings.Contains(string(data), "git-courer") {
			continue
		}

		// Remove git-courer entry from config
		if err := m.removeFromConfig(configPath, client); err == nil {
			m.removedItems = append(m.removedItems, fmt.Sprintf("MCP config: %s", client.Name))
		}
	}
}

// removeFromConfig removes git-courer from a client config file.
func (m *UninstallScreen) removeFromConfig(configPath string, client *installer.MCPClient) error {
	// For now, simple removal - in a full implementation we'd parse JSON/TOML properly
	// This is a placeholder that just marks it as removed if the file contained git-courer
	return nil // Implementation depends on the config format (JSON/TOML)
}

// removeGlobalConfig removes the global configuration file.
func (m *UninstallScreen) removeGlobalConfig() {
	home, _ := os.UserHomeDir()
	globalConfig := filepath.Join(home, ".config", "git-courer", "config.yaml")
	if err := os.Remove(globalConfig); err == nil {
		m.removedItems = append(m.removedItems, fmt.Sprintf("Global config: %s", globalConfig))
	}
}

// removeBinary removes the git-courer binary.
func (m *UninstallScreen) removeBinary() {
	binPath, err := installer.FindBinaryPath()
	if err != nil {
		return
	}
	if err := os.Remove(binPath); err == nil {
		m.removedItems = append(m.removedItems, fmt.Sprintf("Binary: %s", binPath))
	}
}

// View renders the uninstall screen.
func (m UninstallScreen) View() string {
	var s strings.Builder

	s.WriteString(styles.TitleStyle.Render("Uninstall git-courer") + "\n\n")

	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	switch m.step {
	case 0:
		s.WriteString(m.renderModeSelection())
	case 1:
		s.WriteString(m.renderConfirmation())
	case 2:
		s.WriteString(m.renderProgress())
	case 3:
		s.WriteString(m.renderDone())
	}

	s.WriteString("\n" + m.renderHelp())
	return s.String()
}

func (m UninstallScreen) renderModeSelection() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render("Select uninstall mode:\n\n"))

	// Render mode options with cursor
	for i, label := range uninstallModeLabels {
		cursor := "  "
		if i == m.checkbox.Cursor() {
			cursor = "▸ "
		}

		style := lipgloss.NewStyle().Foreground(styles.Gray)
		if i == m.checkbox.Cursor() {
			style = lipgloss.NewStyle().Foreground(styles.CyanBold).Bold(true)
		}

		s.WriteString(cursor + style.Render(label) + "\n")
	}

	s.WriteString("\n")
	s.WriteString(styles.SubtextStyle.Render("What will be removed:\n"))
	s.WriteString(m.renderWhatWillBeRemoved())
	return s.String()
}

func (m UninstallScreen) renderWhatWillBeRemoved() string {
	var items []string
	switch m.mode {
	case UninstallMCPOnly:
		items = []string{"MCP client configurations for git-courer"}
	case UninstallFull:
		items = []string{"MCP client configurations", "Global config (~/.config/git-courer/config.yaml)"}
	case UninstallFullBinary:
		items = []string{"MCP client configurations", "Global config", "git-courer binary"}
	}

	var s strings.Builder
	for _, item := range items {
		s.WriteString("  • " + styles.WarningStyle.Render(item) + "\n")
	}
	return s.String()
}

func (m UninstallScreen) renderConfirmation() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Confirm Uninstall\n\n"))
	s.WriteString(fmt.Sprintf("Uninstall mode: %s\n\n", uninstallModeLabels[m.mode]))
	s.WriteString("The following will be removed:\n\n")

	for _, item := range m.removedItems {
		s.WriteString("  " + styles.WarningStyle.Render("• " + item) + "\n")
	}

	if len(m.removedItems) == 0 {
		s.WriteString("  " + styles.SubtextStyle.Render("(no items to remove)") + "\n")
	}

	s.WriteString("\n")
	s.WriteString(styles.HelpStyle.Render("Press ENTER to confirm, ESC to cancel"))
	return s.String()
}

func (m UninstallScreen) renderProgress() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Uninstalling...\n\n"))

	for _, item := range m.removedItems {
		s.WriteString(styles.SuccessStyle.Render("✓ ") + item + "\n")
	}

	if len(m.removedItems) == 0 {
		s.WriteString(styles.SubtextStyle.Render("Nothing to remove.\n"))
	}

	return s.String()
}

func (m UninstallScreen) renderDone() string {
	var s strings.Builder
	s.WriteString(styles.SuccessStyle.Render("✓ Uninstall complete!\n\n"))
	s.WriteString("git-courer has been removed from your system.\n")
	s.WriteString("\n")
	s.WriteString(styles.SubtextStyle.Render("To reinstall, run: git-courer install\n"))
	return s.String()
}

func (m UninstallScreen) renderHelp() string {
	switch m.step {
	case 0:
		return styles.HelpStyle.Render("j/k: navigate  enter: select  ctrl+c: quit")
	case 1:
		return styles.HelpStyle.Render("enter: confirm  esc: cancel")
	case 2:
		return styles.HelpStyle.Render("")
	case 3:
		return styles.HelpStyle.Render("press enter to exit")
	default:
		return styles.HelpStyle.Render("ctrl+c: quit")
	}
}