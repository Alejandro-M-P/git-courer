package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/tui/styles"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// initModelState represents the current state of the init TUI.
type initModelState int

const (
	stateInitLoading initModelState = iota
	stateInitReview
	stateInitConfirm
	stateInitDone
)

// focusTarget identifies which field has focus in review state.
type focusTarget int

const (
	focusDescription focusTarget = iota
	focusAreas
	focusConfirm
	focusCancel
)

// projectInitResultMsg is sent when LLM analysis completes.
type projectInitResultMsg struct {
	config *domain.ProjectConfig
}

// projectInitErrorMsg is sent when LLM analysis fails.
type projectInitErrorMsg struct {
	err error
}

// InitModel is the Bubbletea model for `git-courer init --tui`.
// States: loading → review/edit → confirm → done
type InitModel struct {
	state            initModelState
	config           *domain.ProjectConfig
	repoRoot         string
	llmAdapter       LLMInitClient
	descriptionInput textinput.Model
	areaInputs       []areaInput
	focusIndex       focusTarget
	width            int
	height           int
	done             bool
	err              error
}

// LLMInitClient is the minimal interface for the LLM operations needed by the init TUI.
// ports.LLM naturally satisfies this interface, so no wrapper is needed.
type LLMInitClient interface {
	IsAvailable() bool
	ProjectInit(repoRoot string) (*domain.ProjectConfig, error)
}

// areaInput holds the editable fields for a single area entry.
type areaInput struct {
	nameInput textinput.Model
	pathsInput textinput.Model
}

// NewInitModel creates a new InitModel.
// If config is nil and llmAdapter is available, the model starts in loading state
// and calls ProjectInit. If config is provided, it starts in review state.
// If llmAdapter is nil/unavailable, it starts in review with empty fields.
func NewInitModel(width, height int, repoRoot string, config *domain.ProjectConfig, llmAdapter LLMInitClient) InitModel {
	descInput := textinput.New()
	descInput.Placeholder = "Enter project description..."
	descInput.CharLimit = 200
	descInput.Width = 40

	m := InitModel{
		state:          stateInitLoading,
		config:         config,
		repoRoot:       repoRoot,
		llmAdapter:     llmAdapter,
		descriptionInput: descInput,
		focusIndex:     focusDescription,
		width:          width,
		height:         height,
	}

	if config != nil {
		// Pre-populate from provided config (LLM result or manual form)
		m.descriptionInput.SetValue(config.Description)
		m.areaInputs = buildAreaInputs(config.Areas)
		m.state = stateInitReview
	} else if llmAdapter == nil || !llmAdapter.IsAvailable() {
		// No LLM → empty form for manual entry
		m.config = &domain.ProjectConfig{Areas: make(map[string][]string)}
		m.areaInputs = []areaInput{newAreaInput("", "")}
		m.state = stateInitReview
	}

	return m
}

// buildAreaInputs converts an areas map into editable areaInput pairs.
func buildAreaInputs(areas map[string][]string) []areaInput {
	// Sort area names for deterministic order
	keys := make([]string, 0, len(areas))
	for k := range areas {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	inputs := make([]areaInput, 0, len(keys))
	for _, k := range keys {
		paths := strings.Join(areas[k], ", ")
		inputs = append(inputs, newAreaInput(k, paths))
	}
	return inputs
}

func newAreaInput(name, paths string) areaInput {
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

	return areaInput{
		nameInput:  nameInput,
		pathsInput: pathsInput,
	}
}

// setState sets the model state (used for testing).
func (m *InitModel) setState(s initModelState) {
	m.state = s
	if s == stateInitReview {
		m.focusDescription()
	}
}

// Init initializes the TUI model.
func (m InitModel) Init() tea.Cmd {
	if m.state == stateInitLoading && m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
		return func() tea.Msg {
			config, err := m.llmAdapter.ProjectInit(m.repoRoot)
			if err != nil {
				return projectInitErrorMsg{err: err}
			}
			return projectInitResultMsg{config: config}
		}
	}
	return nil
}

