// Package ports defines the interfaces (inward-facing) that the core depends on.
// Adapters implement these interfaces; the core never imports adapters directly.
package ports

// SecurityCheckResult holds the aggregated result of security checks.
type SecurityCheckResult struct {
	Blocked bool
	Files   []SecurityResult
}

// SecurityResult represents a single security check finding.
type SecurityResult struct {
	Halted  bool
	Reason  string
	File    string
	Line    int
	Type    string
	Message string
}

// SecurityService defines the interface for security checks.
// Implementations should provide blacklist, magic bytes, regex, and
// optionally LLM-based security scanning.
type SecurityService interface {
	// CheckFiles performs all security checks on the given files.
	// Returns a SecurityCheckResult with all findings.
	// If Blocked is true, the operation should abort immediately.
	CheckFiles(files []string, diff string) *SecurityCheckResult

	// ShouldUseLLMScan returns true if the underlying model
	// is large enough for LLM-based security scanning.
	ShouldUseLLMScan() bool
}

// IsBlocked returns true if any security check failed.
func (r *SecurityCheckResult) IsBlocked() bool {
	return r.Blocked
}

// FirstBlocking returns the first SecurityResult that was blocked,
// or nil if none.
func (r *SecurityCheckResult) FirstBlocking() *SecurityResult {
	for _, f := range r.Files {
		if f.Halted {
			return &f
		}
	}
	return nil
}
