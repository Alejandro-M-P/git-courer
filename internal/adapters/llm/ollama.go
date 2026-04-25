// Package llm implements the LLM port using Ollama as the backend.
package llm

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

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/secrets"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

// Adapter implements ports.LLM via Ollama.
type Adapter struct {
	host         string
	model        string
	modelsDir    string
	process      *exec.Cmd
	startedByUs  bool
	retryContext string
	supportsJSON bool // auto-detected: does model support format:json in generate API?
}

// New creates a new Ollama adapter.
func New(host, model, modelsDir string) *Adapter {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &Adapter{host: host, model: model, modelsDir: modelsDir}
}

// ResolveModel checks if the configured model is available; falls back to first available.
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
	for _, m := range tags.Models {
		if m.Name == o.model || m.Name == o.model+":latest" {
			// Model found - detect JSON support
			o.supportsJSON = o.detectJSONSupport()
			return nil
		}
	}
	if len(tags.Models) > 0 {
		oldModel := o.model
		o.model = tags.Models[0].Name
		fmt.Printf("⚠ Model %q not found. Using %q instead.\n", oldModel, o.model)
		// Model changed - detect JSON support
		o.supportsJSON = o.detectJSONSupport()
		return nil
	}
	return fmt.Errorf("no models available in Ollama. Pull one with: ollama pull qwen3.5:latest")
}

// detectJSONSupport tests if the model responds correctly to format:json in generate API
func (o *Adapter) detectJSONSupport() bool {
	testPrompt := `responde solo con JSON: {"ok": true}`

	reqBody := map[string]interface{}{
		"model":  o.model,
		"prompt": testPrompt,
		"stream": false,
		"format": "json",
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("[JSON Detect] Failed to marshal request: %v\n", err)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", o.host+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("[JSON Detect] Failed to create request: %v\n", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[JSON Detect] Request failed: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[JSON Detect] Failed to read response: %v\n", err)
		return false
	}

	var response struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("[JSON Detect] Failed to parse response: %v\n", err)
		return false
	}

	// Check if response is valid JSON
	result := strings.TrimSpace(response.Response)
	if result == "" {
		fmt.Printf("[JSON Detect] Empty response - JSON mode NOT supported\n")
		return false
	}

	// Try to parse as JSON
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(result), &js); err != nil {
		fmt.Printf("[JSON Detect] Response is not valid JSON - JSON mode NOT supported\n")
		return false
	}

	fmt.Printf("[JSON Detect] ✅ Model %q supports format:json\n", o.model)
	return true
}

// PreWarm loads the model into memory with a minimal request.
func (o *Adapter) PreWarm() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model": o.model, "prompt": ".", "stream": false,
		"options": map[string]interface{}{"temperature": 0.0, "num_predict": 1},
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
	fmt.Printf("✓ Model %q loaded in memory\n", o.model)
	return nil
}

// EnsureOllama starts Ollama if not running. Returns true if started by us.
func (o *Adapter) EnsureOllama() (bool, error) {
	if o.IsAvailable() {
		if err := o.ResolveModel(); err != nil {
			return false, err
		}
		return false, nil
	}
	fmt.Println("🚀 Ollama está apagado. Arrancando motor local...")

	ollamaPath := findOllamaBinary()
	if ollamaPath == "" {
		return false, fmt.Errorf("ollama binary not found. Install from https://ollama.com")
	}
	fmt.Printf("  Using ollama at: %s\n", ollamaPath)

	currentUser, err := user.Current()
	if err != nil {
		return false, fmt.Errorf("no se pudo determinar el usuario actual: %v", err)
	}

	cmd := exec.Command(ollamaPath, "serve")
	cmd.Env = append(os.Environ(), "HOME="+currentUser.HomeDir)
	if o.modelsDir != "" {
		cmd.Env = append(cmd.Env, "OLLAMA_MODELS="+o.modelsDir)
	}
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("error al arrancar Ollama: %v", err)
	}
	o.process = cmd
	o.startedByUs = true

	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		if o.IsAvailable() {
			fmt.Println("✓ Ollama listo!")
			return true, nil
		}
	}
	return false, fmt.Errorf("Ollama tardó demasiado en arrancar")
}

