package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Domain errors for CommitEntry validation.
var (
	// ErrInvalidSHA is returned when a commit SHA is not a 40-character hex string.
	ErrInvalidSHA = errors.New("invalid commit SHA: must be 40-char hex string")
	// ErrEmptyMessage is returned when a commit message is empty or whitespace-only.
	ErrEmptyMessage = errors.New("commit message must not be empty")
)

// commitEntryConfig holds optional configuration for CommitEntry construction.
type commitEntryConfig struct {
	author string
	date   string
}

// CommitEntryOption is a functional option for CommitEntry construction.
type CommitEntryOption func(*commitEntryConfig)

// WithAuthor sets the author on a CommitEntry.
func WithAuthor(a string) CommitEntryOption {
	return func(c *commitEntryConfig) {
		c.author = a
	}
}

// WithDate sets the date (RFC 3339) on a CommitEntry.
func WithDate(d string) CommitEntryOption {
	return func(c *commitEntryConfig) {
		c.date = d
	}
}

// CommitEntry is a value object representing structured commit metadata.
// It is immutable after construction — all fields are accessed via getters.
// Construction validates SHA and Message; invalid input returns an error.
type CommitEntry struct {
	sha     string
	message string
	author  string
	date    string
}

// NewCommitEntry constructs a CommitEntry with validation.
// SHA must be exactly 40 hexadecimal characters.
// Message must be non-empty (after trimming whitespace).
// Author and Date are optional, set via CommitEntryOption.
func NewCommitEntry(sha, message string, opts ...CommitEntryOption) (CommitEntry, error) {
	if !isValidSHA(sha) {
		return CommitEntry{}, ErrInvalidSHA
	}
	if strings.TrimSpace(message) == "" {
		return CommitEntry{}, ErrEmptyMessage
	}

	cfg := commitEntryConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return CommitEntry{
		sha:     sha,
		message: message,
		author:  cfg.author,
		date:    cfg.date,
	}, nil
}

// isValidSHA checks that a string is exactly 40 hex characters.
func isValidSHA(sha string) bool {
	if len(sha) != 40 {
		return false
	}
	for _, r := range sha {
		if !isHex(r) {
			return false
		}
	}
	return true
}

// isHex checks if a rune is a valid hexadecimal digit.
func isHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// SHA returns the commit SHA.
func (e CommitEntry) SHA() string { return e.sha }

// Message returns the commit message.
func (e CommitEntry) Message() string { return e.message }

// Author returns the commit author (may be empty if not set).
func (e CommitEntry) Author() string { return e.author }

// Date returns the commit date in RFC 3339 format (may be empty if not set).
func (e CommitEntry) Date() string { return e.date }

// String returns a human-readable representation of the CommitEntry.
func (e CommitEntry) String() string {
	return fmt.Sprintf("%s %s", e.sha[:7], e.message)
}

// Messages extracts commit messages from a slice of CommitEntry values.
// Returns the messages in the same order as the input slice.
// If entries is nil, returns nil. If entries is empty, returns an empty slice.
func Messages(entries []CommitEntry) []string {
	if entries == nil {
		return nil
	}
	result := make([]string, len(entries))
	for i, entry := range entries {
		result[i] = entry.Message()
	}
	return result
}