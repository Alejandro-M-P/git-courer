// Package styles provides theming and styling for the git-courer TUI.
package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette — amber/cyan dark theme (gentle-ai inspired)
var (
	// Amber tones (primary accent)
	AmberBold    = lipgloss.Color("#FFB000")
	Amber        = lipgloss.Color("#FFA500")
	AmberMuted   = lipgloss.Color("#CC8800")
	AmberDark    = lipgloss.Color("#805500")

	// Cyan tones (secondary accent)
	CyanBold     = lipgloss.Color("#00D4FF")
	Cyan         = lipgloss.Color("#00BFFF")
	CyanMuted    = lipgloss.Color("#0099CC")
	CyanDark     = lipgloss.Color("#006699")

	// Neutral tones
	White        = lipgloss.Color("#FFFFFF")
	Gray         = lipgloss.Color("#888888")
	GrayMuted    = lipgloss.Color("#555555")
	GrayDark     = lipgloss.Color("#333333")
	Black        = lipgloss.Color("#000000")

	// Semantic colors
	ErrorRed     = lipgloss.Color("#FF4444")
	WarningYellow = lipgloss.Color("#FFD700")
	SuccessGreen  = lipgloss.Color("#44FF44")
)

// Lipgloss styles

// TitleStyle is used for main titles
var TitleStyle = lipgloss.NewStyle().
	Foreground(AmberBold).
	Bold(true)

// SubtextStyle is used for secondary text
var SubtextStyle = lipgloss.NewStyle().
	Foreground(Gray)

// SelectedStyle is used for selected/focused items
var SelectedStyle = lipgloss.NewStyle().
	Foreground(CyanBold).
	Bold(true)

// UnselectedStyle is used for unselected/unfocused items
var UnselectedStyle = lipgloss.NewStyle().
	Foreground(Gray)

// Cursor is the cursor indicator for focused items
var Cursor = lipgloss.NewStyle().
	Foreground(CyanBold).
	SetString("▸")

// NoCursor is the indicator for unfocused items
var NoCursor = lipgloss.NewStyle().
	SetString("  ")

// HelpStyle is used for help text at the bottom
var HelpStyle = lipgloss.NewStyle().
	Foreground(GrayMuted).
	Italic(true)

// ErrorStyle is used for error messages
var ErrorStyle = lipgloss.NewStyle().
	Foreground(ErrorRed).
	Bold(true)

// WarningStyle is used for warning messages
var WarningStyle = lipgloss.NewStyle().
	Foreground(WarningYellow)

// SuccessStyle is used for success messages
var SuccessStyle = lipgloss.NewStyle().
	Foreground(SuccessGreen)

// Checked indicates an item is selected
var Checked = lipgloss.NewStyle().
	Foreground(SuccessGreen).
	SetString("✓")

// Detected indicates an item was detected (but not selected)
var Detected = lipgloss.NewStyle().
	Foreground(AmberMuted).
	SetString("✗")

// CheckboxFocused is the cursor prefix for focused checkbox items
var CheckboxFocused = lipgloss.NewStyle().
	Foreground(CyanBold).
	SetString("▸ ")

// CheckboxUnfocused is the prefix for unfocused checkbox items
var CheckboxUnfocused = lipgloss.NewStyle().
	Foreground(GrayDark).
	SetString("  ")