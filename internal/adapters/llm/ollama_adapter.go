package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/domain/models"
)

// Adapter implements the LLM Port interface using Ollama
type Adapter struct {
	host        string
	model       string
	modelsDir   string // Custom models directory (for distrobox, etc.)
	process     *exec.Cmd
	startedByUs bool
	stats       *TokenStats
}

// NewAdapter creates a new Ollama adapter
func NewAdapter(host string, model string, modelsDir string) *Adapter {
	if host == "" {
		host = "http://localhost:11434"
	}
	stats := &TokenStats{}
	stats.Load()
	return &Adapter{
		host:        host,
		model:       model,
		modelsDir:   modelsDir,
		startedByUs: false,
		stats:       stats,
	}
}

// GetStats returns the token stats tracker
func (o *Adapter) GetStats() *TokenStats {
	return o.stats
}

// ResolveModel checks if the configured model is available in Ollama.
// If not, it picks the first available model. This prevents "model not found"
// errors when the user has a different model than the config default.
func (o *Adapter) ResolveModel() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.host+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to check Ollama models: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama not available: %w", err)
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("failed to parse Ollama tags: %w", err)
	}

	// Check if our model is in the list
	for _, m := range tags.Models {
		if m.Name == o.model || m.Name == o.model+":latest" {
			return nil // Model found, all good
		}
	}

	// Model not found — pick the first available
	if len(tags.Models) > 0 {
		oldModel := o.model
		o.model = tags.Models[0].Name
		fmt.Printf("⚠ Model %q not found. Using %q instead.\n", oldModel, o.model)
		return nil
	}

	return fmt.Errorf("no models available in Ollama. Pull one with: ollama pull qwen3.5:latest")
}

// PreWarm sends a minimal request to load the model into memory.
// This prevents the first real request from timing out while the model loads.
// Uses a tiny prompt with just 1 token to minimize cost.
func (o *Adapter) PreWarm() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model":  o.model,
		"prompt": ".",
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.0,
			"num_predict": 1,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.host+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("model %q failed to load: %w", o.model, err)
	}
	defer resp.Body.Close()

	// Don't care about the response — just need the model loaded
	fmt.Printf("✓ Model %q loaded in memory\n", o.model)
	return nil
}

// findOllamaBinary searches for the ollama binary in common system locations
func findOllamaBinary() string {
	// Standard system-wide locations (Linux, macOS)
	locations := []string{
		"/usr/local/bin/ollama",
		"/usr/bin/ollama",
		"/opt/homebrew/bin/ollama",
		"/snap/bin/ollama",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Fallback: try PATH
	if path, err := exec.LookPath("ollama"); err == nil {
		return path
	}

	return ""
}

// EnsureOllama starts Ollama if not running (Lazy Loading)
// Returns true if Ollama was started by us, false if already running
func (o *Adapter) EnsureOllama() (bool, error) {
	// Check if already running
	if o.IsAvailable() {
		// Resolve model in case config doesn't match installed models
		if err := o.ResolveModel(); err != nil {
			return false, err
		}
		return false, nil
	}

	// Not running, let's start it
	fmt.Println("🚀 Ollama está apagado. Arrancando motor local...")

	// Find ollama binary
	ollamaPath := findOllamaBinary()
	if ollamaPath == "" {
		return false, fmt.Errorf("ollama binary not found. Install from https://ollama.com")
	}

	fmt.Printf("  Using ollama at: %s\n", ollamaPath)

	// Get the real user home directory (not inherited from environment)
	currentUser, err := user.Current()
	if err != nil {
		return false, fmt.Errorf("no se pudo determinar el usuario actual: %v", err)
	}

	// Start Ollama with correct HOME and optionally custom models dir
	cmd := exec.Command(ollamaPath, "serve")
	cmd.Env = append(os.Environ(), "HOME="+currentUser.HomeDir)
	// If custom models directory is configured, use it (for distrobox, etc.)
	if o.modelsDir != "" {
		cmd.Env = append(cmd.Env, "OLLAMA_MODELS="+o.modelsDir)
	}
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("error al arrancar Ollama: %v", err)
	}

	o.process = cmd
	o.startedByUs = true

	// Wait for Ollama to be ready (max 30 seconds)
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		if o.IsAvailable() {
			fmt.Println("✓ Ollama listo!")
			return true, nil
		}
	}

	return false, fmt.Errorf("Ollama tardó demasiado en arrancar")
}