// Update handles messages for the init model.
func (m InitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectInitResultMsg:
		m.config = msg.config
		m.descriptionInput.SetValue(msg.config.Description)
		m.areaInputs = buildAreaInputs(msg.config.Areas)
		m.state = stateInitReview
		m.focusDescription()
		return m, nil

	case projectInitErrorMsg:
		// LLM failed → switch to manual empty form
		m.config = &domain.ProjectConfig{Areas: make(map[string][]string)}
		m.areaInputs = []areaInput{newAreaInput("", "")}
		m.state = stateInitReview
		m.err = msg.err
		m.focusDescription()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.state == stateInitReview {
				m.done = true
				return m, tea.Quit
			}
			return m, tea.Quit

		case "tab", "down":
			if m.state == stateInitReview {
				return m.handleTab()
			}
		case "shift+tab", "up":
			if m.state == stateInitReview {
				return m.handleShiftTab()
			}
		case "enter":
			return m.handleEnter()
		case "n":
			if m.state == stateInitConfirm {
				// 'n' on confirm = cancel
				m.done = true
				return m, tea.Quit
			}
			if m.state == stateInitReview && m.focusIndex == focusCancel {
				m.done = true
				return m, tea.Quit
			}
		case "y":
			if m.state == stateInitConfirm {
				return m.handleConfirm()
			}
		}
	}

	// Forward key messages to focused input in review state
	if m.state == stateInitReview {
		return m.updateInputs(msg)
	}

	return m, nil
}

func (m InitModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case stateInitReview:
		// Build config from current inputs and move to confirm
		m.syncInputsToConfig()
		m.state = stateInitConfirm
		m.focusIndex = focusConfirm
		m.descriptionInput.Blur()
		return m, nil
	case stateInitConfirm:
		return m.handleConfirm()
	}
	return m, nil
}

func (m InitModel) handleConfirm() (tea.Model, tea.Cmd) {
	if m.config == nil {
		m.syncInputsToConfig()
	}
	if err := m.config.Save(m.repoRoot); err != nil {
		m.err = err
		return m, nil
	}
	m.done = true
	return m, tea.Quit
}

func (m InitModel) handleTab() (tea.Model, tea.Cmd) {
	oldFocus := m.focusIndex
	m.blurAll()

	// Calculate total focusable items: description + (each area has 2 fields) + confirm + cancel
	totalFocusable := 1 + len(m.areaInputs)*2 + 2
	newFocus := int(oldFocus) + 1
	if newFocus >= totalFocusable {
		newFocus = 0
	}
	m.focusIndex = focusTarget(newFocus)
	m.focusCurrentInput()
	return m, nil
}

func (m InitModel) handleShiftTab() (tea.Model, tea.Cmd) {
	oldFocus := m.focusIndex
	m.blurAll()

	totalFocusable := 1 + len(m.areaInputs)*2 + 2
	newFocus := int(oldFocus) - 1
	if newFocus < 0 {
		newFocus = totalFocusable - 1
	}
	m.focusIndex = focusTarget(newFocus)
	m.focusCurrentInput()
	return m, nil
}

func (m *InitModel) blurAll() {
	m.descriptionInput.Blur()
	for i := range m.areaInputs {
		m.areaInputs[i].nameInput.Blur()
		m.areaInputs[i].pathsInput.Blur()
	}
}

func (m *InitModel) focusCurrentInput() {
	switch m.focusIndex {
	case focusDescription:
		m.descriptionInput.Focus()
	default:
		// Check if focus is on an area field
		areaIdx, isName := m.areaFocusTarget(m.focusIndex)
		if areaIdx >= 0 && areaIdx < len(m.areaInputs) {
			if isName {
				m.areaInputs[areaIdx].nameInput.Focus()
			} else {
				m.areaInputs[areaIdx].pathsInput.Focus()
			}
		}
	}
}

// areaFocusTarget returns (areaIndex, isNameField) for a given focusIndex.
// Returns (-1, false) if the focusIndex is not an area field.
func (m *InitModel) areaFocusTarget(idx focusTarget) (int, bool) {
	adjusted := int(idx) - 1 // subtract description field
	if adjusted < 0 || adjusted >= len(m.areaInputs)*2 {
		return -1, false
	}
	areaIdx := adjusted / 2
	isName := adjusted%2 == 0
	return areaIdx, isName
}

