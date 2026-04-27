// Package secrets provides regex-based secret detection in files.
// This is pure pattern matching — no AI/LLM involved.
package secrets

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// pattern represents a secret detection pattern.
type pattern struct {
	regex      *regexp.Regexp
	secretType string
	checkExt   bool // if true, check file extension instead of content
}

var patterns = []pattern{
	{regexp.MustCompile(`(?i)` + "\x73\x6b\x2d" + `[a-zA-Z0-9]{20,}`), "openai_key", false},
	{regexp.MustCompile(`(?i)` + "\x67\x68\x70\x5f" + `[a-zA-Z0-9]{36}`), "github_token", false},
	{regexp.MustCompile(`(?i)` + "\x78\x6f\x78" + `[baprs][a-zA-Z0-9]{10,}`), "slack_token", false},
	{regexp.MustCompile(`(?i)` + "\x41\x4b\x49\x41" + `[0-9A-Z]{16}`), "aws_access_key", false},
	{regexp.MustCompile(`(?i)` + "\x61\x6d\x7a\x6e\x2e\x6d\x66\x61\x2e" + `[a-zA-Z0-9]{20,}`), "aws_mfa_token", false},
	{regexp.MustCompile(`(?i)` + "\x41\x49\x7a\x61" + `[0-9A-Za-z_-]{35}`), "google_api_key", false},
	{regexp.MustCompile(`(?i)` + "\x79\x61\x32\x39\x2e" + `[0-9A-Za-z_-]{100,}`), "google_oauth_token", false},
	{regexp.MustCompile(`(?i)` + "\x73\x71\x30" + `[a-z]{3}-[0-9A-Za-z_-]{22}`), "stripe_key", false},
	{regexp.MustCompile(`(?i)` + "\x73\x71\x30\x63\x73\x70\x2d" + `[0-9A-Za-z_-]{43}`), "stripe_secret", false},
	{regexp.MustCompile(`(?i)` + "\x73\x6b\x5f\x6c\x69\x76\x65\x5f" + `[0-9a-zA-Z_]{24,}`), "stripe_live_key", false},
	{regexp.MustCompile(`(?i)` + "\x73\x6b\x5f\x74\x65\x73\x74\x5f" + `[0-9a-zA-Z_]{24,}`), "stripe_test_key", false},
	{regexp.MustCompile(`(?i)` + "\x70\x6b\x5f\x6c\x69\x76\x65\x5f" + `[0-9a-zA-Z_]{24,}`), "stripe_live_pubkey", false},
	{regexp.MustCompile(`(?i)` + "\x70\x6b\x5f\x74\x65\x73\x74\x5f" + `[0-9a-zA-Z_]{24,}`), "stripe_test_pubkey", false},
	{regexp.MustCompile(`(?i)` + "\x73\x6b\x2d\x61\x6e\x74\x2d" + `[a-zA-Z0-9\-]{90,}`), "anthropic_key", false},
	{regexp.MustCompile(`(?i)` + "\x68\x66\x5f" + `[a-zA-Z0-9]{34,}`), "huggingface_token", false},
	{regexp.MustCompile(`(?i)` + "\x72\x38\x5f" + `[a-zA-Z0-9]{40}`), "replicate_token", false},
	{regexp.MustCompile("\x65\x79\x4a" + `[a-zA-Z0-9]{100,}`), "jwt_token", false},
}

var sensitiveExts = map[string]string{
	".env":      "env_file",
	".pem":      "private_key",
	".key":      "private_key",
	".pkcs8":    "private_key",
	".p12":      "keystore",
	".keystore": "keystore",
}

// Detect checks if files contain secrets using regex patterns.
func Detect(files []string) ([]domain.SecretDetection, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var secrets []domain.SecretDetection

	for _, file := range files {
		// Check file extension
		ext := strings.ToLower(filepath.Ext(file))
		if secretType, ok := sensitiveExts[ext]; ok {
			secrets = append(secrets, domain.SecretDetection{
				File: file,
				Line: 0,
				Type: secretType,
			})
			continue
		}

		// Check file name for credentials files
		filename := strings.ToLower(filepath.Base(file))
		if strings.Contains(filename, "credentials") ||
			strings.Contains(filename, "secrets") ||
			strings.Contains(filename, "password") ||
			strings.Contains(filename, ".env") {
			secrets = append(secrets, domain.SecretDetection{
				File: file,
				Line: 0,
				Type: "sensitive_file",
			})
			continue
		}

		// Scan file content for patterns
		f, err := os.Open(file)
		if err != nil {
			continue // skip files that can't be read
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			content := scanner.Text()

			// Skip comments and obvious examples
			if strings.HasPrefix(content, "#") || strings.HasPrefix(content, "//") {
				continue
			}

			for _, p := range patterns {
				if p.regex.MatchString(content) {
					// Redact the secret for logging
					redacted := p.regex.ReplaceAllStringFunc(content, func(match string) string {
						if len(match) > 8 {
							return match[:4] + "..." + match[len(match)-4:]
						}
						return "***"
					})
					secrets = append(secrets, domain.SecretDetection{
						File:    file,
						Line:    lineNum,
						Type:    p.secretType,
						Content: redacted,
					})
					break // move to next line after first match
				}
			}
		}
		f.Close()
	}

	return secrets, nil
}

// DetectInContent scans a string (like a git diff) for sensitive patterns.
func DetectInContent(content string) []domain.SecretDetection {
	var findings []domain.SecretDetection
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for _, p := range patterns {
			if p.regex.MatchString(line) {
				findings = append(findings, domain.SecretDetection{
					File:    "diff",
					Line:    i + 1,
					Type:    p.secretType,
					Content: line, // In a diff, the line itself is the context
				})
				break
			}
		}
	}
	return findings
}
