package domain

// GitCourerError represents a structured error with context and suggestions for MCP tool responses.
type GitCourerError struct {
	Error      string `json:"error"`
	Context    string `json:"context"`
	Suggestion string `json:"suggestion"`
}

// NewError creates a new GitCourerError with the given message, context and suggestion.
func NewError(msg, context, suggestion string) GitCourerError {
	return GitCourerError{Error: msg, Context: context, Suggestion: suggestion}
}

// Common errors used throughout the application.
var (
	// ErrNotARepo is returned when a git operation is attempted outside a repository.
	ErrNotARepo = NewError(
		"not a git repository",
		"run git init or cd into a repo",
		"",
	)

	// ErrInvalidBranchName is returned when a branch name contains invalid characters.
	ErrInvalidBranchName = NewError(
		"invalid branch name",
		"branch name contains invalid characters",
		"use only a-z, A-Z, 0-9, ., _, /, and -",
	)

	// ErrNoChanges is returned when attempting to commit with nothing staged.
	ErrNoChanges = NewError(
		"no changes to commit",
		"working tree is clean or nothing is staged",
		"stage changes with git add or modify files first",
	)

	// ErrNoPendingPlan is returned when trying to apply or abort without a pending operation.
	ErrNoPendingPlan = NewError(
		"no pending operation",
		"there is no active plan to apply or abort",
		"run {OP}_START first to create a plan",
	)

	// ErrOperationFailed is returned when a git operation fails for an unspecified reason.
	ErrOperationFailed = NewError(
		"operation failed",
		"the git command returned an error",
		"check git status, permissions, and network connectivity",
	)
)
