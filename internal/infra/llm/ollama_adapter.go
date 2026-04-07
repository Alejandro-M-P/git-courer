package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/secrets"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
	"github.com/Alejandro-M-P/git-courer/internal/shared/stats"
)

// Adapter implements the LLM Port interface using Ollama
type Adapter struct {
	host        string
	model       string
	modelsDir   string // Custom models directory (for distrobox, etc.)
	process     *exec.Cmd
	startedByUs bool
	stats       *stats.Tracker
}

// NewAdapter creates a new Ollama adapter
func NewAdapter(host string, model string, modelsDir string) *Adapter {
	if host == "" {
		host = "http://localhost:11434"
	}
	tracker := stats.Load()
	return &Adapter{
		host:        host,
		model:       model,
		modelsDir:   modelsDir,
		startedByUs: false,
		stats:       tracker,
	}
}

// GetStats returns the token stats tracker
func (o *Adapter) GetStats() *stats.Tracker {
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

// DetectSecrets checks if files contain secrets using regex patterns.
// Delegates to the secrets package — no AI/LLM involved.
func (o *Adapter) DetectSecrets(files []string) ([]domain.SecretDetection, error) {
	return secrets.Detect(files)
}

const secretVerificationPrompt = `You are a security expert analyzing code to detect secrets.
A code analysis tool found potential secrets in the following diff:

%s

Potential secrets found:
%s

Your task: Determine if these are ACTUAL secrets or false positives.

Rules:
- API keys, tokens, passwords in production code are REAL secrets
- Example values, placeholder text, test data are NOT secrets
- Variable names like "YOUR_API_KEY" or "example_key" are NOT secrets
- Configuration that references secrets without containing them is NOT a leak

Respond with ONLY one word:
- "YES" if these are REAL secrets that should NOT be committed
- "NO" if these are false positives or example data
`

// VerifySecrets calls Ollama to verify if the given findings are real secrets.
// It takes the git diff content and regex findings, then uses the LLM to
// determine if they are actual secrets or false positives.
// Returns true if Ollama confirms they are real secrets, false otherwise.
func (o *Adapter) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	if len(findings) == 0 {
		return false, nil
	}

	// Format findings for prompt
	var findingsStr strings.Builder
	for _, f := range findings {
		findingsStr.WriteString(fmt.Sprintf("- %s in %s (line %d): %s\n",
			f.Type, f.File, f.Line, f.Content))
	}

	// Format prompt
	prompt := fmt.Sprintf(secretVerificationPrompt, diff, findingsStr.String())

	// Call Ollama
	response, promptTokens, evalTokens, err := o.generate(prompt)
	if err != nil {
		return false, fmt.Errorf("Ollama verification failed: %w", err)
	}

	// Record token usage
	if o.stats != nil {
		o.stats.RecordOperation(int64(promptTokens+evalTokens), promptTokens, evalTokens)
	}

	// Parse response
	response = strings.TrimSpace(strings.ToUpper(response))
	if strings.HasPrefix(response, "YES") {
		return true, nil // Real secrets confirmed
	}
	return false, nil // False positives
}

// GenerateChunkMessage generates a commit message for a single diff chunk.
// Uses the generate_message template with think=true for accuracy on small chunks.
func (o *Adapter) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	prompt, err := prompts.Render(prompts.GenerateMessage, prompts.BuildMessageParams(chunk.Files, chunk.Diff))
	if err != nil {
		return "", err
	}

	result, promptTokens, evalTokens, err := o.generateWithThink(prompt, false)
	if err != nil {
		return "", err
	}

	// Record token usage
	if o.stats != nil {
		o.stats.RecordOperation(int64(promptTokens+evalTokens), promptTokens, evalTokens)
	}

	// Clean up response
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// DEBUG: Log the LLM response for each chunk
	log.Printf("[DEBUG] GenerateChunkMessage - Files: %d, LLM_Response_Length: %d, LLM_Response: %q",
		len(chunk.Files), len(result), result)

	if result == "" {
		return "", fmt.Errorf("LLM returned empty message for chunk")
	}

	return result, nil
}

// DecideCommit sends the instruction and git status to the LLM to decide what to commit.
// This is a fast, single call to determine if we should include untracked files.
func (o *Adapter) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	prompt, err := prompts.Render(prompts.DecideCommit, prompts.BuildDecideParams(instruction, gitStatus, untracked, modified, deleted))
	if err != nil {
		return domain.CommitIntent{}, err
	}

	// Use a simple, fast call (think=false)
	result, promptTokens, evalTokens, err := o.generateWithThink(prompt, false)
	if err != nil {
		return domain.CommitIntent{}, err
	}

	// Record token usage
	if o.stats != nil {
		o.stats.RecordOperation(int64(promptTokens+evalTokens), promptTokens, evalTokens)
	}

	// Parse JSON response
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	var decision struct {
		IncludeUntracked bool   `json:"include_untracked"`
		Filter           string `json:"filter"`
	}

	if err := json.Unmarshal([]byte(result), &decision); err != nil {
		return domain.CommitIntent{}, fmt.Errorf("failed to parse LLM decision: %w", err)
	}

	return domain.CommitIntent{
		IncludeUntracked: decision.IncludeUntracked,
		Filter:           decision.Filter,
	}, nil
}

// Ensure Adapter implements the ports.LLM interface
var _ ports.LLM = (*Adapter)(nil)

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

// generate sends a prompt to Ollama with thinking disabled (fast mode)
func (o *Adapter) generate(prompt string) (string, int, int, error) {
	return o.generateWithThink(prompt, false)
}

// generateWithThink sends a prompt to Ollama with optional thinking mode.
// When thinkMode is true, the model reasons before responding (better accuracy for complex tasks).
// When thinkMode is false, responses are faster but less thorough.
func (o *Adapter) generateWithThink(prompt string, thinkMode bool) (string, int, int, error) {
	reqBody := map[string]interface{}{
		"model":  o.model,
		"prompt": prompt,
		"stream": false,
		"think":  thinkMode,
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

// Ensure Adapter implements the ports.LLM interface
var _ ports.LLM = (*Adapter)(nil)
