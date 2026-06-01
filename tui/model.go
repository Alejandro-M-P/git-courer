// Package tui provides the git-courer interactive TUI.
package tui

import (
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/installer"
	"github.com/blak0p/git-courer/tui/components"
	"github.com/blak0p/git-courer/tui/screens"
	"github.com/blak0p/git-courer/tui/styles"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppState represents the current screen of the TUI.
type AppState int

const (
	stateWelcome AppState = iota
	stateMCPCfg
	stateLLMCfg // Merged: General Settings + LLM Context Window → LLM Configuration
	stateFinish
	stateUninstall
	stateMCPSetup
	stateUpdate
)

// updateMsg represents the result of a binary update.
type updateMsg struct {
	err error
}

// versionFoundMsg is sent when a new version is detected.
type versionFoundMsg struct {
	version string
}

// MenuOption represents options in the welcome screen.
type MenuOption int

const (
	menuInstall MenuOption = iota
	menuUpdateBinary
	menuUninstall
	menuQuit
)

// AppModel is the main TUI model.
type AppModel struct {
	state         AppState
	history       []AppState
	menuState     MenuOption
	width         int
	height        int
	install       screens.InstallScreen
	uninstall     screens.UninstallScreen
	mcpSetup      screens.MCPSetupScreen
	form          components.FormModel
	cfg           *config.Config
	updating      bool
	targetVersion string
	resolving     bool
	spin          spinner.Model
	err           error
}

// NewAppModel creates a new app model.
func NewAppModel(width, height int) AppModel {
	cfg := config.Default()

	// Check if global config exists
	if _, err := config.Load(); err == nil {
		cfg, _ = config.Load()
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Cyan)

	return AppModel{
		state:     stateWelcome,
		history:   []AppState{},
		menuState: menuInstall,
		width:     width,
		height:    height,
		cfg:       cfg,
		install:   screens.NewInstallScreen(width, cfg),
		uninstall: screens.NewUninstallScreen(width),
		mcpSetup:  screens.NewMCPSetupScreen(width),
		form:      components.NewFormModel(cfg, width),
		spin:      s,
	}
}

// Init initializes the TUI.
func (m AppModel) Init() tea.Cmd {
	return nil
}

// pushState adds a state to history and updates current state.
func (m *AppModel) pushState(s AppState) {
	m.history = append(m.history, m.state)
	m.state = s
}

// popState returns to previous state.
func (m *AppModel) popState() {
	if len(m.history) > 0 {
		m.state = m.history[len(m.history)-1]
		m.history = m.history[:len(m.history)-1]
	}
}

// Update handles messages for the app.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case versionFoundMsg:
		m.targetVersion = msg.version
		return m, func() tea.Msg {
			err := installer.DownloadUpdate()
			return updateMsg{err: err}
		}

	case updateMsg:
		m.updating = false
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case spinner.TickMsg:
		if m.state == stateLLMCfg && m.resolving {
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
		if msg.Ctx > 0 {
			m.cfg.LLM.ContextWindow = msg.Ctx
			m.cfg.LLM.ContextWindowStr = fmt.Sprintf("%d", msg.Ctx)
			m.form = components.NewFormModel(m.cfg, m.width)
		}
		m.pushState(stateFinish)
		return m, nil

	case tea.KeyMsg:
		// Handle keys based on state
		switch m.state {
		case stateUpdate:
			if msg.String() == "ctrl+c" || msg.String() == "esc" {
				m.popState()
				return m, nil
			}
			if !m.updating && msg.String() == "enter" {
				m.popState()
				return m, nil
			}
			return m, nil
		case stateUninstall:
			if msg.String() == "esc" {
				m.popState()
				return m, nil
			}
			var cmd tea.Cmd
			newModel, cmd := m.uninstall.Update(msg)
			m.uninstall = *(newModel.(*screens.UninstallScreen))
			if m.uninstall.Done() {
				m.popState()
			}
			return m, cmd
		case stateMCPSetup, stateMCPCfg:
			if msg.String() == "esc" {
				m.popState()
				return m, nil
			}
			// If it's stateMCPCfg and we press enter, we handle it here to move forward
			if msg.String() == "enter" && m.state == stateMCPCfg {
				return m.handleEnter()
			}

			var cmd tea.Cmd
			newModel, cmd := m.mcpSetup.Update(msg)
			m.mcpSetup = *(newModel.(*screens.MCPSetupScreen))
			// Only pop if it was stateMCPSetup (not stateMCPCfg which is part of install wizard)
			if m.state == stateMCPSetup && m.mcpSetup.Done() {
				m.popState()
			}
			return m, cmd
		case stateLLMCfg:
			if msg.String() == "esc" {
				m.popState()
				return m, nil
			}
			if msg.String() == "ctrl+r" {
				// Delegate to form which handles Ctrl+R for context resolution
				var cmd tea.Cmd
				newForm, cmd := m.form.Update(msg)
				m.form = *(newForm.(*components.FormModel))
				return m, cmd
			}
			if msg.String() == "enter" {
				return m.handleEnter()
			}
			var cmd tea.Cmd
			newForm, cmd := m.form.Update(msg)
			m.form = *(newForm.(*components.FormModel))
			return m, cmd
		}

		switch msg.String() {
		case "esc":
			if m.state == stateWelcome {
				return m, tea.Quit
			}
			m.popState()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()

		case "down":
			return m.handleNav(1)
		case "up":
			return m.handleNav(-1)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleNav handles navigation between menu options.
func (m *AppModel) handleNav(delta int) (tea.Model, tea.Cmd) {
	// Only navigate in welcome state
	if m.state != stateWelcome {
		return m, nil
	}

	newMenu := int(m.menuState) + delta
	if newMenu < 0 {
		newMenu = int(menuQuit)
	} else if newMenu > int(menuQuit) {
		newMenu = int(menuInstall)
	}
	m.menuState = MenuOption(newMenu)
	return m, nil
}

// handleEnter handles the enter key based on current state.
func (m *AppModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case stateWelcome:
		switch m.menuState {
		case menuInstall:
			m.pushState(stateMCPCfg)
		case menuUpdateBinary:
			m.pushState(stateUpdate)
			m.updating = true
			m.err = nil
			m.targetVersion = "..."
			return m, func() tea.Msg {
				_, version, err := installer.CheckForUpdates()
				if err != nil {
					return updateMsg{err: err}
				}
				return versionFoundMsg{version: version}
			}
		case menuUninstall:
			m.pushState(stateUninstall)
		case menuQuit:
			return m, tea.Quit
		}
		return m, nil

	case stateMCPCfg:
		// Continue to LLM Configuration step
		m.pushState(stateLLMCfg)

	case stateLLMCfg:
		// Save config and resolve context window, then go to Finish
		if err := m.cfg.SaveGlobal(); err != nil {
			m.err = err
		}
		if !m.resolving {
			m.resolving = true
			return m, tea.Batch(m.spin.Tick, m.startContextResolution())
		}
		return m, nil

	case stateFinish:
		return m, tea.Quit
	}

	return m, nil
}

func (m *AppModel) startContextResolution() tea.Cmd {
	modelName := m.cfg.LLM.Model
	baseURL := m.cfg.LLM.BaseURL
	return func() tea.Msg {
		ctx, err := installer.ResolveContextWindow(modelName, baseURL, m.cfg.LLM.ContextWindow)
		return components.ResolvedContextMsg{Ctx: ctx, Err: err}
	}
}

// renderLLMCfg shows the merged LLM Configuration step (form + context resolve).
func (m AppModel) renderLLMCfg() string {
	var s strings.Builder

	stepText := "Step 2/3: LLM Configuration"
	s.WriteString(styles.SubtextStyle.Render(stepText) + "\n\n")

	content := styles.BoxHeaderStyle.Render("LLM CONFIGURATION") + "\n\n"

	if m.resolving {
		spinView := m.spin.View()
		content += spinView + "  " + styles.BoxContentStyle.Render("Resolving context window...") + "\n"
		content += styles.SubtextStyle.Render(fmt.Sprintf("  Model:    %s\n", m.cfg.LLM.Model))
		content += styles.SubtextStyle.Render(fmt.Sprintf("  Base URL: %s\n", m.cfg.LLM.BaseURL))
		content += "\n"
		content += styles.BoxHelpStyle.Render("ctrl+c: cancel")
	} else {
		content += m.form.View() + "\n"
		if m.cfg.LLM.ContextWindow > 0 {
			content += styles.SubtextStyle.Render(fmt.Sprintf("  Context Window: %d tokens (resolved)\n", m.cfg.LLM.ContextWindow))
		}
		content += styles.BoxHelpStyle.Render("h/l: cycle/step  ctrl+r: resolve context  enter: continue  esc: back")
	}

	s.WriteString(styles.BoxStyle.Render(content))

	return s.String()
}

// View renders the TUI.
func (m AppModel) View() string {
	var content string

	// State-specific content
	switch m.state {
	case stateWelcome:
		content = m.renderWelcome()
	case stateMCPCfg:
		content = m.renderMCPCfg()
	case stateLLMCfg:
		content = m.renderLLMCfg()
	case stateFinish:
		content = m.renderFinish()
	case stateUninstall:
		content = m.uninstall.View()
	case stateMCPSetup:
		content = m.mcpSetup.View()
	case stateUpdate:
		content = m.renderUpdate()
	}

	return m.wrapScreen(content)
}

// renderUpdate shows the binary update screen.
func (m AppModel) renderUpdate() string {
	var s strings.Builder

	header := styles.BoxHeaderStyle.Render("UPDATE BINARY") + "\n\n"

	if m.updating {
		versionText := ""
		if m.targetVersion != "" {
			versionText = " (v" + m.targetVersion + ")"
		}
		s.WriteString(styles.SelectedStyle.Render("Downloading update"+versionText+"...") + "\n\n")
		s.WriteString(styles.SubtextStyle.Render("Please wait while we update git-courer to the latest version.\n"))
	} else if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render("Update failed!") + "\n\n")
		s.WriteString(fmt.Sprintf("%v\n\n", m.err))
		s.WriteString(styles.BoxHelpStyle.Render("enter: back  esc: back"))
	} else {
		s.WriteString(styles.SuccessStyle.Render("✓ Update complete!") + "\n\n")
		s.WriteString(fmt.Sprintf("git-courer has been updated to v%s successfully.\n", m.targetVersion))
		s.WriteString(styles.SubtitleStyle.Render("Note: You might need to restart the application to use the new version.\n\n"))
		s.WriteString(styles.BoxHelpStyle.Render("enter: back"))
	}

	return styles.BoxStyle.Render(header + s.String())
}

