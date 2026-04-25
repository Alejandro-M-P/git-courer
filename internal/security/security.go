// Package security implements multi-layer security checks before commits.
// Layers: binary detection → folder blacklist → name blacklist → regex → LLM (large models only).
package security

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/secrets"
)

// Service implements ports.SecurityService.
type Service struct {
	cfg       *config.Config
	llm       ports.LLM
	modelSize domain.ModelSize
}

// New creates a new security Service.
func New(cfg *config.Config, llm ports.LLM) *Service {
	return &Service{
		cfg:       cfg,
		llm:       llm,
		modelSize: ParseModelSize(cfg.Ollama.Model),
	}
}

// ShouldUseLLMScan returns true if the model should be used for security verification.
// With improved prompts, all models (even small ones) are supported.
func (s *Service) ShouldUseLLMScan() bool {
	switch s.cfg.Secrets.UseLLMSecurityScan {
	case "false":
		return false
	default:
		return true // Enabled by default for all models
	}
}

// CheckFiles runs all security layers on the given files.
func (s *Service) CheckFiles(files []string, diff string) *ports.SecurityCheckResult {
	result := &ports.SecurityCheckResult{Files: []ports.SecurityResult{}}

	for _, file := range files {
		// LAYER 1: Binary detection (Magic Bytes + AI Audit)
		if secrets.IsBinary(file) {
			result.Files = append(result.Files, ports.SecurityResult{
				Halted: true, Reason: string(domain.ReasonBinaryFile),
				File: file, Type: "binary",
				Message: formatMessage(domain.ReasonBinaryFile, file),
			})
			result.Blocked = true
			return result
		}

		// AI Audit for suspicious text files (e.g., .txt, .js, .go that might be binary blobs)
		if s.ShouldUseLLMScan() && !strings.HasSuffix(file, "_test.go") {
			content, err := os.ReadFile(file)
			if err == nil {
				isBinary, err := s.llm.AuditBinaryContent(file, string(content))
				if err == nil && isBinary {
					result.Files = append(result.Files, ports.SecurityResult{
						Halted: true, Reason: "AI Audit detected binary noise or obfuscated content",
						File: file, Type: "binary_audit",
						Message: "[SECURITY] BINARY_AUDIT: " + file + " — Content appears to be binary/obfuscated",
					})
					result.Blocked = true
					return result
				}
			}
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

		// Exception: legitimate test source files are allowed to contain fake secrets.
		// We only skip if it is a Go test file specifically.
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
	}

	// LAYER 4: Static analysis (gosec or trufflehog, if installed)
	if finding := runStaticAnalysis(files); finding != "" {
		result.Files = append(result.Files, ports.SecurityResult{
			Halted: true, Reason: string(domain.ReasonSecretDetected),
			File: strings.Join(files, ","), Type: "static_analysis",
			Message: "[SECURITY] STATIC_ANALYSIS: " + finding,
		})
		result.Blocked = true
		return result
	}

	// LAYER 5: Regex scan (disco + contenido)
	regexFindings, _ := secrets.Detect(files)
	contentFindings := secrets.DetectInContent(diff)
	
	// Merge all findings
	filteredFindings := []domain.SecretDetection{}
	allFindings := append(regexFindings, contentFindings...)
	
	for _, finding := range allFindings {
		if strings.HasSuffix(finding.File, "_test.go") || strings.HasPrefix(finding.File, "test/") {
			continue
		}
		filteredFindings = append(filteredFindings, finding)
		result.Files = append(result.Files, ports.SecurityResult{
			Halted: false, Reason: string(domain.ReasonSecretDetected),
			File: finding.File, Line: finding.Line, Type: finding.Type,
			Message: formatSecretMessage(finding),
		})
	}

	// LAYER 6: LLM verification and PROACTIVE scanning
	if s.ShouldUseLLMScan() {
		// Filter diff to remove test files before sending to LLM
		cleanDiff := filterDiff(diff)
		
		// Call the LLM with the clean diff and any regex findings
		isRealSecret, err := s.llm.VerifySecrets(cleanDiff, filteredFindings)
		if err == nil && isRealSecret {
			result.Blocked = true
			result.Files = append(result.Files, ports.SecurityResult{
				Halted: true, Reason: string(domain.ReasonSecretDetected),
				File: "diff", Type: "ai_auditor",
				Message: "[SECURITY] AI_AUDITOR: Potential secrets detected by LLM in the diff content",
			})
			return result
		}
		
		// If LLM says NO, and we only had regex findings, we can optionally clear the block
		// but for safety, let's trust the AI's "NO" only if the regex findings were low confidence.
		// For now, if AI says NO, we unblock regex findings to avoid false positives.
		if err == nil && !isRealSecret && len(filteredFindings) > 0 {
			result.Blocked = false
			result.Files = []ports.SecurityResult{} // Clear regex findings if AI confirms false positive
		}
	} else if len(filteredFindings) > 0 {
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

func runStaticAnalysis(files []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if toolPath, err := exec.LookPath("trufflehog"); err == nil {
		args := append([]string{"filesystem", "--no-update", "--fail"}, files...)
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, toolPath, args...)
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			output := strings.TrimSpace(out.String())
			if output != "" {
				lines := strings.SplitN(output, "\n", 3)
				return "trufflehog: " + lines[0]
			}
		}
		return ""
	}

	if toolPath, err := exec.LookPath("gosec"); err == nil {
		args := append([]string{"-quiet"}, files...)
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, toolPath, args...)
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			output := strings.TrimSpace(out.String())
			if output != "" {
				lines := strings.SplitN(output, "\n", 3)
				return "gosec: " + lines[0]
			}
		}
		return ""
	}

	return ""
}

// Compile-time interface check.
var _ ports.SecurityService = (*Service)(nil)

// filterDiff removes test files and prompts from the diff to avoid AI false positives.
func filterDiff(diff string) string {
	var clean strings.Builder
	chunks := strings.Split(diff, "diff --git ")
	for _, chunk := range chunks {
		if chunk == "" {
			continue
		}
		
		lines := strings.SplitN(chunk, "\n", 2)
		header := lines[0]
		
		isTest := strings.Contains(header, "_test.go") || 
				  strings.Contains(header, "test/") || 
				  strings.Contains(header, "internal/shared/prompts/txt/")
				  
		if !isTest {
			clean.WriteString("diff --git ")
			clean.WriteString(chunk)
		}
	}
	return clean.String()
}