// Stop leaves Ollama running for next use (no kill on shutdown).
func (o *Adapter) Stop() {
	if o.startedByUs && o.process != nil && o.process.Process != nil {
		fmt.Println("🛑 Leaving Ollama running for next use...")
		o.process = nil
		o.startedByUs = false
	}
}

// IsAvailable checks if Ollama is reachable.
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

// SetRetryContext stores the previous rejected message for retry flow.
func (o *Adapter) SetRetryContext(previousMessage string) { o.retryContext = previousMessage }

// ClearRetryContext clears the retry context after commit or abort.
func (o *Adapter) ClearRetryContext() { o.retryContext = "" }

// GenerateChunkMessage generates a commit message for a single diff chunk.
func (o *Adapter) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	var prompt string
	var err error
	if o.retryContext != "" {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, o.retryContext))
	} else {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParams(chunk.Files, chunk.Diff))
	}
	if err != nil {
		return "", err
	}

	result, _, _, err := o.generateWithThink(prompt, false)
	if err != nil {
		return "", err
	}
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)
	if result == "" {
		return "", fmt.Errorf("LLM returned empty message for chunk")
	}
	return result, nil
}

// DecideCommit asks the LLM what files to stage based on instruction and git status.
func (o *Adapter) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	prompt, err := prompts.Render(prompts.GetDecideCommit(), prompts.BuildDecideParams(instruction, gitStatus, untracked, modified, deleted))
	if err != nil {
		return domain.CommitIntent{}, err
	}
	result, _, _, err := o.generateChatJSON(prompt, decideCommitSchema)
	if err != nil {
		return domain.CommitIntent{}, err
	}
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// Check if result is valid JSON
	var decision struct {
		IncludeUntracked bool     `json:"include_untracked"`
		Filter           string   `json:"file_filter"`
		FilesSelected    []string `json:"files_selected"`
		FilesExcluded    []string `json:"files_excluded"`
	}
	if err := json.Unmarshal([]byte(result), &decision); err != nil {
		// Model doesn't return JSON - parse YES/NO response instead
		resultUpper := strings.ToUpper(result)
		includeUntracked := strings.Contains(resultUpper, "YES") || strings.Contains(resultUpper, "SI")

		// Extract filter if present (e.g., "YES, src/")
		filter := ""
		if strings.Contains(result, ",") {
			parts := strings.SplitN(result, ",", 2)
			if len(parts) > 1 {
				filter = strings.TrimSpace(parts[1])
			}
		}

		return domain.CommitIntent{
			IncludeUntracked: includeUntracked,
			Filter:           filter,
		}, nil
	}
	return domain.CommitIntent{
		IncludeUntracked: decision.IncludeUntracked,
		Filter:           decision.Filter,
		FilesSelected:    decision.FilesSelected,
		FilesExcluded:    decision.FilesExcluded,
	}, nil
}

