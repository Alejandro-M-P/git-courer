package openai_standard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

// commitSystemPrompt instructs the model to produce only a commit message
// without reasoning, thinking, or explanatory text.
const commitSystemPrompt = "You are a commit message generator. Output ONLY the commit message text. Do NOT explain, think, reflect, or wrap in markdown. No reasoning tags."

// Per-operation LLM parameter defaults.
const (
	commitGenTemp      = 0.3
	commitGenMaxTokens = 256
	regenTemp          = 0.5
	regenMaxTokens     = 256
	decideTemp         = 0.0
	decideMaxTokens    = 128
	interpretTemp      = 0.1
	interpretMaxTokens = 256
	verifyTemp         = 0.0
	verifyMaxTokens    = 64
	auditTemp          = 0.0
	auditMaxTokens     = 64
	changelogTemp      = 0.3
	changelogMaxTokens = 1024
)

// OpenAIStandardAdapter implements ports.LLM and ports.Lifecycle via
// OpenAI-compatible endpoints.
// It works with any local backend that exposes /chat/completions and
// /completions under a versioned base URL (e.g., http://host:port/v1)
// (Ollama, llama.cpp, LM Studio, etc.).
type OpenAIStandardAdapter struct {
	client       *Client
	model        string
	retryContext string
	numParallel  int // default 1 (serial), >1 enables bounded parallel LLM calls
}

// Compile-time interface checks.
var _ ports.Lifecycle = (*OpenAIStandardAdapter)(nil)

// NewOpenAIStandardAdapter creates a new adapter for OpenAI-compatible backends.
func NewOpenAIStandardAdapter(baseURL, model string, opts ...ClientOption) *OpenAIStandardAdapter {
	return &OpenAIStandardAdapter{
		client:      NewClient(baseURL, opts...),
		model:       model,
		numParallel: 1,
	}
}

// commitMessages builds the standard system+user message pair for commit generation.
func commitMessages(userPrompt string) []ChatMessage {
	return []ChatMessage{
		{Role: "system", Content: commitSystemPrompt},
		{Role: "user", Content: userPrompt},
	}
}

// SetNumParallel bounds concurrent LLM calls. Values <= 0 are treated as 1.
func (a *OpenAIStandardAdapter) SetNumParallel(n int) {
	if n <= 0 {
		n = 1
	}
	a.numParallel = n
}

// CommitMessageJSON is the structured output from the commit LLM prompt.
type CommitMessageJSON struct {
	Type        string `json:"type"`
	Scope       string `json:"scope"`
	Description string `json:"description"`
	Breaking    bool   `json:"breaking"`
	Body        string `json:"body"`
}

// GenerateChunkMessage generates a conventional commit message for a single diff chunk.
func (a *OpenAIStandardAdapter) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	var prompt string
	var err error
	if a.retryContext != "" {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, a.retryContext))
	} else {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParams(chunk.Files, chunk.Diff))
	}
	if err != nil {
		return "", fmt.Errorf("render commit prompt: %w", err)
	}

	messages := commitMessages(prompt)

	result, err := a.chatCompletionWithMessages(messages, chatCompletionOpts{
		reasoningEffort: "none",
		temperature:     floatPtr(commitGenTemp),
		maxTokens:       commitGenMaxTokens,
	})
	if err != nil {
		return "", err
	}

	result = cleanJSON(result)
	if result == "" {
		return "", fmt.Errorf("LLM returned empty message for chunk")
	}

	// Parse JSON response into structured commit message
	var commit CommitMessageJSON
	if err := json.Unmarshal([]byte(result), &commit); err != nil {
		// Fallback: return raw result for backward compatibility
		return result, nil
	}

	// Format as conventional commit string
	var sb strings.Builder
	if commit.Scope != "" {
		sb.WriteString(fmt.Sprintf("%s(%s): %s", commit.Type, commit.Scope, commit.Description))
	} else {
		sb.WriteString(fmt.Sprintf("%s: %s", commit.Type, commit.Description))
	}
	if commit.Breaking {
		// Insert ! before the colon
		msg := sb.String()
		sb.Reset()
		if idx := strings.Index(msg, ":"); idx != -1 {
			sb.WriteString(msg[:idx] + "!" + msg[idx:])
		} else {
			sb.WriteString(msg + "!")
		}
	}
	if commit.Body != "" {
		sb.WriteString("\n\n" + commit.Body)
	}
	return sb.String(), nil
}

