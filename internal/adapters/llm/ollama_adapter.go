package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
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
}

// NewAdapter creates a new Ollama adapter
func NewAdapter(host string, model string, modelsDir string) *Adapter {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &Adapter{
		host:        host,
		model:       model,
		modelsDir:   modelsDir,
		startedByUs: false,
	}
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

// DetectSecrets checks if files contain secrets
func (o *Adapter) DetectSecrets(files []string) ([]models.SecretDetection, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// This is a simplified version - in production you'd read files and analyze
	// For now, just return empty - will be enhanced later
	return []models.SecretDetection{}, nil
}

// GetContextNeeded - FIRST CALL to Ollama
// Given instruction, returns what context (files, diffs, etc.) is needed
func (o *Adapter) GetContextNeeded(instruction string) (models.ContextRequest, error) {
	prompt := fmt.Sprintf(`You are a git context analyzer. Given this instruction: "%s"

Analyze what context is needed to execute this operation. Return ONLY valid JSON:

{
  "files_needed": ["file1.go", "file2.go"],  // specific files to read
  "diff_needed": true/false,                // whether staged diff is needed
  "branch_info": true/false,                 // whether current branch is needed
  "status_info": true/false,                  // whether repo status is needed
  "log_info": true/false,                    // whether commit log is needed
  "description": "what you will do with this context"
}

Examples:
- "commit the login changes" → {"diff_needed": true, "status_info": true, "description": "generate commit message"}
- "show status" → {"status_info": true, "description": "show repository status"}
- "create a feature branch" → {"branch_info": true, "description": "determine branch name"}
- "push to remote" → {"branch_info": true, "log_info": true, "description": "check what to push"}

Return ONLY the JSON, no other text.`, instruction)

	result, err := o.generate(prompt)
	if err != nil {
		return models.ContextRequest{}, err
	}

	// Parse JSON response
	var ctxReq models.ContextRequest
	if err := json.Unmarshal([]byte(result), &ctxReq); err != nil {
		// If parse fails, return default (minimal context)
		return models.ContextRequest{
			DiffNeeded:  true,
			StatusInfo:  true,
			Description: "default context for: " + instruction,
		}, nil
	}

	return ctxReq, nil
}

// GetFullDecision - SECOND CALL to Ollama
// Given instruction + context, returns full decision (JSON with commits, commands, etc.)
func (o *Adapter) GetFullDecision(instruction, context string) (models.GitDecision, error) {
	prompt := fmt.Sprintf(`You are a git operation planner. Given this instruction: "%s"

And this context:
%s

Generate a detailed plan. Return ONLY valid JSON:

{
  "type": "read|write",
  "strategy": "single|split",
  "commits": [
    {
      "files": ["file.go"],
      "commit": "type: description",
      "commands": ["git add file.go", "git commit -m \"type: description\""]
    }
  ],
  "summary": {
    "action": "what will be done",
    "files": ["files involved"],
    "branch": "current branch"
  },
  "secrets": [],
  "suspicious": []
}

Rules:
- commit messages MUST follow conventional commits (feat:, fix:, chore:, docs:, refactor:, test:, perf:)
- IMPORTANT: Add "[local-ollama]" at the end of commit messages to confirm this was generated by local AI
- commands MUST start with "git "
- if diff is too large (>200 lines), use "strategy": "split" and divide into multiple commits
- analyze for secrets: API keys, passwords, tokens, .env files
- suspicious files: compiled files (dist/, build/), logs, binaries

Return ONLY the JSON, no other text.`, instruction, context)

	result, err := o.generate(prompt)
	if err != nil {
		return models.GitDecision{}, err
	}

	// Parse JSON response
	var decision models.GitDecision
	if err := json.Unmarshal([]byte(result), &decision); err != nil {
		return models.GitDecision{}, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	return decision, nil
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

	result, err := o.generate(prompt)
	if err != nil {
		return "", err
	}

	// Clean up the result
	result = strings.TrimSpace(result)

	return result, nil
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

// generate sends a prompt to Ollama and returns the response
// Retry with progressive wait if model is loading
func (o *Adapter) generate(prompt string) (string, error) {
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
		return "", err
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
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to Ollama: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait)
				continue
			}
			return "", lastErr
		}

		body, readErr := readBody(resp)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait)
				continue
			}
			return "", lastErr
		}

		if resp.StatusCode != 200 {
			// Check if model is loading
			if strings.Contains(body, "loading") || resp.StatusCode == 503 {
				lastErr = fmt.Errorf("model %q is loading, retrying (%d/%d)...", o.model, attempt+1, len(waitTimes))
				time.Sleep(wait)
				continue
			}
			return "", fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, truncate(body, 200))
		}

		var response struct {
			Response string `json:"response"`
		}
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			lastErr = fmt.Errorf("failed to parse Ollama response: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait)
				continue
			}
			return "", lastErr
		}

		return response.Response, nil
	}

	return "", fmt.Errorf("Ollama failed after %d retries: %w", len(waitTimes), lastErr)
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
