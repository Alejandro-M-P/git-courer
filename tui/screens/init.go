// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/infra/chunkers"
	"github.com/blak0p/git-courer/tui/components"
	"github.com/blak0p/git-courer/tui/styles"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// initBoxStyle is a wider variant of BoxStyle for the init wizard.
var initBoxStyle = styles.BoxStyle.Copy().Width(76)

const (
	stepWelcome      = 0
	stepDescription  = 1
	stepBaseBranch   = 2
	stepTestCommand  = 3
	stepGrammars     = 4
	stepReview       = 5
	stepFinish       = 6
	stepAIGenerating = 7

	progressSteps = "Welcome,Description,Base Branch,Test Command,Grammars,Review,Finish"
)

// aiResultMsg carries the result of a background ProjectInit call.
type aiResultMsg struct {
	cfg *domain.ProjectConfig
	err error
}

// grammarResultMsg carries the result of a grammar download.
type grammarResultMsg struct {
	lang string
	err  error
}

func progressStepsList() []string {
	return strings.Split(progressSteps, ",")
}

// InitScreen represents the project init wizard model.
type InitScreen struct {
	step             int
	width            int
	height           int
	repoRoot         string
	hasConfig        bool
	descForm         components.DynamicFormModel
	baseBranchInput  textinput.Model
	testCommandInput textinput.Model
	err              error
	confirmed        bool
	llm              ports.LLM
	git              ports.Git
	menuCursor       int
	spin             spinner.Model
	downloading      bool
	grammars         map[string]bool // lang -> success
}

// NewInitScreen creates an init wizard without AI support.
func NewInitScreen(width int, repoRoot string, git ports.Git) InitScreen {
	return newInitScreen(width, repoRoot, nil, false, git)
}

// NewInitScreenWithLLM creates an init wizard with AI mode enabled.
func NewInitScreenWithLLM(width int, repoRoot string, llm ports.LLM, retry bool, git ports.Git) InitScreen {
	return newInitScreen(width, repoRoot, llm, retry, git)
}

func newInitScreen(width int, repoRoot string, llm ports.LLM, retry bool, git ports.Git) InitScreen {
	hasConfig := false
	var existingDesc string
	var existingBaseBranch string
	var existingTestCommand string

	if cfg, err := domain.LoadProjectConfig(repoRoot); err == nil && cfg != nil {
		hasConfig = true
		existingDesc = cfg.Description
		existingBaseBranch = cfg.BaseBranch
		existingTestCommand = cfg.TestCommand
	}

	// Auto-detect base branch as default, but let the user change it
	detectedBranch := ""
	if git != nil {
		detectedBranch = domain.DetectBaseBranch(git)
	}
	defaultBranch := detectedBranch
	if defaultBranch == "" {
		defaultBranch = existingBaseBranch
	}

	descFields := []components.DynamicField{{
		ID:          "description",
		Name:        "Description",
		Type:        components.DynFieldText,
		Value:       existingDesc,
		Placeholder: "Enter a one-sentence description of your project...",
	}}
	descForm := components.NewDynamicFormModel(descFields, width)

	baseBranchInput := textinput.New()
	baseBranchInput.Placeholder = "main"
	baseBranchInput.CharLimit = 100
	baseBranchInput.Width = 30
	baseBranchInput.SetValue(defaultBranch)
	baseBranchInput.Focus()

	testCommandInput := textinput.New()
	testCommandInput.Placeholder = "go test ./..."
	testCommandInput.CharLimit = 200
	testCommandInput.Width = 30
	if existingTestCommand != "" {
		testCommandInput.SetValue(existingTestCommand)
	} else {
		testCommandInput.SetValue("go test ./...")
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Cyan)

	return InitScreen{
		step:             stepWelcome,
		width:            width,
		height:           0,
		repoRoot:         repoRoot,
		hasConfig:        hasConfig,
		descForm:         descForm,
		baseBranchInput:  baseBranchInput,
		testCommandInput: testCommandInput,
		llm:              llm,
		git:              git,
		spin:             s,
	}
}

