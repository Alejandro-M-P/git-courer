package openai_standard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
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
}

// Compile-time interface checks.
var _ ports.Lifecycle = (*OpenAIStandardAdapter)(nil)

// NewOpenAIStandardAdapter creates a new adapter for OpenAI-compatible backends.
func NewOpenAIStandardAdapter(baseURL, model string, opts ...ClientOption) *OpenAIStandardAdapter {
	return &OpenAIStandardAdapter{
		client: NewClient(baseURL, opts...),
		model:  model,
	}
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

	req := CompletionRequest{
		Model:       a.model,
		Prompt:      prompt,
		Temperature: 0.3,
		Stream:      false,
	}

	body, err := a.client.Post(context.Background(), "/completions", req)
	if err != nil {
		return "", fmt.Errorf("completion request failed: %w", err)
	}

	var resp CompletionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse completion response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no completion choices")
	}

	result := cleanResponse(resp.Choices[0].Text)
	if result == "" {
		return "", fmt.Errorf("LLM returned empty message for chunk")
	}
	return result, nil
}

// DecideCommit determines what files to stage based on user instruction and git status.
func (a *OpenAIStandardAdapter) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	prompt, err := prompts.Render(prompts.GetDecideCommit(), prompts.BuildDecideParams(instruction, gitStatus, untracked, modified, deleted))
	if err != nil {
		return domain.CommitIntent{}, fmt.Errorf("render decide commit prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, true)
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
	prompt, err := prompts.Render(prompts.InterpretGitOp, prompts.BuildInterpretParams(op, instruction, context))
	if err != nil {
		return nil, fmt.Errorf("render interpret prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, true)
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

// PreWarm sends a minimal completion request to load the model into memory.
// This is useful for local backends (vLLM, LM Studio, llama.cpp) where the
// first request can be slow while the model loads.
func (a *OpenAIStandardAdapter) PreWarm() error {
	req := CompletionRequest{
		Model:     a.model,
		Prompt:    ".",
		MaxTokens: 1,
		Stream:    false,
	}
	_, err := a.client.Post(context.Background(), "/completions", req)
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

	response, err := a.chatCompletion(prompt, false)
	if err != nil {
		return false, fmt.Errorf("verify secrets failed: %w", err)
	}
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(response)), "YES"), nil
}

// AuditBinaryContent determines if content is binary noise or legitimate text.
func (a *OpenAIStandardAdapter) AuditBinaryContent(filename, content string) (bool, error) {
	contentPreview := truncate(content, 1024)

	prompt, err := prompts.Render(prompts.Get("binary_check"), map[string]string{
		"File":    filename,
		"Content": contentPreview,
	})
	if err != nil {
		return false, fmt.Errorf("render binary_check prompt: %w", err)
	}

	response, err := a.chatCompletion(prompt, false)
	if err != nil {
		return false, fmt.Errorf("binary audit failed: %w", err)
	}

	return strings.Contains(strings.ToUpper(response), "BINARY"), nil
}

// GenerateChangelog generates changelog from commits and returns it.
func (a *OpenAIStandardAdapter) GenerateChangelog(commits, previousChangelog, outputFile string) (string, error) {
	prompt, err := prompts.Render(prompts.Get("changelog_generate"), map[string]string{
		"commits": commits,
	})
	if err != nil {
		return "", fmt.Errorf("render changelog prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, false)
	if err != nil {
		return "", err
	}

	result = cleanResponse(result)
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

// RegenerateMessage generates new commit messages based on feedback.
func (a *OpenAIStandardAdapter) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	if len(previousMessages) != len(chunks) {
		return nil, fmt.Errorf("previous messages count %d does not match chunks count %d", len(previousMessages), len(chunks))
	}

	newMessages := make([]string, len(chunks))
	for i, chunk := range chunks {
		prompt, err := prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, feedback))
		if err != nil {
			return nil, fmt.Errorf("render regenerate prompt for chunk %d: %w", i, err)
		}

		req := CompletionRequest{
			Model:       a.model,
			Prompt:      prompt,
			Temperature: 0.3,
			Stream:      false,
		}

	body, err := a.client.Post(context.Background(), "/completions", req)
		if err != nil {
			return nil, fmt.Errorf("completion request for chunk %d failed: %w", i, err)
		}

		var resp CompletionResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse response for chunk %d: %w", i, err)
		}
		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("LLM returned no completion choices for chunk %d", i)
		}

		result := cleanResponse(resp.Choices[0].Text)
		if result == "" {
			return nil, fmt.Errorf("LLM returned empty message for chunk %d", i)
		}
		newMessages[i] = result
	}
	return newMessages, nil
}

// chatCompletion sends a prompt via /chat/completions and returns the response content.
// When jsonMode is true, the request includes format: "json" for structured output.
func (a *OpenAIStandardAdapter) chatCompletion(prompt string, jsonMode bool) (string, error) {
	req := ChatRequest{
		Model:    a.model,
		Messages: []ChatMessage{{Role: "user", Content: prompt}},
		Stream:   false,
	}
	if jsonMode {
		req.Format = "json"
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