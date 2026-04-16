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
	{regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{20,}`), "openai_key", false},
	{regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`), "github_token", false},
	{regexp.MustCompile(`(?i)xox[baprs][a-zA-Z0-9]{10,}`), "slack_token", false},
	{regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`), "aws_access_key", false},
	{regexp.MustCompile(`(?i)amzn\.mfa\.[a-zA-Z0-9]{20,}`), "aws_mfa_token", false},
	{regexp.MustCompile(`(?i)AIza[0-9A-Za-z_-]{35}`), "google_api_key", false},
	{regexp.MustCompile(`(?i)ya29\.[0-9A-Za-z_-]{100,}`), "google_oauth_token", false},
	{regexp.MustCompile(`(?i)sq0[a-z]{3}-[0-9A-Za-z_-]{22}`), "stripe_key", false},
	{regexp.MustCompile(`(?i)sq0csp-[0-9A-Za-z_-]{43}`), "stripe_secret", false},
	{regexp.MustCompile(`(?i)sk_live_[0-9a-zA-Z]{24,}`), "stripe_live_key", false},
	{regexp.MustCompile(`(?i)sk_test_[0-9a-zA-Z]{24,}`), "stripe_test_key", false},
	{regexp.MustCompile(`(?i)pk_live_[0-9a-zA-Z]{24,}`), "stripe_live_pubkey", false},
	{regexp.MustCompile(`(?i)pk_test_[0-9a-zA-Z]{24,}`), "stripe_test_pubkey", false},
	{regexp.MustCompile(`(?i)sk-ant-[a-zA-Z0-9\-]{90,}`), "anthropic_key", false},
	{regexp.MustCompile(`(?i)hf_[a-zA-Z0-9]{34,}`), "huggingface_token", false},
	{regexp.MustCompile(`(?i)r8_[a-zA-Z0-9]{40}`), "replicate_token", false},
	{regexp.MustCompile(`eyJ[a-zA-Z0-9]{100,}`), "jwt_token", false},
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
		lower := strings.ToLower(file)
		if strings.Contains(lower, "credentials") ||
			strings.Contains(lower, "secrets") ||
			strings.Contains(lower, "password") ||
			strings.Contains(lower, ".env") {
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