// DecideCommit determines what files to stage based on user instruction and git status.
func (a *OpenAIStandardAdapter) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	prompt, err := prompts.Render(prompts.GetDecideCommit(), prompts.BuildDecideParams(instruction, gitStatus, untracked, modified, deleted))
	if err != nil {
		return domain.CommitIntent{}, fmt.Errorf("render decide commit prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		jsonMode:    true,
		temperature: floatPtr(decideTemp),
		maxTokens:   decideMaxTokens,
	})
	if err != nil {
		return domain.CommitIntent{}, err
	}

	result = cleanJSON(result)

	// Try to parse as JSON with file_filter as array
	var decision struct {
		IncludeUntracked bool     `json:"include_untracked"`
		FileFilter       []string `json:"file_filter"`
	}
	if err := json.Unmarshal([]byte(result), &decision); err != nil {
		// Fallback: try parsing as YES/NO response (model doesn't return JSON)
		resultUpper := strings.ToUpper(result)
		includeUntracked := strings.Contains(resultUpper, "YES") || strings.Contains(resultUpper, "SI")

		// Extract filter if present (e.g., "YES, src/")
		var filter []string
		if strings.Contains(result, ",") {
			parts := strings.SplitN(result, ",", 2)
			if len(parts) > 1 {
				filter = splitPathsHelper(strings.TrimSpace(parts[1]))
			}
		}

		return domain.CommitIntent{
			IncludeUntracked: includeUntracked,
			Filter:          filter,
		}, nil
	}

	return domain.CommitIntent{
		IncludeUntracked: decision.IncludeUntracked,
		Filter:          decision.FileFilter,
	}, nil
}

// splitPathsHelper normalizes a path string for fallback parsing.
func splitPathsHelper(input string) []string {
	if input == "" {
		return nil
	}
	return strings.Fields(strings.ReplaceAll(input, ",", " "))
}

// InterpretGitOp interprets a natural language instruction for a git operation.
func (a *OpenAIStandardAdapter) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	if context == nil {
		context = make(map[string]string)
	}
	context["Instruction"] = instruction

	tmpl := prompts.Get(op)
	prompt, err := prompts.Render(tmpl, context)
	if err != nil {
		return nil, fmt.Errorf("render interpret prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		jsonMode:    true,
		temperature: floatPtr(interpretTemp),
		maxTokens:   interpretMaxTokens,
	})
	if err != nil {
		return nil, err
	}

	resultTrimmed := cleanJSON(result)
	if len(resultTrimmed) == 0 {
		return map[string]string{}, nil
	}

	// Handle non-JSON responses
	if resultTrimmed[0] != '{' {
		return map[string]string{}, nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(resultTrimmed), &parsed); err != nil {
		return map[string]string{}, nil
	}

	// Convert all values to strings
	args := make(map[string]string)
	for k, v := range parsed {
		args[k] = fmt.Sprintf("%v", v)
	}
	return args, nil
}

// AuditBinaryContent determines if content is binary noise using a fast heuristic.
func (a *OpenAIStandardAdapter) AuditBinaryContent(filename, content string) (bool, error) {
	return prompts.IsBinary([]byte(content)), nil
}

// SetRetryContext stores the previous rejected message for retry flow.
func (a *OpenAIStandardAdapter) SetRetryContext(previousMessage string) {
	a.retryContext = previousMessage
}

// ClearRetryContext clears the retry context after commit or abort.
func (a *OpenAIStandardAdapter) ClearRetryContext() {
	a.retryContext = ""
}

// RetryContext returns the current retry context (for testing and wrapper inspection).
func (a *OpenAIStandardAdapter) RetryContext() string {
	return a.retryContext
}

// EnsureRunning checks if the backend is available via GET /models.
// Returns (false, nil) if available (we didn't start it ourselves).
// Returns (false, error) if the backend is unreachable.
func (a *OpenAIStandardAdapter) EnsureRunning() (bool, error) {
	_, err := a.client.Get(context.Background(), "/models")
	if err != nil {
		return false, fmt.Errorf("backend not available: %w", err)
	}
	return false, nil
}