// Stop stops the Ollama process if we started it
// Note: We no longer kill Ollama on shutdown - it stays running for next use
func (o *Adapter) Stop() {
	if o.startedByUs && o.process != nil && o.process.Process != nil {
		fmt.Println("🛑 Leaving Ollama running for next use...")
		o.process = nil
		o.startedByUs = false
	}
}

// DetectSecrets checks if files contain secrets using regex patterns
func (o *Adapter) DetectSecrets(files []string) ([]models.SecretDetection, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var secrets []models.SecretDetection

	// Patterns: (pattern, type, checkContent)
	patterns := []struct {
		regex      *regexp.Regexp
		secretType string
		checkExt   bool // if true, check file extension instead of content
	}{
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
	}

	// File extension patterns for sensitive files
	sensitiveExts := map[string]string{
		".env":      "env_file",
		".pem":      "private_key",
		".key":      "private_key",
		".pkcs8":    "private_key",
		".p12":      "keystore",
		".keystore": "keystore",
	}

	for _, file := range files {
		// Check file extension
		ext := strings.ToLower(filepath.Ext(file))
		if secretType, ok := sensitiveExts[ext]; ok {
			secrets = append(secrets, models.SecretDetection{
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
			secrets = append(secrets, models.SecretDetection{
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
		defer f.Close()

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
					secrets = append(secrets, models.SecretDetection{
						File:    file,
						Line:    lineNum,
						Type:    p.secretType,
						Content: redacted,
					})
					break // move to next line after first match
				}
			}
		}
	}

	return secrets, nil
}

// GenerateCommitMessage generates a commit message for the given files
// Uses a short, fast prompt with think=false for speed
func (o *Adapter) GenerateCommitMessage(instruction string, files []string) (string, error) {
	// Build file list string
	var fileList string
	for _, f := range files {
		fileList += "- " + f + "\n"
	}

	numFiles := len(files)

	// Dynamic prompt based on number of files
	var prompt string
	if numFiles <= 2 {
		prompt = fmt.Sprintf(`Files: %s

Write ONE line: type + action (max 50 chars). No explanation.
Types: feat, fix, chore, docs, refactor, test, perf
Example: "feat: add login button [local-ollama]"
Output:`, fileList)
	} else if numFiles <= 5 {
		prompt = fmt.Sprintf(`Files: %s

Write type + action (max 60 chars). If needed, one line explaining what/why.
Example: "fix: resolve auth bug\n\nFixes token expiration handling. [local-ollama]"
Output:`, fileList)
	} else {
		prompt = fmt.Sprintf(`Files: %s

Write type + action (max 60 chars) + one sentence body explaining the change.
Example: "feat: add user dashboard\n\nNew dashboard showing stats and recent activity. [local-ollama]"
Output:`, fileList)
	}

	result, promptTokens, evalTokens, err := o.generate(prompt)
	if err != nil {
		return "", err
	}

	// Record real token usage
	if o.stats != nil {
		o.stats.RecordOperation(int64(promptTokens+evalTokens), promptTokens, evalTokens)
	}

	return strings.TrimSpace(result), nil
}

// AnalyzeAndPlanCommit analyzes files and diff to plan commits with proper grouping
// and secret detection
func (o *Adapter) AnalyzeAndPlanCommit(files []string, diff string) (models.CommitAnalysis, error) {
	if len(diff) > 4000 {
		diff = diff[:4000] + "\n... (truncated)"
	}

	prompt := fmt.Sprintf(`Output ONLY valid JSON. No explanation. No markdown. No text before or after.

Files: %s

Diff:
%s

Return this exact structure:
{
  "strategy": "single",
  "commits": [
    {
      "files": ["file.go"],
      "message": "feat: add login validation",
      "type": "feat"
    }
  ],
  "excluded": [
    {
      "file": ".env",
      "reason": "contains secrets"
    }
  ],
  "warnings": []
}

EXCLUSION RULES — these files MUST go in excluded, never in commits:
- Files with API keys, tokens, passwords, secrets in the diff
- .env, .env.local, .env.*, *.pem, *.key, *.token, credentials.*
- node_modules/, vendor/, .venv/, __pycache__/
- *.exe, *.dll, *.so, *.dylib, dist/, build/, bin/
- package-lock.json, yarn.lock, go.sum (unless only change)
- Any variable named: API_KEY, SECRET, PASSWORD, TOKEN, PRIVATE_KEY
- Any hardcoded URL with credentials: postgres://user:pass@...

COMMIT RULES:
- message MUST start with: feat: fix: chore: docs: refactor: test: perf:
- message max 72 chars
- if excluded files leave nothing to commit, return empty commits array
- if remaining files are unrelated, use strategy "split" with multiple commits

SECRET PATTERNS to detect in diff:
- sk-[a-zA-Z0-9]{20,}
- ghp_[a-zA-Z0-9]{36}
- AKIA[0-9A-Z]{16}
- AIza[0-9A-Za-z_-]{35}

OUTPUT ONLY JSON:`, strings.Join(files, ", "), diff)

	// Retry 3 times if JSON invalid
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, promptTokens, evalTokens, err := o.generate(prompt)
		if err != nil {
			lastErr = err
			continue
		}

		if o.stats != nil {
			o.stats.RecordOperation(int64(promptTokens+evalTokens), promptTokens, evalTokens)
		}

		result = strings.TrimSpace(result)
		result = strings.TrimPrefix(result, "```json")
		result = strings.TrimPrefix(result, "```")
		result = strings.TrimSuffix(result, "```")
		result = strings.TrimSpace(result)

		var analysis models.CommitAnalysis
		if err := json.Unmarshal([]byte(result), &analysis); err != nil {
			lastErr = fmt.Errorf("attempt %d: invalid JSON: %w", attempt+1, err)
			continue
		}

		return analysis, nil
	}

	return models.CommitAnalysis{}, fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// IsAvailable checks if Ollama is running
func (o *Adapter) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.host+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// IsModelReady checks if the configured model is loaded and ready to generate.
// Returns true if ready, false if model is still loading or not available.
func (o *Adapter) IsModelReady() bool {
	// Try a minimal generate request with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model":  o.model,
		"prompt": ".",
		"stream": false,
		"options": map[string]interface{}{
			"num_predict": 1,
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", o.host+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// If we get any error (timeout, connection refused, etc), model isn't ready
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// generate sends a prompt to Ollama and returns the response plus real token counts
// Retry with progressive wait if model is loading
// Returns: response text, prompt_eval_count, eval_count, error
func (o *Adapter) generate(prompt string) (string, int, int, error) {
	reqBody := map[string]interface{}{
		"model":  o.model,
		"prompt": prompt,
		"stream": false,
		"think":  false, // Disable thinking to make responses faster
		"options": map[string]interface{}{
			"temperature": 0.3,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, err
	}

	// Retry with progressive wait (model might be loading)
	// For large models (6GB+), first load can take 30-60 seconds
	waitTimes := []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second, 60 * time.Second, 90 * time.Second}
	var lastErr error

	for attempt, wait := range waitTimes {
		timeout := timeoutFor(attempt)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", o.host+"/api/generate", bytes.NewBuffer(jsonBody))
		if err != nil {
			return "", 0, 0, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to Ollama: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait)
				continue
			}
			return "", 0, 0, lastErr
		}

		body, readErr := readBody(resp)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait)
				continue
			}
			return "", 0, 0, lastErr
		}

		if resp.StatusCode != 200 {
			// Check if model is loading
			if strings.Contains(body, "loading") || resp.StatusCode == 503 {
				lastErr = fmt.Errorf("model %q is loading, retrying (%d/%d)...", o.model, attempt+1, len(waitTimes))
				time.Sleep(wait)
				continue
			}
			return "", 0, 0, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, truncate(body, 200))
		}

		var response struct {
			Response        string `json:"response"`
			PromptEvalCount int    `json:"prompt_eval_count"`
			EvalCount       int    `json:"eval_count"`
		}
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			lastErr = fmt.Errorf("failed to parse Ollama response: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait)
				continue
			}
			return "", 0, 0, lastErr
		}

		return response.Response, response.PromptEvalCount, response.EvalCount, nil
	}

	return "", 0, 0, fmt.Errorf("Ollama failed after %d retries: %w", len(waitTimes), lastErr)
}

func readBody(resp *http.Response) (string, error) {
	buf, err := io.ReadAll(resp.Body)
	return string(buf), err
}

func timeoutFor(attempt int) time.Duration {
	switch {
	case attempt >= 4:
		return 180 * time.Second
	case attempt >= 2:
		return 120 * time.Second
	default:
		return 60 * time.Second
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Ensure Adapter implements the LLMPort interface
var _ models.LLMPort = (*Adapter)(nil)
