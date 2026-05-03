// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/tui/components"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
)

// InstallScreen represents the installation wizard model.
type InstallScreen struct {
	step        int
	width       int
	hasConfig   bool
	cfg         *config.Config
	form        components.FormModel
	err         error
	confirmed   bool
}

// NewInstallScreen creates a new install wizard screen.
func NewInstallScreen(width int, cfg *config.Config) InstallScreen {
	// Check if config already exists
	hasConfig := false
	if _, err := config.Load(); err == nil {
		hasConfig = true
	}

	return InstallScreen{
		step:      0,
		width:     width,
		hasConfig: hasConfig,
		cfg:       cfg,
		form:      components.NewFormModel(cfg, width),
	}
}

// Init initializes the install screen.
func (m InstallScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages for the install screen.
func (m *InstallScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		case "j", "down":
			m.form.Update(msg)
		case "k", "up":
			m.form.Update(msg)
		case "h", "left", "l", "right":
			m.form.Update(msg)
		case " ":
			m.form.Update(msg)
		}
	}

	return m, nil
}

// handleEnter handles the enter key based on current step.
func (m *InstallScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // Welcome screen
		if m.hasConfig {
			m.step = 3 // Skip to review if config exists
		} else {
			m.step = 1
		}
	case 1: // MCP Config
		m.step = 2
	case 2: // YAML Config
		m.step = 3
	case 3: // Review
		// Save config
		if err := m.cfg.SaveGlobal(); err != nil {
			m.err = err
			return m, nil
		}
		m.confirmed = true
	case 4: // Finish
		return m, tea.Quit
	}
	return m, nil
}

// View renders the install wizard.
func (m InstallScreen) View() string {
	var s strings.Builder

	s.WriteString(styles.TitleStyle.Render("git-courer Setup Wizard") + "\n")
	s.WriteString(styles.SubtextStyle.Render(m.progressIndicator()) + "\n\n")

	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	switch m.step {
	case 0:
		s.WriteString(m.renderWelcome())
	case 1:
		s.WriteString(m.renderMCPConfig())
	case 2:
		s.WriteString(m.renderYAMLConfig())
	case 3:
		s.WriteString(m.renderReview())
	case 4:
		s.WriteString(m.renderFinish())
	}

	s.WriteString("\n" + m.renderHelp())
	return s.String()
}

func (m InstallScreen) progressIndicator() string {
	steps := []string{"Welcome", "MCP Config", "YAML Config", "Review", "Finish"}
	var result []string
	for i, step := range steps {
		if i == m.step {
			result = append(result, styles.SelectedStyle.Render("["+step+"]"))
		} else if i < m.step {
			result = append(result, styles.SuccessStyle.Render("✓ "+step))
		} else {
			result = append(result, styles.SubtextStyle.Render(step))
		}
	}
	return strings.Join(result, " → ")
}

func (m InstallScreen) renderWelcome() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render("Welcome to git-courer!\n\n"))
	s.WriteString("This wizard will help you configure git-courer on your system.\n\n")

	if m.hasConfig {
		s.WriteString(styles.SuccessStyle.Render("✓ Existing configuration detected\n\n"))
		s.WriteString("Press ENTER to review your current settings.\n")
	} else {
		s.WriteString("Press ENTER to start the setup process.\n")
	}
	return s.String()
}

func (m InstallScreen) renderMCPConfig() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 1: MCP Configuration\n\n"))
	s.WriteString(styles.SubtextStyle.Render("Configure your MCP clients.\n\n"))

	s.WriteString(m.form.View())
	return s.String()
}

func (m InstallScreen) renderYAMLConfig() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 2: YAML Configuration\n\n"))
	s.WriteString(styles.SubtextStyle.Render("Configure git-courer via YAML files.\n\n"))
	s.WriteString(m.form.View())
	return s.String()
}

func (m InstallScreen) renderReview() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 3: Review & Save\n\n"))
	s.WriteString("Please review your configuration:\n\n")

	// LLM Settings
	s.WriteString(styles.SubtextStyle.Render("LLM Settings:\n"))
	s.WriteString(fmt.Sprintf("  Provider:      %s\n", m.cfg.LLM.Provider))
	s.WriteString(fmt.Sprintf("  Model:         %s\n", m.cfg.LLM.Model))
	s.WriteString(fmt.Sprintf("  Base URL:      %s\n", m.cfg.LLM.BaseURL))
	s.WriteString(fmt.Sprintf("  Parallel:      %d\n\n", m.cfg.LLM.NumParallel))

	// Preview Settings
	s.WriteString(styles.SubtextStyle.Render("Preview Settings:\n"))
	s.WriteString(fmt.Sprintf("  Enabled:       %t\n\n", m.cfg.Preview.Enabled))

	// Git Settings
	s.WriteString(styles.SubtextStyle.Render("Git Settings:\n"))
	s.WriteString(fmt.Sprintf("  WorkDir:       %s\n\n", m.cfg.Git.WorkDir))

	// Context Settings
	s.WriteString(styles.SubtextStyle.Render("Context Settings:\n"))
	s.WriteString(fmt.Sprintf("  Project:       %s\n", m.cfg.Context.Project))
	s.WriteString(fmt.Sprintf("  Style:         %s\n\n", m.cfg.Context.Style))

	if m.hasConfig {
		s.WriteString(styles.HelpStyle.Render("Press ENTER to update your configuration.\n"))
	} else {
		s.WriteString(styles.HelpStyle.Render("Press ENTER to save and continue.\n"))
	}
	return s.String()
}

func (m InstallScreen) renderFinish() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 4: Finish\n\n"))
	s.WriteString(styles.SuccessStyle.Render("✓ Configuration saved successfully!\n\n"))
	s.WriteString("Press ENTER to exit or ctrl+c to quit.\n")
	return s.String()
}

func (m InstallScreen) renderHelp() string {
	if m.step == 0 || m.step == 5 {
		return styles.HelpStyle.Render("j/k: navigate  enter: confirm  ctrl+c: quit")
	}
	return styles.HelpStyle.Render("j/k: navigate  h/l: change value  enter: next  esc: back")
}

// Config returns the current config.
func (m InstallScreen) Config() *config.Config {
	return m.cfg
}

// IsConfirmed returns true if the user confirmed the configuration.
func (m InstallScreen) IsConfirmed() bool {
	return m.confirmed
}

// NextStep advances to the next step.
func (m *InstallScreen) NextStep() {
	if m.step < 4 {
		m.step++
	}
}

// PrevStep goes back to the previous step.
func (m *InstallScreen) PrevStep() {
	if m.step > 0 {
		m.step--
	}
}

// SetStep sets the current step directly.
func (m *InstallScreen) SetStep(step int) {
	m.step = step
}