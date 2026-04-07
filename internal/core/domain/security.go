package domain

// SecurityResult holds the result of a security check.
// It indicates whether the operation should proceed or be halted.
type SecurityResult struct {
	Safe    bool
	Halted  bool
	Reason  SecurityReason
	File    string
	Line    int
	Type    string
	Message string
}

// SecurityReason categorizes why a check failed.
type SecurityReason string

const (
	// ReasonBlacklistedFile indicates a file is on the hard blacklist
	ReasonBlacklistedFile SecurityReason = "BLACKLISTED_FILE"
	// ReasonBlacklistedFolder indicates a file is in a blacklisted folder
	ReasonBlacklistedFolder SecurityReason = "BLACKLISTED_FOLDER"
	// ReasonBinaryFile indicates a file is a binary (detected by magic bytes)
	ReasonBinaryFile SecurityReason = "BINARY_FILE"
	// ReasonSecretDetected indicates a secret pattern was found
	ReasonSecretDetected SecurityReason = "SECRET_DETECTED"
	// ReasonSelfBinary indicates the file is git-courer's own binary
	ReasonSelfBinary SecurityReason = "SELF_BINARY"
)

// SecurityError creates a SecurityResult indicating a halted operation.
func SecurityError(reason SecurityReason, file, fileType, message string) *SecurityResult {
	return &SecurityResult{
		Safe:    false,
		Halted:  true,
		Reason:  reason,
		File:    file,
		Type:    fileType,
		Message: message,
	}
}

// SecurityCheckResult is returned by security checks that need to report multiple findings.
type SecurityCheckResult struct {
	Files   []SecurityResult
	Blocked bool
}

// IsBlocked returns true if any file was blocked.
func (r *SecurityCheckResult) IsBlocked() bool {
	return r.Blocked
}

// FirstBlocked returns the first blocking result, or nil if none.
func (r *SecurityCheckResult) FirstBlocked() *SecurityResult {
	for _, f := range r.Files {
		if f.Halted {
			return &f
		}
	}
	return nil
}

// ModelSize represents the size category of an LLM model.
type ModelSize string

const (
	ModelSizeSmall  ModelSize = "small"  // 1B-7B
	ModelSizeMedium ModelSize = "medium" // 7B-14B
	ModelSizeLarge  ModelSize = "large"  // 14B+
)

// ShouldUseLLMSecurityScan returns true if the model is large enough
// to be trusted for security scanning (14B+ parameters only).
func (m ModelSize) ShouldUseLLMSecurityScan() bool {
	return m == ModelSizeLarge
}

// IsSmall returns true for very small models that shouldn't be used for security.
func (m ModelSize) IsSmall() bool {
	return m == ModelSizeSmall
}