// Init initializes the init screen.
func (m InitScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages for the init screen.
func (m *InitScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.step == stepAIGenerating || m.downloading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case aiResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.step = stepWelcome
			return m, nil
		}
		m.applyAIResult(msg.cfg)
		m.step = stepGrammars
		return m, m.startGrammarDownload()

	case grammarResultMsg:
		if m.grammars == nil {
			m.grammars = make(map[string]bool)
		}
		m.grammars[msg.lang] = msg.err == nil
		m.downloading = false
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up":
			if m.step == stepWelcome && m.llm != nil {
				m.menuCursor--
				if m.menuCursor < 0 {
					m.menuCursor = 1
				}
			}
			return m, nil

		case "down":
			if m.step == stepWelcome && m.llm != nil {
				m.menuCursor++
				if m.menuCursor > 1 {
					m.menuCursor = 0
				}
			}
			return m, nil
		}
	}

	if m.step == stepDescription {
		updated, cmd := m.descForm.Update(msg)
		m.descForm = updated.(components.DynamicFormModel)
		return m, cmd
	}

	if m.step == stepBaseBranch {
		var cmd tea.Cmd
		m.baseBranchInput, cmd = m.baseBranchInput.Update(msg)
		return m, cmd
	}

	if m.step == stepTestCommand {
		var cmd tea.Cmd
		m.testCommandInput, cmd = m.testCommandInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *InitScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		if m.llm != nil && m.menuCursor == 1 {
			m.step = stepAIGenerating
			m.err = nil
			return m, tea.Batch(m.spin.Tick, m.startAIGeneration())
		}
		m.step = stepDescription
	case stepDescription:
		m.step = stepBaseBranch
		m.baseBranchInput.Focus()
	case stepBaseBranch:
		m.step = stepTestCommand
		m.testCommandInput.Focus()
	case stepTestCommand:
		m.step = stepGrammars
		m.downloading = true
		return m, tea.Batch(m.spin.Tick, m.startGrammarDownload())
	case stepGrammars:
		if m.downloading {
			return m, nil
		}
		m.step = stepReview
	case stepReview:
		return m.handleSave()
	case stepFinish:
		return m, tea.Quit
	}
	return m, nil
}

func (m *InitScreen) startAIGeneration() tea.Cmd {
	llm := m.llm
	repoRoot := m.repoRoot
	return func() tea.Msg {
		cfg, err := llm.ProjectInit(repoRoot)
		return aiResultMsg{cfg: cfg, err: err}
	}
}

func (m *InitScreen) startGrammarDownload() tea.Cmd {
	return func() tea.Msg {
		// 1. Scan extensions from repo
		exts := make(map[string]bool)
		_ = filepath.Walk(m.repoRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			// Skip hidden paths
			if strings.HasPrefix(info.Name(), ".") && info.Name() != ".env" {
				return nil
			}
			parts := strings.Split(path, string(os.PathSeparator))
			for _, p := range parts {
				if strings.HasPrefix(p, ".") && p != "." && p != ".." && p != ".env" {
					return nil
				}
			}
			ext := filepath.Ext(path)
			if ext != "" {
				exts[ext] = true
			}
			return nil
		})

		// 2. Map extensions to language names
		catalog := chunkers.NewLanguageCatalog()
		langMap := make(map[string]bool)
		for ext := range exts {
			if entry, ok := catalog.ExtensionToLanguage(ext); ok {
				langMap[entry.DomainName] = true
			}
		}

		var langs []string
		for l := range langMap {
			langs = append(langs, l)
		}
		sort.Strings(langs)

		if len(langs) == 0 {
			return grammarResultMsg{lang: "None detected", err: nil}
		}

		// 3. Ensure languages (downloads grammars)
		err := chunkers.EnsureLanguages(langs)
		return grammarResultMsg{lang: strings.Join(langs, ", "), err: err}
	}
}

func (m *InitScreen) applyAIResult(cfg *domain.ProjectConfig) {
	descFields := []components.DynamicField{{
		ID:          "description",
		Name:        "Description",
		Type:        components.DynFieldText,
		Value:       cfg.Description,
		Placeholder: "Enter a one-sentence description of your project...",
	}}
	m.descForm = components.NewDynamicFormModel(descFields, m.width)
}

func (m *InitScreen) handleSave() (tea.Model, tea.Cmd) {
	vals := m.descForm.Values()
	cfg := &domain.ProjectConfig{
		Description:  strings.TrimSpace(vals["description"]),
		BaseBranch:   strings.TrimSpace(m.baseBranchInput.Value()),
		TestCommand:  strings.TrimSpace(m.testCommandInput.Value()),
	}
	if err := cfg.Save(m.repoRoot); err != nil {
		m.err = err
		return m, nil
	}

	// Configure remote.origin.fetch to include refs/courer/*
	if m.git != nil {
		existing, getErr := m.git.ConfigGet("remote.origin.fetch")
		if getErr == nil && !strings.Contains(existing, "refs/courer/*") {
			if _, setErr := m.git.ConfigSet("remote.origin.fetch", "+refs/courer/*:refs/courer/*"); setErr != nil {
				log.Printf("[WARN] init: failed to set remote.origin.fetch refspec: %v", setErr)
			}
		}
	}

	m.confirmed = true
	m.step = stepFinish
	return m, nil
}

