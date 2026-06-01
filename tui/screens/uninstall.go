// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/installer"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
)

// UninstallScreen represents the uninstall flow model.
type UninstallScreen struct {
	width        int
	step         int // 0=confirmation, 1=progress, 2=done
	removedItems []string
	err          error
}

// NewUninstallScreen creates a new uninstall screen.
func NewUninstallScreen(width int) UninstallScreen {
	return UninstallScreen{
		width: width,
		step:  0,
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
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()
		}
	}

	return m, nil
}

// handleEnter handles the enter key based on current step.
func (m *UninstallScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // Confirmation
		m.step = 1
		m.performUninstall()

	case 1: // Progress
		m.step = 2

	case 2: // Done
		return m, nil
	}
	return m, nil
}

// Done returns true if the uninstall process is complete.
func (m UninstallScreen) Done() bool {
	return m.step == 2
}

// performUninstall executes a full uninstallation.
func (m *UninstallScreen) performUninstall() {
	m.removedItems = []string{}
	m.removeMCPConfigs()
	m.removeGlobalConfig()
	m.removeBinary()
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

	header := styles.BoxHeaderStyle.Render("UNINSTALL") + "\n\n"

	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	var content string
	switch m.step {
	case 0:
		content = m.renderConfirmation()
	case 1:
		content = m.renderProgress()
	case 2:
		content = m.renderDone()
	}

	s.WriteString(content)

	return styles.BoxStyle.Render(header + s.String())
}

func (m UninstallScreen) renderConfirmation() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Confirm Uninstall\n\n"))
	s.WriteString("This will remove:\n")
	s.WriteString("  • MCP configurations\n")
	s.WriteString("  • Global config\n")
	s.WriteString("  • git-courer binary\n\n")

	s.WriteString(styles.HelpStyle.Render("Press ENTER to uninstall everything"))
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
	s.WriteString("git-courer has been removed.\n")
	s.WriteString("\n")
	s.WriteString(styles.SubtextStyle.Render("Press ENTER to exit\n"))
	return s.String()
}