// InterpretGitOp interprets a natural language instruction for a git operation.
// Returns concrete args as a map (e.g. {"branch": "feat/login"}).
func (o *Adapter) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	prompt, err := prompts.Render(prompts.InterpretGitOp, prompts.BuildInterpretParams(op, instruction, context))
	if err != nil {
		return nil, err
	}
	result, _, _, err := o.generateChatJSON(prompt, interpretGitOpSchema)
	if err != nil {
		return nil, err
	}
	result = strings.TrimSpace(result)

	// Defensive: strip markdown code blocks FIRST
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// Try to detect if it's valid JSON and handle edge cases gracefully
	resultTrimmed := strings.TrimSpace(result)
	if len(resultTrimmed) == 0 {
		return map[string]string{}, nil
	}

	// Check for and strip any remaining markdown code blocks
	resultTrimmed = strings.TrimPrefix(resultTrimmed, "```json")
	resultTrimmed = strings.TrimPrefix(resultTrimmed, "```")
	resultTrimmed = strings.TrimSuffix(resultTrimmed, "```")
	resultTrimmed = strings.TrimSpace(resultTrimmed)
	if len(resultTrimmed) == 0 {
		return map[string]string{}, nil
	}

	// Handle primitive values first (bool, number, string, array)
	resultLower := strings.ToLower(resultTrimmed)
	if resultLower == "true" || resultLower == "false" ||
		resultLower == "null" || resultLower == "none" {
		return map[string]string{}, nil
	}

	// Handle arrays
	if resultTrimmed[0] == '[' {
		return map[string]string{}, nil
	}

	// Handle non-JSON responses (plain strings, numbers)
	firstChar := resultTrimmed[0]
	if firstChar != '{' && firstChar != '"' && (firstChar < 'a' || firstChar > 'z') {
		return map[string]string{}, nil
	}

	// Now try to parse as JSON object - but use permissive parsing FIRST
	var permissing map[string]interface{}
	if err := json.Unmarshal([]byte(resultTrimmed), &permissing); err != nil {
		// Invalid JSON - return empty
		return map[string]string{}, nil
	}

	// Convert all values to strings
	args := make(map[string]string)
	for k, v := range permissing {
		strVal := fmt.Sprintf("%v", v)
		args[k] = strVal
	}
	return args, nil
}

// DetectSecrets checks files for secrets using regex patterns.
func (o *Adapter) DetectSecrets(files []string) ([]domain.SecretDetection, error) {
	return secrets.Detect(files)
}

const secretVerificationPrompt = `You are a security expert analyzing code to detect secrets.
A code analysis tool found potential secrets in the following diff:

%s

Potential secrets found:
%s

Determine if these are ACTUAL secrets or false positives.
Rules:
- API keys, tokens, passwords in production code are REAL secrets
- Example values, placeholder text, test data are NOT secrets

Respond with ONLY one word: "YES" if real secrets, "NO" if false positives.`

// VerifySecrets uses the LLM to verify if regex findings are real secrets.
func (o *Adapter) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	if len(findings) == 0 {
		return false, nil
	}
	var findingsStr strings.Builder
	for _, f := range findings {
		findingsStr.WriteString(fmt.Sprintf("- %s in %s (line %d): %s\n", f.Type, f.File, f.Line, f.Content))
	}
	prompt := fmt.Sprintf(secretVerificationPrompt, diff, findingsStr.String())
	response, _, _, err := o.generate(prompt)
	if err != nil {
		return false, fmt.Errorf("Ollama verification failed: %w", err)
	}
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(response)), "YES"), nil
}

// InterpretReleaseIntent interprets user's release intent.
func (o *Adapter) InterpretReleaseIntent(instruction, releases, branches, currentBranch string) (*domain.ReleaseIntent, error) {
	prompt, err := prompts.Render(prompts.Get("release_interpret"), map[string]string{
		"instruction":    instruction,
		"releases":       releases,
		"branches":       branches,
		"current_branch": currentBranch,
	})
	if err != nil {
		return nil, err
	}
	result, _, _, err := o.generateChatJSON(prompt, releaseIntentSchema)
	if err != nil {
		return nil, err
	}
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// Extract the JSON object in case the model added surrounding text or thinking tokens.
	if i := strings.Index(result, "{"); i >= 0 {
		if j := strings.LastIndex(result, "}"); j >= i {
			result = result[i : j+1]
		}
	}

	if result == "" {
		return nil, fmt.Errorf("LLM returned empty response for release intent")
	}

	var intent struct {
		Intent  string   `json:"intent"`
		Version string   `json:"version"`
		Bump    string   `json:"bump"`
		Merge   []string `json:"merge"`
		Reason  string   `json:"reason"`
	}
	if err := json.Unmarshal([]byte(result), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse ReleaseIntent: %w", err)
	}
	return &domain.ReleaseIntent{
		TagName:     intent.Version,
		IsRelease:   intent.Intent == "release",
		VersionBump: intent.Bump,
		BranchFrom:  intent.Reason,
		MergePath:   intent.Merge,
	}, nil
}