// PreWarm sends a minimal chat completion request to load the model into memory.
// This is useful for local backends (vLLM, LM Studio, llama.cpp) where the
// first request can be slow while the model loads.
func (a *OpenAIStandardAdapter) PreWarm() error {
	messages := commitMessages(".")
	_, err := a.chatCompletionWithMessages(messages, chatCompletionOpts{
		maxTokens: 1,
	})
	if err != nil {
		return fmt.Errorf("model %q failed to warm up: %w", a.model, err)
	}
	return nil
}

// Stop is a no-op for OpenAI-compatible backends — the user controls
// the server process lifecycle externally.
func (a *OpenAIStandardAdapter) Stop() {
	// no-op: remote/local servers managed by the user
}

// IsAvailable returns true if the LLM backend is reachable.
func (a *OpenAIStandardAdapter) IsAvailable() bool {
	_, err := a.client.Get(context.Background(), "/models")
	return err == nil
}

// IsWarmed returns true if the model has been loaded into memory.
// For OpenAI-compatible endpoints, we assume always warmed (no preload needed).
func (a *OpenAIStandardAdapter) IsWarmed() bool {
	return true
}

// VerifySecrets uses the LLM to verify if a diff contains sensitive information.
func (a *OpenAIStandardAdapter) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	if len(findings) == 0 {
		return false, nil
	}

	var findingsStr strings.Builder
	for _, f := range findings {
		findingsStr.WriteString(fmt.Sprintf("- %s in %s (line %d): %s\n", f.Type, f.File, f.Line, f.Content))
	}

	prompt, err := prompts.Render(prompts.Get("credential_audit"), map[string]string{
		"Diff":     diff,
		"Findings": findingsStr.String(),
	})
	if err != nil {
		return false, fmt.Errorf("render credential_audit prompt: %w", err)
	}

	response, err := a.chatCompletion(prompt, chatCompletionOpts{
		temperature: floatPtr(verifyTemp),
		maxTokens:   verifyMaxTokens,
	})
	if err != nil {
		return false, fmt.Errorf("verify secrets failed: %w", err)
	}
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(response)), "YES"), nil
}