// View renders the init wizard — mirrors AppModel.View().
func (m InitScreen) View() string {
	var content string
	switch m.step {
	case stepWelcome:
		content = m.renderWelcome()
	case stepDescription:
		content = m.renderDescription()
	case stepBaseBranch:
		content = m.renderBaseBranch()
	case stepTestCommand:
		content = m.renderTestCommand()
	case stepGrammars:
		content = m.renderGrammars()
	case stepReview:
		content = m.renderReview()
	case stepFinish:
		content = m.renderFinish()
	case stepAIGenerating:
		content = m.renderAIGenerating()
	}
	return m.wrapScreen(content)
}

// wrapScreen centers content — mirrors AppModel.wrapScreen().
func (m InitScreen) wrapScreen(content string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// renderWelcome mirrors AppModel.renderWelcome().
// Shows a mode-selection menu when LLM is available, plain welcome otherwise.
func (m InitScreen) renderWelcome() string {
	header := lipgloss.JoinVertical(
		lipgloss.Center,
		styles.TitleStyle.Render("git-courer Project Init"),
		styles.SubtitleStyle.Render("Project Configuration Wizard"),
		"",
	)

	progress := styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step))

	var inner strings.Builder

	if m.err != nil {
		inner.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	if m.llm != nil {
		opts := []struct{ label string }{
			{"Manual Setup"},
			{"AI Setup (Ollama)"},
		}
		for i, o := range opts {
			style := styles.MenuItemStyle
			prefix := "  "
			if m.menuCursor == i {
				style = styles.SelectedMenuItemStyle
				prefix = "▸ "
			}
			inner.WriteString(style.Render(prefix+o.label) + "\n")
		}
		inner.WriteString("\n")
		inner.WriteString(styles.BoxHelpStyle.Render("up/down: navigate  enter: select  ctrl+c: quit"))
	} else {
		if m.hasConfig {
			inner.WriteString(styles.SuccessStyle.Render("✓ Existing configuration detected") + "\n\n")
			inner.WriteString(styles.BoxContentStyle.Render("Press ENTER to update your configuration.") + "\n\n")
		} else {
			inner.WriteString(styles.BoxContentStyle.Render("This wizard will help you configure your project.") + "\n\n")
			inner.WriteString(styles.BoxContentStyle.Render("Press ENTER to start the setup process.") + "\n\n")
		}
		inner.WriteString(styles.BoxHelpStyle.Render("enter: start  ctrl+c: quit"))
	}

	box := initBoxStyle.Render(inner.String())
	return lipgloss.JoinVertical(lipgloss.Center, header, progress, "", box)
}

// renderAIGenerating shows a spinner while ProjectInit runs in the background.
func (m InitScreen) renderAIGenerating() string {
	var s strings.Builder

	spinView := m.spin.View()
	content := styles.BoxHeaderStyle.Render("GENERATING WITH AI") + "\n\n" +
		spinView + "  " + styles.BoxContentStyle.Render("Reading project documentation") + "\n" +
		spinView + "  " + styles.BoxContentStyle.Render("Mapping directory structure") + "\n\n" +
		styles.BoxHelpStyle.Render("ctrl+c: cancel")

	s.WriteString(initBoxStyle.Render(content))
	return s.String()
}

// renderDescription mirrors AppModel.renderMCPCfg().
func (m InitScreen) renderDescription() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step)) + "\n\n")

	content := styles.BoxHeaderStyle.Render("PROJECT DESCRIPTION") + "\n\n" +
		styles.BoxContentStyle.Render("Enter a one-sentence description of your project.\n\n") +
		m.descForm.View() + "\n" +
		styles.BoxHelpStyle.Render("enter: next  ctrl+c: quit")

	s.WriteString(initBoxStyle.Render(content))
	return s.String()
}

// renderBaseBranch shows the base branch input step.
// The auto-detected default is pre-filled, but the user can change it or clear it.
func (m InitScreen) renderBaseBranch() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step)) + "\n\n")

	detectedHint := ""
	if m.git != nil {
		detected := domain.DetectBaseBranch(m.git)
		if detected != "" {
			detectedHint = styles.SubtextStyle.Render(fmt.Sprintf("(auto-detected: %s)", detected))
		}
	}

	content := styles.BoxHeaderStyle.Render("BASE BRANCH") + "\n\n" +
		styles.BoxContentStyle.Render("Which branch is your main/trunk branch? (e.g., main, develop)\n"+
			"Leave empty to try main/master/develop automatically.\n\n") +
		detectedHint + "\n" +
		m.baseBranchInput.View() + "\n\n" +
		styles.BoxHelpStyle.Render("enter: next  ctrl+c: quit")

	s.WriteString(initBoxStyle.Render(content))
	return s.String()
}