func (m *InitModel) focusDescription() {
	m.blurAll()
	m.focusIndex = focusDescription
	m.descriptionInput.Focus()
}

func (m InitModel) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.focusIndex {
	case focusDescription:
		m.descriptionInput, cmd = m.descriptionInput.Update(msg)
	default:
		areaIdx, isName := m.areaFocusTarget(m.focusIndex)
		if areaIdx >= 0 && areaIdx < len(m.areaInputs) {
			if isName {
				m.areaInputs[areaIdx].nameInput, cmd = m.areaInputs[areaIdx].nameInput.Update(msg)
			} else {
				m.areaInputs[areaIdx].pathsInput, cmd = m.areaInputs[areaIdx].pathsInput.Update(msg)
			}
		}
	}

	return m, cmd
}

// syncInputsToConfig reads current input values and builds the ProjectConfig.
func (m *InitModel) syncInputsToConfig() {
	areas := make(map[string][]string)
	for _, ai := range m.areaInputs {
		name := strings.TrimSpace(ai.nameInput.Value())
		pathsStr := strings.TrimSpace(ai.pathsInput.Value())
		if name == "" {
			continue
		}
		paths := strings.Split(pathsStr, ",")
		for i := range paths {
			paths[i] = strings.TrimSpace(paths[i])
		}
		// Filter empty paths
		filtered := make([]string, 0, len(paths))
		for _, p := range paths {
			if p != "" {
				filtered = append(filtered, p)
			}
		}
		areas[name] = filtered
	}

	m.config = &domain.ProjectConfig{
		Description: strings.TrimSpace(m.descriptionInput.Value()),
		Areas:       areas,
	}
}

// View renders the init TUI.
func (m InitModel) View() string {
	switch m.state {
	case stateInitLoading:
		return m.viewLoading()
	case stateInitReview:
		return m.viewReview()
	case stateInitConfirm:
		return m.viewConfirm()
	case stateInitDone:
		return m.viewDone()
	}
	return ""
}

func (m InitModel) viewLoading() string {
	var s strings.Builder
	header := styles.BoxHeaderStyle.Render("PROJECT INIT") + "\n\n"
	s.WriteString(header)
	s.WriteString(styles.SelectedStyle.Render("⠋ Analyzing project structure with AI...") + "\n\n")
	s.WriteString(styles.SubtextStyle.Render("Please wait while the LLM analyzes your codebase.") + "\n\n")
	s.WriteString(styles.BoxHelpStyle.Render("ctrl+c: cancel"))

	return styles.BoxStyle.Render(s.String())
}

