package mcp

import (
	"strings"
)

// IntentType categorizes what the user wants to do
type IntentType int

const (
	IntentQuery       IntentType = iota // Read-only, no side effects
	IntentCreate                        // Create something (branch, tag, stash)
	IntentModify                        // Modify state (checkout, reset, clean)
	IntentWrite                         // Create commits (needs Ollama for message)
	IntentPassthrough                   // Generic git command, pass through directly
)

// Intent represents a parsed user intent
type Intent struct {
	Type        IntentType
	Action      string // e.g., "status", "commit", "log"
	Raw         string // Original instruction
	NeedsOllama bool   // Whether this needs Ollama for message generation
}

// IntentClassifier parses natural language into structured intents
type IntentClassifier struct{}

// NewIntentClassifier creates a new classifier
func NewIntentClassifier() *IntentClassifier {
	return &IntentClassifier{}
}

// Classify parses instruction into an Intent
func (c *IntentClassifier) Classify(instruction string) Intent {
	lower := strings.ToLower(strings.TrimSpace(instruction))
	raw := strings.TrimSpace(instruction)

	intent := Intent{
		Type:        IntentQuery, // Default to safest
		Action:      "unknown",
		Raw:         raw,
		NeedsOllama: false,
	}

	if lower == "" {
		return intent
	}

	// First check for explicit git commands (passthrough)
	if c.isGitCommand(lower) {
		intent.Type = IntentPassthrough
		intent.Action = c.extractGitCommand(lower)
		intent.NeedsOllama = false
		return intent
	}

	// Query operations (read-only, no side effects)
	if c.isQueryOp(lower) {
		intent.Type = IntentQuery
		intent.Action = c.extractQueryAction(lower)
		return intent
	}

	// Create operations
	if c.isCreateOp(lower, raw) {
		intent.Type = IntentCreate
		intent.Action = c.extractCreateAction(lower)
		return intent
	}

	// Modify operations
	if c.isModifyOp(lower, raw) {
		intent.Type = IntentModify
		intent.Action = c.extractModifyAction(lower)
		return intent
	}

	// Write operations (commits)
	if c.isWriteOp(lower) {
		intent.Type = IntentWrite
		intent.Action = "commit"
		intent.NeedsOllama = true
		return intent
	}

	// Default: passthrough as git command
	intent.Type = IntentPassthrough
	intent.Action = raw
	intent.NeedsOllama = false
	return intent
}

// isGitCommand checks if instruction looks like a git command
func (c *IntentClassifier) isGitCommand(lower string) bool {
	gitCommands := []string{
		"log", "diff", "status", "branch", "checkout", "commit", "push", "pull",
		"fetch", "merge", "rebase", "reset", "clean", "stash", "tag", "blame",
		"grep", "show", "ls-files", "cat-file", "rev-parse", "show-ref",
		"for-each-ref", "describe", "diff-tree", "hash-object", "update-index",
		"write-tree", "commit-tree", "rev-list", "name-rev", "cherry", "cherry-pick",
		"revert", "bisect", "worktree", "reflog", "archive", "bundle", "fsck",
		"gc", "prune", "count-objects", "verify-pack", "ls-tree",
	}
	for _, cmd := range gitCommands {
		if strings.HasPrefix(lower, cmd+" ") || lower == cmd {
			return true
		}
	}
	return false
}

// extractGitCommand extracts the git command portion
func (c *IntentClassifier) extractGitCommand(lower string) string {
	parts := strings.SplitN(lower, " ", 2)
	if len(parts) >= 2 {
		return parts[1]
	}
	return lower
}

// isQueryOp checks if operation is read-only query
func (c *IntentClassifier) isQueryOp(lower string) bool {
	queryPatterns := []string{
		"status", "estado", "log", "historial", "history", "recent commits",
		"commits", "diff", "cambios", "branches", "branch", "ramas",
		"remote", "remotes", "stash list", "list stash", "show stash",
		"ls-files", "list files", "files", "show files", "blame", "grep",
		"show head", "show ref", "reflog", "describe", "worktree",
		"archive", "bundle", "fsck",
	}

	for _, pattern := range queryPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	bareQueries := []string{"status", "log", "diff", "blame", "grep", "reflog", "fsck"}
	for _, q := range bareQueries {
		if lower == q {
			return true
		}
	}

	return false
}

// extractQueryAction extracts the query action type
func (c *IntentClassifier) extractQueryAction(lower string) string {
	if strings.Contains(lower, "status") {
		return "status"
	}
	if strings.Contains(lower, "log") || strings.Contains(lower, "history") ||
		strings.Contains(lower, "commits") || strings.Contains(lower, "historial") {
		return "log"
	}
	if strings.Contains(lower, "diff") || strings.Contains(lower, "cambios") {
		return "diff"
	}
	if strings.Contains(lower, "branch") || strings.Contains(lower, "branches") {
		return "branches"
	}
	if strings.Contains(lower, "remote") {
		return "remotes"
	}
	if strings.Contains(lower, "blame") {
		return "blame"
	}
	if strings.Contains(lower, "grep") {
		return "grep"
	}
	if strings.Contains(lower, "reflog") {
		return "reflog"
	}
	return "status"
}

// isCreateOp checks if operation creates something
func (c *IntentClassifier) isCreateOp(lower string, raw string) bool {
	createPatterns := []string{
		"create branch", "new branch", "make branch", "crea branch",
		"crear branch", "crear rama", "nueva rama", "checkout -b",
		"tag", "create tag", "new tag", "make tag", "version", "release",
		"stash",
	}

	for _, pattern := range createPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// extractCreateAction extracts the create action type
func (c *IntentClassifier) extractCreateAction(lower string) string {
	if strings.Contains(lower, "checkout -b") || strings.Contains(lower, "branch") {
		return "create-branch"
	}
	if strings.Contains(lower, "tag") || strings.Contains(lower, "version") {
		return "create-tag"
	}
	if strings.Contains(lower, "stash") {
		return "stash"
	}
	return "create"
}

// isModifyOp checks if operation modifies state
func (c *IntentClassifier) isModifyOp(lower string, raw string) bool {
	modifyPatterns := []string{
		"checkout", "switch to", "cambia a", "ir a",
		"reset", "restaurar",
		"clean", "limpiar",
		"merge", "fusionar",
		"rebase",
		"fetch",
		"revert", "revertir",
		"add ", "stage ", "agregar",
	}

	for _, pattern := range modifyPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// extractModifyAction extracts the modify action type
func (c *IntentClassifier) extractModifyAction(lower string) string {
	if strings.Contains(lower, "checkout") && !strings.Contains(lower, "-b") {
		return "checkout"
	}
	if strings.Contains(lower, "reset") {
		return "reset"
	}
	if strings.Contains(lower, "clean") {
		return "clean"
	}
	if strings.Contains(lower, "merge") {
		return "merge"
	}
	if strings.Contains(lower, "rebase") {
		return "rebase"
	}
	if strings.Contains(lower, "fetch") {
		return "fetch"
	}
	if strings.Contains(lower, "revert") {
		return "revert"
	}
	if strings.Contains(lower, "add ") || strings.Contains(lower, "stage ") {
		return "add"
	}
	return "modify"
}

// isWriteOp checks if operation creates commits
func (c *IntentClassifier) isWriteOp(lower string) bool {
	writePatterns := []string{
		"commit", "commitea", "commiteame", "guarda", "guardar",
		"commit -m",
	}

	for _, pattern := range writePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}
