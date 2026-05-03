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
			m.step = 5 // Skip to review if config exists
		} else {
			m.step = 1
		}
	case 1: // LLM config
		m.step = 2
	case 2: // Preview settings
		m.step = 3
	case 3: // Git settings
		m.step = 4
	case 4: // Context settings
		m.step = 5
	case 5: // Review
		// Save config
		if err := m.cfg.SaveGlobal(); err != nil {
			m.err = err
			return m, nil
		}
		m.confirmed = true
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
		s.WriteString(m.renderLLMConfig())
	case 2:
		s.WriteString(m.renderPreviewConfig())
	case 3:
		s.WriteString(m.renderGitConfig())
	case 4:
		s.WriteString(m.renderContextConfig())
	case 5:
		s.WriteString(m.renderReview())
	}

	s.WriteString("\n" + m.renderHelp())
	return s.String()
}

func (m InstallScreen) progressIndicator() string {
	steps := []string{"Welcome", "LLM", "Preview", "Git", "Context", "Review"}
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

func (m InstallScreen) renderLLMConfig() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 1: LLM Configuration\n\n"))
	s.WriteString(styles.SubtextStyle.Render("Configure your LLM provider settings.\n\n"))

	s.WriteString(m.form.View())
	return s.String()
}

func (m InstallScreen) renderPreviewConfig() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 2: Preview Settings\n\n"))
	s.WriteString(styles.SubtextStyle.Render("Configure preview behavior for git operations.\n\n"))
	s.WriteString(m.form.View())
	return s.String()
}

func (m InstallScreen) renderGitConfig() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 3: Git Settings\n\n"))
	s.WriteString(styles.SubtextStyle.Render("Configure git integration settings.\n\n"))
	s.WriteString(m.form.View())
	return s.String()
}

func (m InstallScreen) renderContextConfig() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 4: Context Settings\n\n"))
	s.WriteString(styles.SubtextStyle.Render("Configure project context for LLM prompts.\n\n"))
	s.WriteString(m.form.View())
	return s.String()
}

func (m InstallScreen) renderReview() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 5: Review & Save\n\n"))
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
	if m.step < 5 {
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