// GenerateChangelog generates changelog from commits and returns it.
func (o *Adapter) GenerateChangelog(commits, previousChangelog, outputFile string) (string, error) {
	prompt, err := prompts.Render(prompts.Get("changelog_generate"), map[string]string{
		"commits": commits,
	})
	if err != nil {
		return "", err
	}
	result, _, _, err := o.generate(prompt)
	if err != nil {
		return "", err
	}

	// Clean up response
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```markdown")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// If result is empty or too short, something went wrong
	if len(result) < 10 {
		return "", fmt.Errorf("LLM returned invalid changelog: %q", result)
	}

	// Append to file if provided
	if outputFile != "" {
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			f.WriteString(result + "\n\n")
		}
	}

	return result, nil
}

// PolishChangelog polishes the final changelog from chunks.
func (o *Adapter) PolishChangelog(chunks []string) (string, error) {
	prompt, err := prompts.Render(prompts.Get("changelog_polish"), map[string]string{
		"Chunks": strings.Join(chunks, "\n---\n"),
	})
	if err != nil {
		return "", err
	}
	result, _, _, err := o.generate(prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// RegenerateMessage generates new commit messages based on feedback.
// Used when the user requests regeneration of commit messages in preview mode.
func (o *Adapter) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	if len(previousMessages) != len(chunks) {
		return nil, fmt.Errorf("previous messages count %d does not match chunks count %d", len(previousMessages), len(chunks))
	}
	
	newMessages := make([]string, len(chunks))
	for i, chunk := range chunks {
		prompt, err := prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, feedback))
		if err != nil {
			return nil, err
		}
		result, _, _, err := o.generateWithThink(prompt, false)
		if err != nil {
			return nil, err
		}
		result = strings.TrimSpace(result)
		result = strings.TrimPrefix(result, "```")
		result = strings.TrimSuffix(result, "```")
		result = strings.TrimSpace(result)
		if result == "" {
			return nil, fmt.Errorf("LLM returned empty message for chunk %d", i)
		}
		newMessages[i] = result
	}
	return newMessages, nil
}

// generate sends a prompt to Ollama (no thinking mode).
func (o *Adapter) generate(prompt string) (string, int, int, error) {
	return o.generateWithThink(prompt, false)
}

