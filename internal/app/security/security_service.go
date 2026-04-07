package security

import (
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/secrets"
)

// securityService implements ports.SecurityService
type securityService struct {
	config    *config.Config
	modelSize domain.ModelSize
}

// NewSecurityService creates a new SecurityService
func NewSecurityService(cfg *config.Config) *securityService {
	modelSize := ParseModelSize(cfg.Ollama.Model)
	return &securityService{
		config:    cfg,
		modelSize: modelSize,
	}
}

// ShouldUseLLMScan implements ports.SecurityService
func (s *securityService) ShouldUseLLMScan() bool {
	// Check config override first
	switch s.config.Secrets.UseLLMSecurityScan {
	case "false":
		return false
	case "true":
		return true // Force on (even small models — user override)
	case "auto":
		fallthrough
	default:
		// Use model size threshold (only large = 14B+)
		return s.modelSize.ShouldUseLLMSecurityScan()
	}
}

// CheckFiles implements ports.SecurityService
func (s *securityService) CheckFiles(files []string, diff string) *ports.SecurityCheckResult {
	result := &ports.SecurityCheckResult{
		Files:   []ports.SecurityResult{},
		Blocked: false,
	}

	for _, file := range files {
		// LAYER 1: Magic bytes — if binary, HALT immediately
		if secrets.IsBinary(file) {
			result.Files = append(result.Files, ports.SecurityResult{
				Halted:  true,
				Reason:  string(domain.ReasonBinaryFile),
				File:    file,
				Type:    "binary",
				Message: formatMessage(domain.ReasonBinaryFile, file),
			})
			result.Blocked = true
			return result // HARD FAIL — abort immediately
		}

		// LAYER 2: Folder blacklist — if in blacklisted folder, HALT immediately
		if secrets.IsBlacklistedFolder(file) {
			result.Files = append(result.Files, ports.SecurityResult{
				Halted:  true,
				Reason:  string(domain.ReasonBlacklistedFolder),
				File:    file,
				Type:    "blacklisted_folder",
				Message: formatMessage(domain.ReasonBlacklistedFolder, file),
			})
			result.Blocked = true
			return result // HARD FAIL — abort immediately
		}

		// LAYER 3: Name blacklist — if filename is blacklisted, HALT immediately
		filename := secrets.ExtractFilename(file)
		if secrets.IsBlacklistedName(filename) {
			result.Files = append(result.Files, ports.SecurityResult{
				Halted:  true,
				Reason:  string(domain.ReasonBlacklistedFile),
				File:    file,
				Type:    "blacklisted_file",
				Message: formatMessage(domain.ReasonBlacklistedFile, filename),
			})
			result.Blocked = true
			return result // HARD FAIL — abort immediately
		}
	}

	// LAYER 4: Regex scan — always runs
	regexFindings, _ := secrets.Detect(files)
	for _, finding := range regexFindings {
		result.Files = append(result.Files, ports.SecurityResult{
			Halted:  false, // Don't halt yet — let LLM verify
			Reason:  string(domain.ReasonSecretDetected),
			File:    finding.File,
			Line:    finding.Line,
			Type:    finding.Type,
			Message: formatSecretMessage(finding),
		})
	}

	// LAYER 5: Ollama LLM verification
	// If regex found something AND model is large enough, do LLM verification
	// SEC-007 will implement the actual Ollama call
	if len(regexFindings) > 0 && s.ShouldUseLLMScan() {
		// SEC-007 TODO: Call Ollama to verify
		// For now, mark as blocked since we found something suspicious
		// The actual Ollama call would be in SEC-007
		result.Blocked = true
		return result
	}

	// If we found something but LLM scan is disabled, block conservatively
	// Better to block false positive than leak real secret
	if len(regexFindings) > 0 && !s.ShouldUseLLMScan() {
		result.Blocked = true
		return result
	}

	// If we get here, all checks passed
	return result
}

// formatMessage formats a security error message
func formatMessage(reason domain.SecurityReason, file string) string {
	switch reason {
	case domain.ReasonBinaryFile:
		return "[SECURITY] BINARY_FILE: " + file + " — Cannot commit binary files"
	case domain.ReasonSelfBinary:
		return "[SECURITY] SELF_BINARY: " + file + " — Cannot commit git-courer binary"
	case domain.ReasonBlacklistedFolder:
		return "[SECURITY] BLACKLISTED_FOLDER: " + file + " — Cannot commit from blacklisted folder"
	case domain.ReasonBlacklistedFile:
		return "[SECURITY] BLACKLISTED_FILE: " + file + " — Cannot commit blacklisted file"
	case domain.ReasonSecretDetected:
		return "[SECURITY] SECRET_DETECTED: " + file + " — Potential secret detected"
	default:
		return "[SECURITY] " + string(reason) + ": " + file
	}
}

// formatSecretMessage formats a secret detection message
func formatSecretMessage(detection domain.SecretDetection) string {
	location := detection.File
	if detection.Line > 0 {
		location = fmt.Sprintf("%s:%d", detection.File, detection.Line)
	}
	return fmt.Sprintf("[SECURITY] SECRET_DETECTED: %s — Potential %s detected", location, detection.Type)
}
