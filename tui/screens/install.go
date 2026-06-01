// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/installer"
	"github.com/Alejandro-M-P/git-courer/tui/components"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	resolving   bool
	resolvedCtx int
	spin        spinner.Model
}

// NewInstallScreen creates a new install wizard screen.
func NewInstallScreen(width int, cfg *config.Config) InstallScreen {
	// Check if config already exists
	hasConfig := false
	if _, err := config.Load(); err == nil {
		hasConfig = true
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Cyan)

	return InstallScreen{
		step:      0,
		width:     width,
		hasConfig: hasConfig,
		cfg:       cfg,
		form:      components.NewFormModel(cfg, width),
		spin:      s,
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

	case spinner.TickMsg:
		if m.step == 2 && m.resolving {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case components.ResolvedContextMsg:
		m.resolving = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.resolvedCtx = msg.Ctx
		if msg.Ctx > 0 {
			m.cfg.LLM.ContextWindow = msg.Ctx
			m.cfg.LLM.ContextWindowStr = fmt.Sprintf("%d", msg.Ctx)
			m.form = components.NewFormModel(m.cfg, m.width)
		}
		// Advance to finish (step 3)
		m.step = 3
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "down":
			m.form.Update(msg)
		case "up":
			m.form.Update(msg)
		case "h", "left", "l", "right":
			m.form.Update(msg)
		case " ":
			m.form.Update(msg)
		}
	}

	return m, nil
}

// startContextResolution kicks off context window resolution in a goroutine.
func (m *InstallScreen) startContextResolution() tea.Cmd {
	modelName := m.cfg.LLM.Model
	baseURL := m.cfg.LLM.BaseURL
	return func() tea.Msg {
		ctx, err := installer.ResolveContextWindow(modelName, baseURL, m.cfg.LLM.ContextWindow)
		return components.ResolvedContextMsg{Ctx: ctx, Err: err}
	}
}

// handleEnter handles the enter key based on current step.
// New flow: 0=Welcome, 1=MCP Config, 2=LLM Configuration, 3=Finish
func (m *InstallScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // Welcome screen
		if m.hasConfig {
			m.step = 2 // Skip to LLM Config if config exists
		} else {
			m.step = 1
		}
	case 1: // MCP Config
		m.step = 2
	case 2: // LLM Configuration (form + context resolve)
		if !m.resolving {
			m.resolving = true
			return m, tea.Batch(m.spin.Tick, m.startContextResolution())
		}
		return m, nil
	case 3: // Finish
		// Save config
		if err := m.cfg.SaveGlobal(); err != nil {
			m.err = err
			return m, nil
		}
		m.confirmed = true
		return m, tea.Quit
	}
	return m, nil
}

// View renders the install wizard.
func (m InstallScreen) View() string {
	var s strings.Builder

	s.WriteString(styles.TitleStyle.Render("git-courer Setup Wizard") + "\n")
	steps := []string{"Welcome", "MCP Config", "LLM Config", "Finish"}
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(steps, m.step)) + "\n\n")

	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	switch m.step {
	case 0:
		s.WriteString(m.renderWelcome())
	case 1:
		s.WriteString(m.renderMCPConfig())
	case 2:
		s.WriteString(m.renderLLMConfig())
	case 3:
		s.WriteString(m.renderFinish())
	}

	s.WriteString("\n" + m.renderHelp())
	return s.String()
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

func (m InstallScreen) renderLLMConfig() string {
	var s strings.Builder

	s.WriteString(styles.SelectedStyle.Render("Step 2: LLM Configuration") + "\n\n")

	if m.resolving {
		spinView := m.spin.View()
		s.WriteString(styles.BoxHeaderStyle.Render("RESOLVING CONTEXT WINDOW") + "\n\n")
		s.WriteString(spinView + "  " + styles.BoxContentStyle.Render("Detecting context window...") + "\n")
		s.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("  Model:    %s\n", m.cfg.LLM.Model)))
		s.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("  Base URL: %s\n", m.cfg.LLM.BaseURL)))
		s.WriteString(styles.BoxHelpStyle.Render("ctrl+c: cancel"))
	} else {
		s.WriteString(styles.BoxContentStyle.Render("Configure LLM provider, model, and context window.") + "\n\n")
		s.WriteString(m.form.View())
		if m.resolvedCtx > 0 {
			s.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("\n  Context Window: %d tokens (resolved)\n", m.resolvedCtx)))
		}
		s.WriteString("\n")
		s.WriteString(styles.BoxHelpStyle.Render("ctrl+r: resolve context  enter: resolve & continue  ctrl+c: cancel"))
	}

	return s.String()
}

func (m InstallScreen) renderFinish() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Step 3: Finish") + "\n\n")
	s.WriteString(styles.SuccessStyle.Render("Configuration saved successfully!") + "\n\n")
	s.WriteString("Press ENTER to exit or ctrl+c to quit.\n")
	return s.String()
}

func (m InstallScreen) renderHelp() string {
	if m.step == 0 || m.step == 3 {
		return styles.HelpStyle.Render("enter: confirm  ctrl+c: quit")
	}
	return styles.HelpStyle.Render("h/l: change value  ctrl+r: resolve context  enter: next  ctrl+c: back")
}
