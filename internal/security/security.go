// Package security implements multi-layer security checks before commits.
// Layers: binary detection → folder blacklist → name blacklist → regex → LLM (large models only).
package security

import (
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/secrets"
)

// Service implements ports.SecurityService.
type Service struct {
	cfg       *config.Config
	modelSize domain.ModelSize
}

// New creates a new security Service.
func New(cfg *config.Config) *Service {
	return &Service{
		cfg:       cfg,
		modelSize: ParseModelSize(cfg.Ollama.Model),
	}
}

// ShouldUseLLMScan returns true if the model is large enough for LLM-based scanning.
func (s *Service) ShouldUseLLMScan() bool {
	switch s.cfg.Secrets.UseLLMSecurityScan {
	case "false":
		return false
	case "true":
		return true
	default:
		return s.modelSize.ShouldUseLLMSecurityScan()
	}
}

// CheckFiles runs all security layers on the given files.
func (s *Service) CheckFiles(files []string, diff string) *ports.SecurityCheckResult {
	result := &ports.SecurityCheckResult{Files: []ports.SecurityResult{}}

	for _, file := range files {
		// LAYER 1: Binary detection — hard stop
		if secrets.IsBinary(file) {
			result.Files = append(result.Files, ports.SecurityResult{
				Halted: true, Reason: string(domain.ReasonBinaryFile),
				File: file, Type: "binary",
				Message: formatMessage(domain.ReasonBinaryFile, file),
			})
			result.Blocked = true
			return result
		}

		// LAYER 2: Folder blacklist — hard stop
		if secrets.IsBlacklistedFolder(file) {
			result.Files = append(result.Files, ports.SecurityResult{
				Halted: true, Reason: string(domain.ReasonBlacklistedFolder),
				File: file, Type: "blacklisted_folder",
				Message: formatMessage(domain.ReasonBlacklistedFolder, file),
			})
			result.Blocked = true
			return result
		}

		// LAYER 3: Name blacklist — hard stop
		filename := secrets.ExtractFilename(file)
		if secrets.IsBlacklistedName(filename) {
			result.Files = append(result.Files, ports.SecurityResult{
				Halted: true, Reason: string(domain.ReasonBlacklistedFile),
				File: file, Type: "blacklisted_file",
				Message: formatMessage(domain.ReasonBlacklistedFile, filename),
			})
			result.Blocked = true
			return result
		}
	}

	// LAYER 4: Regex scan
	regexFindings, _ := secrets.Detect(files)
	for _, finding := range regexFindings {
		result.Files = append(result.Files, ports.SecurityResult{
			Halted: false, Reason: string(domain.ReasonSecretDetected),
			File: finding.File, Line: finding.Line, Type: finding.Type,
			Message: formatSecretMessage(finding),
		})
	}

	// LAYER 5: LLM verification (large models only)
	if len(regexFindings) > 0 {
		result.Blocked = true
	}

	return result
}

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
	default:
		return "[SECURITY] " + string(reason) + ": " + file
	}
}

func formatSecretMessage(detection domain.SecretDetection) string {
	location := detection.File
	if detection.Line > 0 {
		location = fmt.Sprintf("%s:%d", detection.File, detection.Line)
	}
	return fmt.Sprintf("[SECURITY] SECRET_DETECTED: %s — Potential %s detected", location, detection.Type)
}

// Compile-time interface check.
var _ ports.SecurityService = (*Service)(nil)
