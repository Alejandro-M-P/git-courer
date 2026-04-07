// Package domainerr provides domain-level error types and sentinel errors.
package errors

import "errors"

// Sentinel errors for common domain scenarios
var (
	ErrNotAGitRepo        = errors.New("not a git repository")
	ErrNothingToCommit    = errors.New("nothing to commit")
	ErrNoRemoteConfigured = errors.New("no remote configured")
	ErrOllamaNotAvailable = errors.New("Ollama is not available")
	ErrModelNotReady      = errors.New("model is not ready")
	ErrSecretsDetected    = errors.New("secrets detected in files")
	ErrInvalidJSON        = errors.New("invalid JSON response")
	ErrOperationCancelled = errors.New("operation cancelled by user")

	// Security errors
	ErrSecurityHalt    = errors.New("SECURITY_HALT: operation cancelled for security reasons")
	ErrBlacklistedFile = errors.New("blacklisted file detected")
	ErrBinaryFile      = errors.New("binary file detected")
	ErrSelfBinary      = errors.New("git-courer own binary detected")
)

// IsDomainError checks if an error is one of our domain sentinel errors
func IsDomainError(err error) bool {
	return errors.Is(err, ErrNotAGitRepo) ||
		errors.Is(err, ErrNothingToCommit) ||
		errors.Is(err, ErrNoRemoteConfigured) ||
		errors.Is(err, ErrOllamaNotAvailable) ||
		errors.Is(err, ErrModelNotReady) ||
		errors.Is(err, ErrSecretsDetected) ||
		errors.Is(err, ErrInvalidJSON) ||
		errors.Is(err, ErrOperationCancelled) ||
		errors.Is(err, ErrSecurityHalt) ||
		errors.Is(err, ErrBlacklistedFile) ||
		errors.Is(err, ErrBinaryFile) ||
		errors.Is(err, ErrSelfBinary)
}