// wrapScreen wraps content in a box and centers it.
func (m AppModel) wrapScreen(content string) string {
	// Centering content
	centeredContent := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)

	return centeredContent
}

// renderWelcome shows the welcome screen with boxed options.
func (m AppModel) renderWelcome() string {
	var options []string

	menuOptions := []struct {
		opt  MenuOption
		text string
	}{
		{menuInstall, "Install / Update Config"},
		{menuUpdateBinary, "Update Binary"},
		{menuUninstall, "Uninstall"},
		{menuQuit, "Quit"},
	}

	for _, o := range menuOptions {
		style := styles.MenuItemStyle
		prefix := "  "
		if m.menuState == o.opt {
			style = styles.SelectedMenuItemStyle
			prefix = "▸ "
		}
		options = append(options, style.Render(prefix+o.text))
	}

	menu := lipgloss.JoinVertical(lipgloss.Left, options...)

	header := lipgloss.JoinVertical(
		lipgloss.Center,
		styles.TitleStyle.Render("git-courer"),
		styles.SubtitleStyle.Render("Interactive TUI Installer"),
		"",
	)

	help := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		m.renderHelp(),
	)

	return lipgloss.JoinVertical(lipgloss.Center, header, menu, help)
}

// renderMCPCfg shows the MCP configuration screen.
func (m AppModel) renderMCPCfg() string {
	var s strings.Builder

	// Progress indicator
	s.WriteString(styles.SubtextStyle.Render("Step 1/3: MCP Configuration") + "\n\n")

	// Get clients from mcpSetup
	clients := m.mcpSetup.Items()
	var clientList strings.Builder
	for i, client := range clients {
		cursor := "  "
		if i == m.mcpSetup.Cursor() {
			cursor = styles.CheckboxFocused.Render()
		} else {
			cursor = styles.CheckboxUnfocused.Render()
		}

		selected := "[ ] "
		if client.Selected {
			selected = "[" + styles.Checked.Render() + "] "
		}

		detected := ""
		if client.Detected {
			detected = styles.SubtitleStyle.Render(" (detected)")
		}

		clientList.WriteString(cursor + selected + styles.BoxContentStyle.Render(client.Name) + detected + "\n")
	}

	content := styles.BoxHeaderStyle.Render("CONFIG MCP") + "\n\n" +
		styles.BoxContentStyle.Render("Select MCP clients to configure:\n\n") +
		clientList.String() + "\n" +
		styles.BoxHelpStyle.Render("space: toggle  enter: continue")

	s.WriteString(styles.BoxStyle.Render(content))

	return s.String()
}

// renderFinish shows the finish screen.
func (m AppModel) renderFinish() string {
	var s strings.Builder

	content := styles.BoxHeaderStyle.Render("FINISH") + "\n\n"
	content += styles.SuccessStyle.Render("✓ Configuration saved successfully!") + "\n\n"
	content += styles.BoxHelpStyle.Render("enter: exit")

	s.WriteString(styles.BoxStyle.Render(content))

	return s.String()
}

// renderHelp shows the help bar.
func (m AppModel) renderHelp() string {
	switch m.state {
	case stateWelcome:
		return styles.HelpStyle.Render("up/down: navigate  enter: select  ctrl+c: quit")
	case stateMCPCfg:
		return styles.HelpStyle.Render("up/down: navigate  space: toggle  enter: continue")
	case stateLLMCfg:
		return styles.HelpStyle.Render("ctrl+r: resolve context  enter: continue  esc: back")
	case stateFinish:
		return styles.HelpStyle.Render("enter: exit")
	default:
		return styles.HelpStyle.Render("ctrl+c: quit")
	}
}

// Run creates and executes the TUI program.
func Run(width, height int) error {
	m := NewAppModel(width, height)
	p := tea.NewProgram(
		&m,
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