// GenerateChangelog generates changelog from commits and returns it.
func (a *OpenAIStandardAdapter) GenerateChangelog(commits, previousChangelog, outputFile string) (string, error) {
	prompt, err := prompts.Render(prompts.Get("changelog_generate"), map[string]string{
		"commits": commits,
	})
	if err != nil {
		return "", fmt.Errorf("render changelog prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		jsonMode:    true,
		temperature: floatPtr(changelogTemp),
		maxTokens:   changelogMaxTokens,
	})
	if err != nil {
		return "", err
	}

	result = cleanJSON(result)
	if len(result) < 10 {
		return "", fmt.Errorf("LLM returned invalid changelog: %q", result)
	}

	// Parse structured changelog JSON
	var changelog struct {
		Features []string `json:"features"`
		Fixes    []string `json:"fixes"`
		Breaking []string `json:"breaking"`
		Docs     []string `json:"docs"`
		Perf     []string `json:"perf"`
		Internal []string `json:"internal"`
	}
	if err := json.Unmarshal([]byte(result), &changelog); err != nil {
		// Fallback: return raw result for backward compatibility
		fmt.Fprintf(os.Stderr, "warning: failed to parse changelog JSON: %v\n", err)
		changelog.Internal = []string{result}
	}

	// Format to readable markdown
	var sb strings.Builder
	if len(changelog.Features) > 0 {
		sb.WriteString("## Features\n")
		for _, f := range changelog.Features {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(changelog.Fixes) > 0 {
		sb.WriteString("## Fixes\n")
		for _, f := range changelog.Fixes {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(changelog.Breaking) > 0 {
		sb.WriteString("## Breaking Changes\n")
		for _, f := range changelog.Breaking {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(changelog.Docs) > 0 {
		sb.WriteString("## Documentation\n")
		for _, f := range changelog.Docs {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(changelog.Perf) > 0 {
		sb.WriteString("## Performance\n")
		for _, f := range changelog.Perf {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(changelog.Internal) > 0 {
		sb.WriteString("## Internal\n")
		for _, f := range changelog.Internal {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	output := sb.String()

	// Append to file if provided
	if outputFile != "" {
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			f.WriteString(output + "\n\n")
		}
	}

	return output, nil
}

// RegenerateMessage generates new commit messages based on feedback.
func (a *OpenAIStandardAdapter) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	if len(previousMessages) != len(chunks) {
		return nil, fmt.Errorf("previous messages count %d does not match chunks count %d", len(previousMessages), len(chunks))
	}

	newMessages := make([]string, len(chunks))

	// Serial fast-path for NumParallel == 1 (identical to old behavior).
	if a.numParallel <= 1 {
		for i, chunk := range chunks {
			msg, err := a.regenerateChunk(chunk, feedback)
			if err != nil {
				return nil, err
			}
			newMessages[i] = msg
		}
		return newMessages, nil
	}

	// Parallel path: errgroup + semaphore bounded by numParallel.
	g, ctx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(int64(a.numParallel))
	var mu sync.Mutex
	var warnings []string

	for i, chunk := range chunks {
		i, chunk := i, chunk // capture loop vars
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			msg, err := a.regenerateChunk(chunk, feedback)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", i+1, err))
				newMessages[i] = ""
				return nil // do NOT fail entire call
			}
			newMessages[i] = msg
			return nil
		})
	}

	// Wait for all goroutines (ignoring nil errors returned).
	_ = g.Wait()

	if len(warnings) > 0 {
		return newMessages, fmt.Errorf("regenerate warnings (%d): %s", len(warnings), strings.Join(warnings, "; "))
	}
	return newMessages, nil
}

// regenerateChunk is the per-chunk logic extracted for reuse in serial and parallel paths.
func (a *OpenAIStandardAdapter) regenerateChunk(chunk domain.DiffChunk, feedback string) (string, error) {
	prompt, err := prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, feedback))
	if err != nil {
		return "", fmt.Errorf("render regenerate prompt: %w", err)
	}

	messages := commitMessages(prompt)

	result, err := a.chatCompletionWithMessages(messages, chatCompletionOpts{
		reasoningEffort: "none",
		temperature:     floatPtr(regenTemp),
		maxTokens:       regenMaxTokens,
	})
	if err != nil {
		return "", err
	}

	result = cleanResponse(result)
	if result == "" {
		return "", fmt.Errorf("LLM returned empty message")
	}
	return result, nil
}

// chatCompletion sends a prompt via /chat/completions and returns the response content.
// When opts.jsonMode is true, the request includes format: "json" for structured output.
func (a *OpenAIStandardAdapter) chatCompletion(prompt string, opts chatCompletionOpts) (string, error) {
	messages := []ChatMessage{{Role: "user", Content: prompt}}
	return a.chatCompletionWithMessages(messages, opts)
}

// chatCompletionOpts configures a chatCompletionWithMessages call.
type chatCompletionOpts struct {
	jsonMode        bool
	reasoningEffort string
	maxTokens       int
	temperature     *float64
}

// floatPtr returns a pointer to the given float64 value.
func floatPtr(f float64) *float64 { return &f }

// chatCompletionWithMessages sends messages via /chat/completions and returns the response content.
func (a *OpenAIStandardAdapter) chatCompletionWithMessages(messages []ChatMessage, opts chatCompletionOpts) (string, error) {
	req := ChatRequest{
		Model:    a.model,
		Messages: messages,
		Stream:   false,
	}
	if opts.jsonMode {
		req.Format = "json"
	}
	if opts.reasoningEffort != "" {
		req.ReasoningEffort = opts.reasoningEffort
	}
	if opts.maxTokens > 0 {
		req.MaxTokens = opts.maxTokens
	}
	if opts.temperature != nil {
		req.Temperature = opts.temperature
	}

	body, err := a.client.Post(context.Background(), "/chat/completions", req)
	if err != nil {
		return "", fmt.Errorf("chat completion request failed: %w", err)
	}

	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no chat choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// cleanJSON strips defensive markdown fences and whitespace from JSON responses.
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// cleanResponse cleans markdown fences from general text responses.
func cleanResponse(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimPrefix(s, "markdown")
	return strings.TrimSpace(s)
}