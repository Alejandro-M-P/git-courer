// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/tui/components"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// initBoxStyle is a wider variant of BoxStyle for the init wizard.
// The areas help text and two-column inputs need more room than the default 60-char box.
var initBoxStyle = styles.BoxStyle.Copy().Width(76)

const (
	stepWelcome      = 0
	stepDescription  = 1
	stepAreas        = 2
	stepGrammars     = 3
	stepReview       = 4
	stepFinish       = 5
	stepAIGenerating = 6

	progressSteps = "Welcome,Description,Areas,Grammars,Review,Finish"
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

type areaEntry struct {
	nameInput  textinput.Model
	pathsInput textinput.Model
}

// InitScreen represents the project init wizard model.
type InitScreen struct {
	step       int
	width      int
	height     int
	repoRoot   string
	hasConfig  bool
	descForm   components.DynamicFormModel
	areas      []areaEntry
	areaFocus  int
	err        error
	confirmed  bool
	llm        ports.LLM
	menuCursor int
	spin       spinner.Model
	downloading bool
	grammars    map[string]bool // lang -> success
}

// NewInitScreen creates an init wizard without AI support.
func NewInitScreen(width int, repoRoot string) InitScreen {
	return newInitScreen(width, repoRoot, nil, false)
}

// NewInitScreenWithLLM creates an init wizard with AI mode enabled.
func NewInitScreenWithLLM(width int, repoRoot string, llm ports.LLM, retry bool) InitScreen {
	return newInitScreen(width, repoRoot, llm, retry)
}

func newInitScreen(width int, repoRoot string, llm ports.LLM, retry bool) InitScreen {
	hasConfig := false
	var existingDesc string
	var existingAreas map[string][]string

	if cfg, err := domain.LoadProjectConfig(repoRoot); err == nil && cfg != nil {
		hasConfig = true
		existingDesc = cfg.Description
		existingAreas = cfg.Areas
	}

	descFields := []components.DynamicField{{
		ID:          "description",
		Name:        "Description",
		Type:        components.DynFieldText,
		Value:       existingDesc,
		Placeholder: "Enter a one-sentence description of your project...",
	}}
	descForm := components.NewDynamicFormModel(descFields, width)

	areas := buildAreasFromMap(existingAreas)
	if len(areas) == 0 {
		areas = []areaEntry{newAreaInput("", "")}
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Cyan)

	return InitScreen{
		step:      stepWelcome,
		width:     width,
		height:    0,
		repoRoot:  repoRoot,
		hasConfig: hasConfig,
		descForm:  descForm,
		areas:     areas,
		areaFocus: 0,
		llm:       llm,
		spin:      s,
	}
}

func buildAreasFromMap(m map[string][]string) []areaEntry {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	areas := make([]areaEntry, 0, len(keys))
	for _, k := range keys {
		areas = append(areas, newAreaInput(k, strings.Join(m[k], ", ")))
	}
	return areas
}

func newAreaInput(name, paths string) areaEntry {
	nameInput := textinput.New()
	nameInput.Placeholder = "area_name"
	nameInput.CharLimit = 50
	nameInput.Width = 15
	nameInput.SetValue(name)

	pathsInput := textinput.New()
	pathsInput.Placeholder = "path/prefix/"
	pathsInput.CharLimit = 200
	pathsInput.Width = 30
	pathsInput.SetValue(paths)

	return areaEntry{nameInput: nameInput, pathsInput: pathsInput}
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

		case "a":
			if m.step == stepAreas {
				m.areas = append(m.areas, newAreaInput("", ""))
				m.areaFocus = len(m.areas) - 1
			}
			return m, nil

		case "d":
			if m.step == stepAreas && len(m.areas) > 1 {
				idx := m.areaFocus
				m.areas = append(m.areas[:idx], m.areas[idx+1:]...)
				if m.areaFocus >= len(m.areas) {
					m.areaFocus = len(m.areas) - 1
				}
			}
			return m, nil

		case "up":
			if m.step == stepWelcome && m.llm != nil {
				m.menuCursor--
				if m.menuCursor < 0 {
					m.menuCursor = 1
				}
			} else if m.step == stepAreas && len(m.areas) > 1 {
				m.areaFocus--
				if m.areaFocus < 0 {
					m.areaFocus = len(m.areas) - 1
				}
			}
			return m, nil

		case "down":
			if m.step == stepWelcome && m.llm != nil {
				m.menuCursor++
				if m.menuCursor > 1 {
					m.menuCursor = 0
				}
			} else if m.step == stepAreas && len(m.areas) > 1 {
				m.areaFocus++
				if m.areaFocus >= len(m.areas) {
					m.areaFocus = 0
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

	if m.step == stepAreas {
		var cmd tea.Cmd
		if m.areaFocus >= 0 && m.areaFocus < len(m.areas) {
			m.areas[m.areaFocus].nameInput, cmd = m.areas[m.areaFocus].nameInput.Update(msg)
			m.areas[m.areaFocus].pathsInput, cmd = m.areas[m.areaFocus].pathsInput.Update(msg)
			return m, cmd
		}
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
		m.step = stepAreas
		if len(m.areas) > 0 {
			m.areas[0].nameInput.Focus()
		}
	case stepAreas:
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

	areas := buildAreasFromMap(cfg.Areas)
	if len(areas) == 0 {
		areas = []areaEntry{newAreaInput("", "")}
	}
	m.areas = areas
	m.areaFocus = 0
}

func (m *InitScreen) handleSave() (tea.Model, tea.Cmd) {
	vals := m.descForm.Values()
	cfg := &domain.ProjectConfig{
		Description: strings.TrimSpace(vals["description"]),
		Areas:       m.buildAreasMap(),
	}
	if err := cfg.Save(m.repoRoot); err != nil {
		m.err = err
		return m, nil
	}
	m.confirmed = true
	m.step = stepFinish
	return m, nil
}

func (m InitScreen) buildAreasMap() map[string][]string {
	areaMap := make(map[string][]string)
	for _, entry := range m.areas {
		name := strings.TrimSpace(entry.nameInput.Value())
		if name == "" {
			continue
		}
		paths := strings.Split(strings.TrimSpace(entry.pathsInput.Value()), ",")
		filtered := make([]string, 0, len(paths))
		for _, p := range paths {
			if p = strings.TrimSpace(p); p != "" {
				filtered = append(filtered, p)
			}
		}
		areaMap[name] = filtered
	}
	return areaMap
}

// View renders the init wizard — mirrors AppModel.View().
func (m InitScreen) View() string {
	var content string
	switch m.step {
	case stepWelcome:
		content = m.renderWelcome()
	case stepDescription:
		content = m.renderDescription()
	case stepAreas:
		content = m.renderAreas()
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

// renderAreas mirrors AppModel.renderMCPCfg() list pattern.
func (m InitScreen) renderAreas() string {
	var s strings.Builder
	s.WriteString(styles.SubtextStyle.Render(components.RenderProgress(progressStepsList(), m.step)) + "\n\n")

	var areaList strings.Builder
	for i, entry := range m.areas {
		cursor := styles.CheckboxUnfocused.Render()
		if i == m.areaFocus {
			cursor = styles.CheckboxFocused.Render()
		}
		areaList.WriteString(cursor)
		areaList.WriteString(entry.nameInput.View())
		areaList.WriteString("  ")
		areaList.WriteString(entry.pathsInput.View())
		areaList.WriteString("\n")
	}

	content := styles.BoxHeaderStyle.Render("PROJECT AREAS") + "\n\n" +
		styles.BoxContentStyle.Render("Define functional areas. Format: name → path/prefix/\n\n") +
		areaList.String() + "\n" +
		styles.BoxHelpStyle.Render("a: add  d: delete  up/down: navigate  enter: next  ctrl+c: quit")

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
	inner.WriteString(styles.BoxContentStyle.Render("Areas:") + "\n")

	areaMap := m.buildAreasMap()
	if len(areaMap) == 0 {
		inner.WriteString("  (none)\n")
	} else {
		keys := make([]string, 0, len(areaMap))
		for k := range areaMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			inner.WriteString(fmt.Sprintf("  %s: %s\n", k, strings.Join(areaMap[k], ", ")))
		}
	}
	inner.WriteString("\n")

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
func RunInitScreen(repoRoot string) error {
	model := NewInitScreen(80, repoRoot)
	p := tea.NewProgram(&model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunInitScreenWithLLM creates and runs the InitScreen TUI program with AI mode.
func RunInitScreenWithLLM(repoRoot string, llm ports.LLM, retry bool) error {
	model := NewInitScreenWithLLM(80, repoRoot, llm, retry)
	p := tea.NewProgram(&model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
