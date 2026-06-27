// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/blak0p/git-courer/internal/installer"
	"github.com/blak0p/git-courer/tui/styles"
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
	m.removeHooks()
	m.removeGlobalConfig()
	m.removeBinary()
}

// removeHooks removes hooks.json, prompt blocks, and GIT_COURER.md for each client.
func (m *UninstallScreen) removeHooks() {
	clients := installer.MCPClients()
	for _, client := range clients {
		// Remove hooks.json if configured.
		if client.HooksConfig != nil {
			if err := installer.RemoveHook(client.HooksConfig.HooksPath); err == nil {
				m.removedItems = append(m.removedItems, fmt.Sprintf("Hooks: %s", client.Name))
			}
		}

		// Remove prompt block from target file.
		if err := installer.RemovePromptBlock(client); err == nil {
			m.removedItems = append(m.removedItems, fmt.Sprintf("Prompt block: %s", client.Name))
		}

		// Remove GIT_COURER.md.
		configPath := client.Paths[0]
		for _, path := range client.Paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
		rulesPath := filepath.Join(filepath.Dir(configPath), "GIT_COURER.md")
		if err := os.Remove(rulesPath); err == nil {
			m.removedItems = append(m.removedItems, fmt.Sprintf("GIT_COURER.md: %s", client.Name))
		}
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
// Supports JSON and TOML formats based on the client's ConfigFormat.
func (m *UninstallScreen) removeFromConfig(configPath string, client *installer.MCPClient) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	format := client.ConfigFormat
	if format == "" {
		format = "json"
	}

	switch format {
	case "toml":
		return removeFromTOMLConfig(configPath, client.RootKey, data)
	default:
		return removeFromJSONConfig(configPath, client.RootKey, data)
	}
}

// removeFromJSONConfig removes the git-courer entry from a JSON config file.
func removeFromJSONConfig(configPath, rootKey string, data []byte) error {
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	rootMap, ok := config[rootKey].(map[string]interface{})
	if !ok {
		return nil // no root key — nothing to remove
	}

	delete(rootMap, "git-courer")

	// If the root key is now empty, remove it too.
	if len(rootMap) == 0 {
		delete(config, rootKey)
	}

	// If the config is now empty, remove the file.
	if len(config) == 0 {
		return os.Remove(configPath)
	}

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, output, 0644)
}

// removeFromTOMLConfig removes the git-courer entry from a TOML config file.
func removeFromTOMLConfig(configPath, rootKey string, data []byte) error {
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		return err
	}

	rootMap, ok := config[rootKey].(map[string]interface{})
	if !ok {
		return nil // no root key — nothing to remove
	}

	delete(rootMap, "git-courer")

	// If the root key is now empty, remove it too.
	if len(rootMap) == 0 {
		delete(config, rootKey)
	}

	// If the config is now empty, remove the file.
	if len(config) == 0 {
		return os.Remove(configPath)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(config); err != nil {
		return err
	}
	return os.WriteFile(configPath, buf.Bytes(), 0644)
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