// generateChatJSON sends a prompt to Ollama using the chat API with JSON schema enforcement.
func (o *Adapter) generateWithThink(prompt string, thinkMode bool) (string, int, int, error) {
	reqBody := map[string]interface{}{
		"model": o.model, "prompt": prompt, "stream": false, "think": thinkMode,
		"options": map[string]interface{}{"temperature": 0.3},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, err
	}

	waitTimes := []time.Duration{10, 20, 30, 60, 90}
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
			if resp != nil {
				resp.Body.Close()
			}
			lastErr = fmt.Errorf("failed to connect to Ollama: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}

		bodyStr := string(body)
		if resp.StatusCode != 200 {
			if strings.Contains(bodyStr, "loading") || resp.StatusCode == 503 {
				lastErr = fmt.Errorf("model %q is loading, retrying (%d/%d)...", o.model, attempt+1, len(waitTimes))
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, truncate(bodyStr, 200))
		}

		var response struct {
			Response        string `json:"response"`
			Think           string `json:"think,omitempty"`
			PromptEvalCount int    `json:"prompt_eval_count"`
			EvalCount       int    `json:"eval_count"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			lastErr = fmt.Errorf("failed to parse Ollama response: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}
		return response.Response, response.PromptEvalCount, response.EvalCount, nil
	}
	return "", 0, 0, fmt.Errorf("Ollama failed after %d retries: %w", len(waitTimes), lastErr)
}

func findOllamaBinary() string {
	locations := []string{
		"/usr/local/bin/ollama", "/usr/bin/ollama",
		"/opt/homebrew/bin/ollama", "/snap/bin/ollama",
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	if path, err := exec.LookPath("ollama"); err == nil {
		return path
	}
	return ""
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

// generateChatJSON sends a prompt to Ollama using the chat API with JSON format.
// This ensures the model returns valid JSON that matches the expected structure.
func (o *Adapter) generateChatJSON(prompt string, schema map[string]interface{}) (string, int, int, error) {
	fmt.Printf("[DEBUG] generateChatJSON: model=%s, host=%s\n", o.model, o.host)

	// Truncate prompt for logging
	promptPreview := prompt
	if len(prompt) > 200 {
		promptPreview = prompt[:200] + "..."
	}
	fmt.Printf("[DEBUG] Prompt: %s\n", promptPreview)

	// Use generate API (not chat) if model doesn't support JSON
	// This is more reliable for models like qwen
	if !o.supportsJSON {
		return o.generate(prompt)
	}

	// Model supports JSON - use chat API with format:json
	reqBody := map[string]interface{}{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
		"format": "json",
		"options": map[string]interface{}{
			"temperature": 0.1,
			"num_predict": 4096,
		},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, err
	}

	waitTimes := []time.Duration{10, 20, 30, 60, 90}
	var lastErr error

	// Use shorter timeouts for chat API to avoid hanging
	for attempt, wait := range []time.Duration{5, 10, 15, 30, 60} {
		timeout := timeoutFor(attempt)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", o.host+"/api/chat", bytes.NewBuffer(jsonBody))
		if err != nil {
			return "", 0, 0, err
		}
		req.Header.Set("Content-Type", "application/json")

		fmt.Printf("[DEBUG] Attempt %d: Sending request (timeout: %v)...\n", attempt+1, timeout)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			lastErr = fmt.Errorf("failed to connect to Ollama: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}

		bodyStr := string(body)
		if resp.StatusCode != 200 {
			if strings.Contains(bodyStr, "loading") || resp.StatusCode == 503 {
				lastErr = fmt.Errorf("model %q is loading, retrying (%d/%d)...", o.model, attempt+1, len(waitTimes))
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, truncate(bodyStr, 200))
		}

		var response struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			PromptEvalCount int `json:"prompt_eval_count"`
			EvalCount       int `json:"eval_count"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			lastErr = fmt.Errorf("failed to parse Ollama response: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}

		result := strings.TrimSpace(response.Message.Content)
		fmt.Printf("[DEBUG] Raw response: %s\n", result)
		result = strings.TrimPrefix(result, "```json")
		result = strings.TrimPrefix(result, "```")
		result = strings.TrimSuffix(result, "```")
		result = strings.TrimSpace(result)

		// Validate JSON
		if len(result) == 0 {
			lastErr = fmt.Errorf("empty response from model")
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}

		var js map[string]interface{}
		if err := json.Unmarshal([]byte(result), &js); err != nil {
			lastErr = fmt.Errorf("invalid JSON response: %w", err)
			if attempt < len(waitTimes)-1 {
				time.Sleep(wait * time.Second)
				continue
			}
			return "", 0, 0, lastErr
		}

		fmt.Printf("[DEBUG] Success! Valid JSON received\n")
		return result, response.PromptEvalCount, response.EvalCount, nil
	}
	return "", 0, 0, fmt.Errorf("Ollama failed after %d retries: %w", len(waitTimes), lastErr)
}

// JSON Schemas for structured output
var (
	decideCommitSchema = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"include_untracked": map[string]interface{}{"type": "boolean"},
			"file_filter":       map[string]interface{}{"type": "string"},
		},
		"required": []string{"include_untracked"},
	}

	interpretGitOpSchema = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{"type": "string"},
			"branch":    map[string]interface{}{"type": "string"},
			"name":      map[string]interface{}{"type": "string"},
			"base":      map[string]interface{}{"type": "string"},
			"tag":       map[string]interface{}{"type": "string"},
		},
	}

	releaseIntentSchema = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"intent":  map[string]interface{}{"type": "string"},
			"version": map[string]interface{}{"type": "string"},
			"bump":    map[string]interface{}{"type": "string"},
			"merge":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"reason":  map[string]interface{}{"type": "string"},
		},
		"required": []string{"intent", "version"},
	}
)

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Compile-time interface check.
var _ ports.LLM = (*Adapter)(nil)
