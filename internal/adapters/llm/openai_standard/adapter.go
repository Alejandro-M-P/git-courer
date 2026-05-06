package openai_standard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

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
type OpenAIStandardAdapter struct {
	client       *Client
	provider     string
	model        string
	context      string
	retryContext string
	numParallel  int
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

func (a *OpenAIStandardAdapter) SetProvider(p string) {
	a.provider = p
}

// Compile-time interface checks.
var _ ports.Lifecycle = (*OpenAIStandardAdapter)(nil)

// SetNumParallel bounds concurrent LLM calls. Values <= 0 are treated as 1.
func (a *OpenAIStandardAdapter) SetNumParallel(n int) {
	if n <= 0 {
		n = 1
	}
	a.numParallel = n
}

// Sentinel errors for centralized JSON parsing.
var (
	ErrEmptyResponse = errors.New("LLM returned empty response")
	ErrInvalidJSON   = errors.New("LLM returned invalid JSON")
)

// parseJSON strips markdown fences, validates the cleaned string, and
// unmarshals into v. Returns ErrEmptyResponse if the result is empty after
// cleaning, or ErrInvalidJSON if the first non-space character is not '{' or
// json.Unmarshal fails.
func parseJSON(raw string, v any) error {
	cleaned := cleanJSON(raw)
	if cleaned == "" {
		return ErrEmptyResponse
	}
	if cleaned[0] != '{' {
		return ErrInvalidJSON
	}
	if err := json.Unmarshal([]byte(cleaned), v); err != nil {
		return ErrInvalidJSON
	}
	return nil
}

// CommitMessageJSON is the structured output from the commit LLM prompt.
type CommitMessageJSON struct {
	Type        string `json:"type"`
	Scope       string `json:"scope"`
	Description string `json:"description"`
	Breaking    bool   `json:"breaking"`
	Body        string `json:"body"`
}

// ToConventionalCommit formats the CommitMessageJSON as a conventional commit string.
func (c CommitMessageJSON) ToConventionalCommit() string {
	var sb strings.Builder
	if c.Scope != "" {
		sb.WriteString(fmt.Sprintf("%s(%s): %s", c.Type, c.Scope, c.Description))
	} else {
		sb.WriteString(fmt.Sprintf("%s: %s", c.Type, c.Description))
	}
	if c.Breaking {
		msg := sb.String()
		sb.Reset()
		if idx := strings.Index(msg, ":"); idx != -1 {
			sb.WriteString(msg[:idx] + "!" + msg[idx:])
		} else {
			sb.WriteString(msg + "!")
		}
	}
	if c.Body != "" {
		sb.WriteString("\n\n" + c.Body)
	}
	return sb.String()
}

// GenerateChunkMessage generates a conventional commit message for a single diff chunk.
func (a *OpenAIStandardAdapter) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	var prompt string
	var err error
	if a.retryContext != "" {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, a.retryContext, a.context))
	} else {
		// Use annotated diff when available, fallback to raw diff
		annotatedDiff := ""
		if chunk.AnnotatedDiff != "" {
			annotatedDiff = chunk.AnnotatedDiff
		}
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParams(chunk.Files, chunk.Diff, annotatedDiff, a.context))
	}
	if err != nil {
		return "", fmt.Errorf("render commit prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "commit",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(commitGenTemp),
		maxTokens:       commitGenMaxTokens,
	})
	if err != nil {
		return "", err
	}

	var commit CommitMessageJSON
	if err := parseJSON(result, &commit); err != nil {
		return "", fmt.Errorf("parse commit message: %w", err)
	}

	return commit.ToConventionalCommit(), nil
}

// DecideCommit determines what files to stage based on user instruction and git status.
func (a *OpenAIStandardAdapter) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	prompt, err := prompts.Render(prompts.GetDecideCommit(), prompts.BuildDecideParams(instruction, gitStatus, untracked, modified, deleted))
	if err != nil {
		return domain.CommitIntent{}, fmt.Errorf("render decide commit prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "decide_commit",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(decideTemp),
		maxTokens:       decideMaxTokens,
	})
	if err != nil {
		return domain.CommitIntent{}, err
	}

	var decision struct {
		IncludeUntracked bool     `json:"include_untracked"`
		FileFilter       []string `json:"file_filter"`
	}
	if err := parseJSON(result, &decision); err != nil {
		return domain.CommitIntent{}, fmt.Errorf("parse decide commit: %w", err)
	}

	return domain.CommitIntent{
		IncludeUntracked: decision.IncludeUntracked,
		Filter:           decision.FileFilter,
	}, nil
}

