// Package styles provides theming and styling for the git-courer TUI.
package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette — Gentle AI inspired (Cyan & Gray)
var (
	Cyan         = lipgloss.Color("#00D4FF")
	Gray         = lipgloss.Color("#888888")
	White        = lipgloss.Color("#FFFFFF")
	WhiteBold    = lipgloss.Color("#FFFFFF")
	Black        = lipgloss.Color("#000000")
	SuccessGreen = lipgloss.Color("#00FF94")
	ErrorRed     = lipgloss.Color("#FF4C4C")
	Orange       = lipgloss.Color("#FFA500")
)

// TitleStyle is used for main titles - Cyan Bold
var TitleStyle = lipgloss.NewStyle().
	Foreground(Cyan).
	Bold(true)

// SubtitleStyle is used for subtitles - Gray Italic
var SubtitleStyle = lipgloss.NewStyle().
	Foreground(Gray).
	Italic(true)

// BoxStyle is the base style for all boxed content - Rounded Cyan
var BoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(Cyan).
	Padding(1, 2).
	Width(60)

// MenuItemStyle is the style for menu options in boxes - Normal Border
var MenuItemStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(Gray).
	Padding(0, 2).
	Width(30).
	Foreground(White)

// SelectedMenuItemStyle is for the currently selected menu option - Thick Cyan Border
var SelectedMenuItemStyle = MenuItemStyle.Copy().
	Border(lipgloss.ThickBorder()).
	BorderForeground(Cyan).
	Bold(true).
	Foreground(Cyan)

// TitleBoxStyle is for section titles in boxes
var TitleBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(Cyan).
	Width(60)

// Lipgloss styles

// SubtextStyle is used for secondary text - Gray
var SubtextStyle = lipgloss.NewStyle().
	Foreground(Gray)

// SelectedStyle is used for selected/focused items - Cyan
var SelectedStyle = lipgloss.NewStyle().
	Foreground(Cyan).
	Bold(true)

// UnselectedStyle is used for unselected/unfocused items - White
var UnselectedStyle = lipgloss.NewStyle().
	Foreground(White)

// Cursor is the cursor indicator for focused items - Cyan
var Cursor = lipgloss.NewStyle().
	Foreground(Cyan).
	SetString("▸")

// NoCursor is the indicator for unfocused items
var NoCursor = lipgloss.NewStyle().
	SetString("  ")

// HelpStyle is used for help text at the bottom - Gray
var HelpStyle = lipgloss.NewStyle().
	Foreground(Gray).
	Italic(true)

// ErrorStyle is used for error messages - Red
var ErrorStyle = lipgloss.NewStyle().
	Foreground(ErrorRed).
	Bold(true)

// WarningStyle is used for warning messages - Gray
var WarningStyle = lipgloss.NewStyle().
	Foreground(Gray)

// SuccessStyle is used for success messages - Green
var SuccessStyle = lipgloss.NewStyle().
	Foreground(SuccessGreen)

// Checked indicates an item is selected - Cyan
var Checked = lipgloss.NewStyle().
	Foreground(Cyan).
	SetString("✓")

// Detected indicates an item was detected (but not selected) - Gray
var Detected = lipgloss.NewStyle().
	Foreground(Gray).
	SetString("✗")

// CheckboxFocused is the cursor prefix for focused checkbox items - Cyan
var CheckboxFocused = lipgloss.NewStyle().
	Foreground(Cyan).
	SetString("▸ ")

// CheckboxUnfocused is the prefix for unfocused checkbox items
var CheckboxUnfocused = lipgloss.NewStyle().
	Foreground(Gray).
	SetString("  ")

// BoxHeaderStyle for section headers inside boxes - Cyan
var BoxHeaderStyle = lipgloss.NewStyle().
	Foreground(Cyan).
	Bold(true)

// BoxContentStyle for content inside boxes - White
var BoxContentStyle = lipgloss.NewStyle().
	Foreground(White)

// BoxItemStyle for centered items inside boxes - White
var BoxItemStyle = lipgloss.NewStyle().
	Foreground(White).
	Align(lipgloss.Center)

// BoxHelpStyle for help text inside boxes - Gray
var BoxHelpStyle = lipgloss.NewStyle().
	Foreground(Gray).
	Italic(true)