// renderTestCommand shows the test command input step.
func (m InitScreen) renderTestCommand() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step)) + "\n\n")

	content := styles.BoxHeaderStyle.Render("TEST COMMAND") + "\n\n" +
		styles.BoxContentStyle.Render("What command runs your tests?\n"+
			"This is used by the PR review tool to verify changes.\n\n") +
		m.testCommandInput.View() + "\n\n" +
		styles.BoxHelpStyle.Render("enter: next  ctrl+c: quit")

	s.WriteString(initBoxStyle.Render(content))
	return s.String()
}

func (m InitScreen) renderGrammars() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step)) + "\n\n")

	content := styles.BoxHeaderStyle.Render("PREPARING GRAMMARS") + "\n\n"

	if m.downloading {
		content += m.spin.View() + "  " + styles.BoxContentStyle.Render("Scanning repository for languages...") + "\n"
		content += styles.SubtextStyle.Render("Downloading required Tree-Sitter grammars.\n\n")
	} else {
		content += styles.SuccessStyle.Render("✓ Languages detected and grammars ready.") + "\n\n"
		if len(m.grammars) > 0 {
			for lang, ok := range m.grammars {
				mark := styles.SuccessStyle.Render("✓")
				if !ok {
					mark = styles.ErrorStyle.Render("✗")
				}
				content += fmt.Sprintf("  %s %s\n", mark, lang)
			}
			content += "\n"
		}
		content += "Press ENTER to continue to review.\n"
	}

	content += styles.BoxHelpStyle.Render("ctrl+c: cancel")

	s.WriteString(initBoxStyle.Render(content))
	return s.String()
}

// renderReview renders the review step of the install screen.
func (m InitScreen) renderReview() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step)) + "\n\n")

	vals := m.descForm.Values()
	desc := strings.TrimSpace(vals["description"])

	var inner strings.Builder
	if m.err != nil {
		inner.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}
	inner.WriteString(styles.BoxContentStyle.Render("Description:") + "\n")
	inner.WriteString("  " + desc + "\n\n")

	baseBranch := strings.TrimSpace(m.baseBranchInput.Value())
	if baseBranch != "" {
		inner.WriteString(styles.BoxContentStyle.Render("Base Branch:") + "\n")
		inner.WriteString("  " + baseBranch + "\n\n")
	} else {
		inner.WriteString(styles.BoxContentStyle.Render("Base Branch:") + "\n")
		inner.WriteString("  (auto-detect: main/master/develop)\n\n")
	}

	testCmd := strings.TrimSpace(m.testCommandInput.Value())
	if testCmd != "" {
		inner.WriteString(styles.BoxContentStyle.Render("Test Command:") + "\n")
		inner.WriteString("  " + testCmd + "\n\n")
	} else {
		inner.WriteString(styles.BoxContentStyle.Render("Test Command:") + "\n")
		inner.WriteString("  (none — PR review will skip tests)\n\n")
	}

	helpText := "enter: save  ctrl+c: cancel"
	if m.hasConfig {
		helpText = "enter: update  ctrl+c: cancel"
	}
	inner.WriteString(styles.BoxHelpStyle.Render(helpText))

	content := styles.BoxHeaderStyle.Render("REVIEW & SAVE") + "\n\n" + inner.String()
	s.WriteString(initBoxStyle.Render(content))
	return s.String()
}

// renderFinish mirrors AppModel.renderFinish().
func (m InitScreen) renderFinish() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step)) + "\n\n")
	content := styles.BoxHeaderStyle.Render("FINISH") + "\n\n" +
		styles.SuccessStyle.Render("✓ Configuration saved successfully!") + "\n\n" +
		styles.BoxHelpStyle.Render("enter: exit")
	s.WriteString(initBoxStyle.Render(content))
	return s.String()
}

// IsConfirmed returns true if the user confirmed the configuration.
func (m InitScreen) IsConfirmed() bool {
	return m.confirmed
}

// RunInitScreen creates and runs the InitScreen TUI program without AI support.
func RunInitScreen(repoRoot string, git ports.Git) error {
	model := NewInitScreen(80, repoRoot, git)
	p := tea.NewProgram(&model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunInitScreenWithLLM creates and runs the InitScreen TUI program with AI mode.
func RunInitScreenWithLLM(repoRoot string, llm ports.LLM, retry bool, git ports.Git) error {
	model := NewInitScreenWithLLM(80, repoRoot, llm, retry, git)
	p := tea.NewProgram(&model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
