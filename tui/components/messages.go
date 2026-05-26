// Package components provides reusable TUI components for the git-courer wizard.
package components

// ResolvedContextMsg carries the result of context window resolution.
// Used by both AppModel (tui/model.go) and InstallScreen (tui/screens/install.go).
type ResolvedContextMsg struct {
	Ctx int
	Err error
}