// InterpretGitOp interprets a natural language instruction for a git operation.
func (a *OpenAIStandardAdapter) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	if context == nil {
		context = make(map[string]string)
	}
	context["Instruction"] = instruction

	tmpl, err := prompts.Get(op)
	if err != nil {
		// Fallback to a generic JSON prompt for undocumented git operations like branch/merge
		tmpl = `Interpret this git instruction: "{{.Instruction}}"\nRespond with ONLY a JSON object with the relevant arguments.`
	}
	prompt, err := prompts.Render(tmpl, context)
	if err != nil {
		return nil, fmt.Errorf("render interpret prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       op,
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(interpretTemp),
		maxTokens:       interpretMaxTokens,
	})
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	if err := parseJSON(result, &parsed); err != nil {
		return nil, fmt.Errorf("parse git op: %w", err)
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

// SetContext stores the project context string for prompt injection.
func (a *OpenAIStandardAdapter) SetContext(ctx string) {
	a.context = ctx
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
	_, err := a.chatCompletion(".", chatCompletionOpts{
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

	tmpl, err := prompts.Get("credential_audit")
	if err != nil {
		return false, fmt.Errorf("prompt not found: %w", err)
	}
	prompt, err := prompts.Render(tmpl, map[string]string{
		"Diff":     diff,
		"Findings": findingsStr.String(),
	})
	if err != nil {
		return false, fmt.Errorf("render credential_audit prompt: %w", err)
	}

	response, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:   "secret_verification",
		temperature: floatPtr(verifyTemp),
		maxTokens:   verifyMaxTokens,
	})
	if err != nil {
		return false, fmt.Errorf("verify secrets failed: %w", err)
	}
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(response)), "YES"), nil
}

// GenerateChangelog generates changelog from commits and returns structured data.
func (a *OpenAIStandardAdapter) GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	tmpl, err := prompts.Get("changelog_generate")
	if err != nil {
		return nil, fmt.Errorf("prompt not found: %w", err)
	}
	prompt, err := prompts.Render(tmpl, map[string]string{
		"commits": commits,
		"Context": a.context,
	})
	if err != nil {
		return nil, fmt.Errorf("render changelog prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "changelog",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(changelogTemp),
		maxTokens:       changelogMaxTokens,
	})
	if err != nil {
		return nil, err
	}

	var changelog domain.Changelog
	if err := parseJSON(result, &changelog); err != nil {
		return nil, fmt.Errorf("parse changelog: %w", err)
	}

	return &changelog, nil
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
	prompt, err := prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, feedback, a.context))
	if err != nil {
		return "", fmt.Errorf("render regenerate prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "regenerate",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(regenTemp),
		maxTokens:       regenMaxTokens,
	})
	if err != nil {
		return "", err
	}

	var commit CommitMessageJSON
	if err := parseJSON(result, &commit); err != nil {
		return "", fmt.Errorf("parse regenerate message: %w", err)
	}

	return commit.ToConventionalCommit(), nil
}

// chatCompletion sends a prompt via /chat/completions and returns the response content.
// When opts.jsonMode is true, the request includes format: "json" for structured output.
// When opts.reasoningEffort is "none", injects a /no_think system message so that
// Qwen3 and similar reasoning models skip the <think>...</think> output entirely.
func (a *OpenAIStandardAdapter) chatCompletion(prompt string, opts chatCompletionOpts) (string, error) {
	a.applyOperationParams(opts.operation, &opts)
	var messages []ChatMessage
	if opts.reasoningEffort == "none" {
		messages = []ChatMessage{
			{Role: "system", Content: "/no_think"},
			{Role: "user", Content: prompt},
		}
	} else {
		messages = []ChatMessage{{Role: "user", Content: prompt}}
	}
	return a.chatCompletionWithMessages(messages, opts)
}

// chatCompletionOpts configures a chatCompletionWithMessages call.
type chatCompletionOpts struct {
	operation       string
	jsonMode        bool
	reasoningEffort string
	maxTokens       int
	temperature     *float64
	topP            *float64
	frequencyPenalty *float64
	presencePenalty  *float64
	seed             *int
	stop             []string
}

// floatPtr returns a pointer to the given float64 value.
func floatPtr(f float64) *float64 { return &f }

// applyOperationParams is a stub — operation params were removed in config simplification.
func (a *OpenAIStandardAdapter) applyOperationParams(operation string, opts *chatCompletionOpts) {
	// no-op: per-operation LLM params are no longer configured
}

// chatCompletionWithMessages sends messages via /chat/completions and returns the response content.
func (a *OpenAIStandardAdapter) chatCompletionWithMessages(messages []ChatMessage, opts chatCompletionOpts) (string, error) {
	req := ChatRequest{
		Model:     a.model,
		Messages:  messages,
		Stream:    false,
		Think:     false, // Always suppress thinking
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
	if opts.topP != nil {
		req.TopP = opts.topP
	}
	if opts.frequencyPenalty != nil {
		req.FrequencyPenalty = opts.frequencyPenalty
	}
	if opts.presencePenalty != nil {
		req.PresencePenalty = opts.presencePenalty
	}
	if opts.seed != nil {
		req.Seed = opts.seed
	}
	if len(opts.stop) > 0 {
		req.Stop = opts.stop
	}

	// Inject Ollama-specific options only for ollama provider
	if a.provider == "ollama" {
		req.Options = make(map[string]interface{})
		if opts.temperature != nil {
			req.Options["temperature"] = *opts.temperature
		}
		if opts.maxTokens > 0 {
			req.Options["num_predict"] = opts.maxTokens
		}
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
	// Strip <think>...</think> blocks emitted by reasoning models (Qwen3, DeepSeek-R1).
	// reasoning_effort:"none" is not always honored by local backends.
	if end := strings.Index(s, "</think>"); end != -1 {
		s = strings.TrimSpace(s[end+len("</think>"):])
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