func (m InitModel) viewReview() string {
	var s strings.Builder
	header := styles.BoxHeaderStyle.Render("PROJECT INIT") + "\n\n"

	if m.err != nil {
		s.WriteString(header)
		s.WriteString(styles.WarningStyle.Render(fmt.Sprintf("AI analysis failed: %v\nFalling back to manual entry.\n\n", m.err)))
	} else {
		s.WriteString(header)
		s.WriteString(styles.SubtextStyle.Render("Review and edit the project configuration:\n\n"))
	}

	// Description field
	cursor := "  "
	if m.focusIndex == focusDescription {
		cursor = styles.Cursor.Render()
	}
	descLabel := styles.BoxContentStyle.Render("Description:")
	if m.focusIndex == focusDescription {
		descLabel = styles.SelectedStyle.Render("Description:")
	}
	s.WriteString(cursor + descLabel + " " + m.descriptionInput.View() + "\n\n")

	// Area fields
	s.WriteString(styles.BoxContentStyle.Render("Areas:") + "\n")
	for i, ai := range m.areaInputs {
		nameCursor := "  "
		pathsCursor := "  "
		if m.focusIndex == focusTarget(1+i*2) {
			nameCursor = styles.Cursor.Render()
		}
		if m.focusIndex == focusTarget(1+i*2+1) {
			pathsCursor = styles.Cursor.Render()
		}

		nameLabel := styles.BoxContentStyle.Render(fmt.Sprintf("  %d. Name:", i+1))
		if m.focusIndex == focusTarget(1+i*2) {
			nameLabel = styles.SelectedStyle.Render(fmt.Sprintf("  %d. Name:", i+1))
		}
		pathsLabel := styles.BoxContentStyle.Render("Paths:")
		if m.focusIndex == focusTarget(1+i*2+1) {
			pathsLabel = styles.SelectedStyle.Render("Paths:")
		}

		s.WriteString(nameCursor + nameLabel + " " + ai.nameInput.View() + "\n")
		s.WriteString(pathsCursor + "     " + pathsLabel + " " + ai.pathsInput.View() + "\n")
	}

	s.WriteString("\n")

	// Confirm / Cancel buttons
	confirmStyle := styles.BoxContentStyle
	cancelStyle := styles.BoxContentStyle
	totalFocusable := 1 + len(m.areaInputs)*2 + 2
	confirmFocusIdx := focusTarget(totalFocusable - 2)
	cancelFocusIdx := focusTarget(totalFocusable - 1)

	if m.focusIndex == confirmFocusIdx {
		confirmStyle = styles.SelectedStyle
	}
	if m.focusIndex == cancelFocusIdx {
		cancelStyle = styles.SelectedStyle
	}

	s.WriteString(confirmStyle.Render("  [Save & Continue]  "))
	s.WriteString(cancelStyle.Render("  [Cancel]  ") + "\n\n")

	s.WriteString(styles.BoxHelpStyle.Render("tab: next field  enter: save  esc: cancel"))

	return styles.BoxStyle.Render(s.String())
}

func (m InitModel) viewConfirm() string {
	var s strings.Builder
	header := styles.BoxHeaderStyle.Render("CONFIRM SAVE") + "\n\n"
	s.WriteString(header)

	s.WriteString(styles.BoxContentStyle.Render("Save this configuration?\n\n"))
	s.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("Description: %s\n", m.config.Description)))
	if len(m.config.Areas) > 0 {
		s.WriteString(styles.SubtextStyle.Render("Areas:\n"))
		keys := make([]string, 0, len(m.config.Areas))
		for k := range m.config.Areas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("  %s: %s\n", k, strings.Join(m.config.Areas[k], ", "))))
		}
	}
	s.WriteString("\n")

	yStyle := styles.BoxContentStyle
	nStyle := styles.BoxContentStyle
	if m.focusIndex == focusConfirm {
		yStyle = styles.SelectedStyle
	}
	if m.focusIndex == focusCancel {
		nStyle = styles.SelectedStyle
	}
	s.WriteString(yStyle.Render("  [y] Yes  "))
	s.WriteString(nStyle.Render("  [n] No  ") + "\n\n")
	s.WriteString(styles.BoxHelpStyle.Render("y: confirm  n: cancel  ctrl+c: quit"))

	return styles.BoxStyle.Render(s.String())
}

func (m InitModel) viewDone() string {
	var s strings.Builder
	header := styles.BoxHeaderStyle.Render("PROJECT INIT") + "\n\n"
	s.WriteString(header)

	if m.err != nil {
		s.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Error: %v\n", m.err)))
	} else {
		s.WriteString(styles.SuccessStyle.Render("✓ Project configuration saved!") + "\n\n")
		configPath := fmt.Sprintf("%s/.git-courer/config.json", m.repoRoot)
		s.WriteString(styles.BoxContentStyle.Render(fmt.Sprintf("Saved to: %s\n", configPath)))
		s.WriteString(styles.BoxHelpStyle.Render("enter: exit"))
	}

	return styles.BoxStyle.Render(s.String())
}

// RunInit creates and runs the project init TUI.
// This is the entry point called from cmd/main.go when --tui flag is set.
func RunInit(repoRoot string, llmAdapter LLMInitClient) error {
	model := NewInitModel(80, 24, repoRoot, nil, llmAdapter)
	p := tea.NewProgram(
		&model,
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}