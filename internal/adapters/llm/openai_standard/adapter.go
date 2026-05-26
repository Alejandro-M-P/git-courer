package openai_standard

import (
	"context"
	"fmt"
	"strings"

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

// ollamaKeepAlive is how long Ollama keeps the model loaded after the last request.
const ollamaKeepAlive = "5m"

// OpenAIStandardAdapter implements ports.LLM and ports.Lifecycle via
// OpenAI-compatible endpoints.
type OpenAIStandardAdapter struct {
	client       *Client
	provider     string
	model        string
	context      string
	retryContext string
	why          string
	numParallel  int
	numCtx       int
}

// Compile-time interface check.
var _ ports.Lifecycle = (*OpenAIStandardAdapter)(nil)

// NewOpenAIStandardAdapter creates a new adapter for OpenAI-compatible backends.
func NewOpenAIStandardAdapter(baseURL, model string, opts ...ClientOption) *OpenAIStandardAdapter {
	adapter := &OpenAIStandardAdapter{
		client:      NewClient(baseURL, opts...),
		model:       model,
		numParallel: 1,
		numCtx:      0, // set via SetNumCtx at runtime
	}

	// Auto-detect Ollama by URL so num_ctx and other Ollama options are sent.
	if strings.Contains(baseURL, ":11434") || strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") {
		adapter.provider = "ollama"
	}

	return adapter
}

func (a *OpenAIStandardAdapter) SetProvider(p string) {
	a.provider = p
}

// SetNumParallel bounds concurrent LLM calls. Values <= 0 are treated as 1.
func (a *OpenAIStandardAdapter) SetNumParallel(n int) {
	if n <= 0 {
		n = 1
	}
	a.numParallel = n
}

// SetNumCtx sets the context window size for Ollama num_ctx injection.
// The adapter only injects num_ctx when this value is > 0.
func (a *OpenAIStandardAdapter) SetNumCtx(n int) {
	a.numCtx = n
}

// AuditBinaryContent determines if content is binary noise using a fast heuristic.
func (a *OpenAIStandardAdapter) AuditBinaryContent(filename, content string) (bool, error) {
	return prompts.IsBinary([]byte(content)), nil
}

// SetContext stores the project context string for prompt injection.
func (a *OpenAIStandardAdapter) SetContext(ctx string) {
	a.context = ctx
}

// SetWhy stores the user's reason for the change for prompt injection.
func (a *OpenAIStandardAdapter) SetWhy(why string) {
	a.why = why
}

// ClearWhy resets the why field, restoring zero-regression behavior.
func (a *OpenAIStandardAdapter) ClearWhy() {
	a.why = ""
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
