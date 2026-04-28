package ollama

import (
	"net/http"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// OllamaAdapter wraps an OpenAIStandardAdapter and adds Ollama-specific features:
// lifecycle management (auto-start, pre-warm), model resolution (/api/tags),
// and JSON support detection (/api/generate).
//
// The standard adapter always uses OpenAI-compatible /v1/ endpoints
// (Ollama >= v0.1.25). For lifecycle and model resolution, the /api/ endpoints
// (Ollama-native) are used.
//
// All ports.LLM methods delegate to the inner OpenAIStandardAdapter.
// For think mode, the adapter can fall back to Ollama's native /api/generate
// endpoint directly.
//
// Implements both ports.LLM and ports.Lifecycle.
type OllamaAdapter struct {
	standard *openai_standard.OpenAIStandardAdapter
	lifecycle *OllamaLifecycle
	resolver  *ModelResolver
}

// AdapterOption is a functional option for OllamaAdapter configuration.
type AdapterOption func(*OllamaAdapter)

// WithAdapterHTTPClient sets a custom HTTP client for both the standard adapter
// and the Ollama-specific components (lifecycle, resolver).
func WithAdapterHTTPClient(client *http.Client) AdapterOption {
	return func(a *OllamaAdapter) {
		standardBaseURL := a.lifecycle.host + "/v1"
		a.standard = openai_standard.NewOpenAIStandardAdapter(
			standardBaseURL,
			a.lifecycle.model,
			openai_standard.WithHTTPClient(client),
		)
		a.lifecycle.httpClient = client
		a.resolver = NewModelResolver(a.lifecycle.host, a.lifecycle.model, WithResolverHTTPClient(client))
		a.lifecycle.resolver = a.resolver
	}
}

// NewOllamaAdapter creates a new OllamaAdapter that wraps an OpenAIStandardAdapter
// for standard endpoints and adds Ollama-specific lifecycle management.
// The standard adapter always uses host+"/v1" for OpenAI-compatible /v1/ endpoints
// (Ollama >= v0.1.25).
func NewOllamaAdapter(host, model, modelsDir string, autoStart bool, opts ...AdapterOption) *OllamaAdapter {
	host = strings.TrimRight(host, "/")

	// The standard adapter's base URL always includes /v1 for OpenAI-compatible
	// endpoints (/v1/chat/completions, /v1/completions, /v1/models).
	standardBaseURL := host + "/v1"

	ol := NewOllamaLifecycle(host, model, modelsDir, autoStart)

	a := &OllamaAdapter{
		standard: openai_standard.NewOpenAIStandardAdapter(standardBaseURL, model),
		lifecycle: ol,
		resolver:  ol.resolver,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// StandardBaseURL returns the base URL used by the internal standard adapter.
// It always returns host+"/v1" for OpenAI-compatible endpoints.
func (a *OllamaAdapter) StandardBaseURL() string {
	return a.lifecycle.host + "/v1"
}

// GenerateChunkMessage delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return a.standard.GenerateChunkMessage(chunk)
}

// DecideCommit delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return a.standard.DecideCommit(instruction, gitStatus, untracked, modified, deleted)
}

// InterpretGitOp delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	return a.standard.InterpretGitOp(op, instruction, context)
}

// SetRetryContext delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) SetRetryContext(previousMessage string) {
	a.standard.SetRetryContext(previousMessage)
}

// ClearRetryContext delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) ClearRetryContext() {
	a.standard.ClearRetryContext()
}

// IsAvailable delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) IsAvailable() bool {
	return a.standard.IsAvailable()
}

// VerifySecrets delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return a.standard.VerifySecrets(diff, findings)
}

// AuditBinaryContent delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) AuditBinaryContent(filename, content string) (bool, error) {
	return a.standard.AuditBinaryContent(filename, content)
}

// GenerateChangelog delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) GenerateChangelog(commits, previousChangelog, outputFile string) (string, error) {
	return a.standard.GenerateChangelog(commits, previousChangelog, outputFile)
}

// RegenerateMessage delegates to the OpenAI standard adapter.
func (a *OllamaAdapter) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return a.standard.RegenerateMessage(previousMessages, feedback, chunks)
}

// ResolveModel resolves the model via the Ollama-specific /api/tags endpoint.
func (a *OllamaAdapter) ResolveModel() error {
	return a.resolver.Resolve()
}

// EnsureRunning delegates to the OllamaLifecycle.
func (a *OllamaAdapter) EnsureRunning() (bool, error) {
	return a.lifecycle.EnsureRunning()
}

// PreWarm delegates to the OllamaLifecycle.
func (a *OllamaAdapter) PreWarm() error {
	return a.lifecycle.PreWarm()
}

// Stop delegates to the OllamaLifecycle.
func (a *OllamaAdapter) Stop() {
	a.lifecycle.Stop()
}

// Compile-time interface checks.
var _ ports.LLM = (*OllamaAdapter)(nil)
var _ ports.Lifecycle = (*OllamaAdapter)(nil)