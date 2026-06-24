// Package screens provides the TUI wizard screens for git-courer.
//
// This file implements the RestoreScreen: a three-step wizard (confirmation →
// progress → done) that restores MCP client config backups and removes the
// git-courer hook + GIT_COURER.md for each detected client. It mirrors
// UninstallScreen's structure exactly, but delegates the destructive work to
// installer.RunRestore (config-only; it never removes the binary or global
// config).
package screens

import (
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/installer"
	"github.com/blak0p/git-courer/tui/styles"
	"github.com/charmbracelet/bubbletea"
)

// RestoreScreen represents the restore flow model.
type RestoreScreen struct {
	width         int
	step          int // 0=confirmation, 1=progress, 2=done
	restoredItems []string
	err           error
}

// NewRestoreScreen creates a new restore screen.
func NewRestoreScreen(width int) RestoreScreen {
	return RestoreScreen{
		width: width,
		step:  0,
	}
}

// Init initializes the restore screen.
func (m RestoreScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages for the restore screen.
func (m *RestoreScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m *RestoreScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // Confirmation
		m.step = 1
		m.performRestore()

	case 1: // Progress
		m.step = 2

	case 2: // Done
		return m, nil
	}
	return m, nil
}

// Done returns true if the restore process is complete.
func (m RestoreScreen) Done() bool {
	return m.step == 2
}

// performRestore executes a config-only restore via installer.RunRestore.
// The installer reports per-client actions on stdout; we capture a summary
// list for the progress view. Unlike uninstall, the binary and global config
// are left untouched.
func (m *RestoreScreen) performRestore() {
	m.restoredItems = []string{}
	if err := installer.RunRestore(); err != nil {
		m.err = err
		return
	}
	// Report a single summary entry derived from the clients the installer
	// iterated over. installer.RunRestore already prints per-client detail;
	// the screen records a high-level confirmation for the progress view.
	for _, client := range installer.DetectClients() {
		m.restoredItems = append(m.restoredItems, fmt.Sprintf("Config restored: %s", client.Name))
	}
}

// View renders the restore screen.
func (m RestoreScreen) View() string {
	var s strings.Builder

	header := styles.BoxHeaderStyle.Render("RESTORE") + "\n\n"

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

func (m RestoreScreen) renderConfirmation() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Confirm Restore\n\n"))
	s.WriteString("This will restore MCP configs from backups and remove hooks\n")

	s.WriteString(styles.HelpStyle.Render("Press ENTER to restore configs"))
	return s.String()
}

func (m RestoreScreen) renderProgress() string {
	var s strings.Builder
	s.WriteString(styles.SelectedStyle.Render("Restoring...\n\n"))

	for _, item := range m.restoredItems {
		s.WriteString(styles.SuccessStyle.Render("✓ ") + item + "\n")
	}

	if len(m.restoredItems) == 0 {
		s.WriteString(styles.SubtextStyle.Render("Nothing to restore.\n"))
	}

	return s.String()
}

func (m RestoreScreen) renderDone() string {
	var s strings.Builder
	s.WriteString(styles.SuccessStyle.Render("✓ Restore complete!\n\n"))
	s.WriteString("MCP configs have been restored from backups.\n")
	s.WriteString("\n")
	s.WriteString(styles.SubtextStyle.Render("Press ENTER to exit\n"))
	return s.String()
}