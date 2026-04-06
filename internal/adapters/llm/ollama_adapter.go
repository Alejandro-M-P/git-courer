package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/domain/models"
)

// Adapter implements the LLM Port interface using Ollama
type Adapter struct {
	host        string
	model       string
	process     *exec.Cmd
	startedByUs bool
}

// NewAdapter creates a new Ollama adapter
func NewAdapter(host string, model string) *Adapter {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &Adapter{
		host:        host,
		model:       model,
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

	cmd := exec.Command("ollama", "serve")
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("error al arrancar Ollama: %v", err)
	}

	o.process = cmd
	o.startedByUs = true

	// Wait for Ollama to be ready (max 15 seconds)
	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		if o.IsAvailable() {
			fmt.Println("✓ Ollama listo!")
			return true, nil
		}
	}

	return false, fmt.Errorf("Ollama tardó demasiado en arrancar")
}

// Stop stops the Ollama process if we started it
func (o *Adapter) Stop() {
	if o.startedByUs && o.process != nil && o.process.Process != nil {
		fmt.Println("🛑 Apagando Ollama...")
		o.process.Process.Kill()
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

// generate sends a prompt to Ollama and returns the response
// Retry with progressive wait if model is loading
func (o *Adapter) generate(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  o.model,
		"prompt": prompt,
		"stream": false,
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
	waitTimes := []time.Duration{3 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 20 * time.Second}
	var lastErr error

	for attempt, wait := range waitTimes {
		// Longer timeout for later retries (model loading takes time)
		timeout := 60 * time.Second
		if attempt >= 2 {
			timeout = 120 * time.Second
		}
		if attempt >= 4 {
			timeout = 180 * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		req, err := http.NewRequestWithContext(ctx, "POST", o.host+"/api/generate", bytes.NewBuffer(jsonBody))
		if err != nil {
			cancel()
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		cancel()

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
	buf := make([]byte, 0, 4096)
	for {
		n, err := resp.Body.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			if n > 0 {
				break
			}
			return string(buf), err
		}
		if len(buf) == cap(buf) {
			buf = append(buf, make([]byte, 4096)...)
		}
	}
	return string(buf), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Ensure Adapter implements the LLMPort interface
var _ models.LLMPort = (*Adapter)(nil)
