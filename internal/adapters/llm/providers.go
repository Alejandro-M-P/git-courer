// Package llm provides the LLM adapter factory for creating provider-specific adapters.
package llm

import (
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/adapters/llm/ollama"
	"github.com/blak0p/git-courer/internal/adapters/llm/openai_standard"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// FactoryConfig holds the configuration for creating an LLM adapter via the factory.
type FactoryConfig struct {
	Provider      string
	BaseURL       string
	Model         string
	APIKey        string
	ContextWindow int
	NumParallel   int
}

// NewLLMAdapter creates an LLM adapter based on the provider specified in cfg.
// It returns the adapter (implementing ports.LLM), a Lifecycle, and an error
// for unknown providers.
//
// All backends use OpenAIStandardAdapter for LLM calls.
// OpenAIStandardAdapter also implements ports.Lifecycle and is returned as
// the lifecycle for non-Ollama providers. Only Ollama has a separate
// OllamaLifecycle for process management.
func NewLLMAdapter(cfg FactoryConfig) (ports.LLM, ports.Lifecycle, error) {
	// All providers use OpenAIStandardAdapter for LLM calls
	switch strings.ToLower(cfg.Provider) {
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}

		// Host for lifecycle (without /v1)
		host := strings.TrimSuffix(baseURL, "/v1")

		adapter := openai_standard.NewOpenAIStandardAdapter(baseURL, cfg.Model)
		adapter.SetNumParallel(cfg.NumParallel)
		adapter.SetProvider("ollama")
		// Ollama models dir and auto-start come from config (not in FactoryConfig after simplification)
		// These are handled by the OllamaLifecycle directly.
		lifecycle := ollama.NewOllamaLifecycle(host, "", true,
			ollama.WithPreWarm(adapter.PreWarm),
		)
		return adapter, lifecycle, nil

	default:
		// Any non-Ollama provider is treated as OpenAI-compatible.
		// This covers lmstudio, vllm, localai, openai-compatible, or any custom server
		// that exposes /v1/chat/completions.
		baseURL := cfg.BaseURL
		if baseURL == "" {
			return nil, nil, fmt.Errorf("base_url is required for provider %q", cfg.Provider)
		}

		var opts []openai_standard.ClientOption
		if cfg.APIKey != "" {
			opts = append(opts, openai_standard.WithAPIKey(cfg.APIKey))
		}

		adapter := openai_standard.NewOpenAIStandardAdapter(baseURL, cfg.Model, opts...)
		adapter.SetNumParallel(cfg.NumParallel)
		adapter.SetProvider(cfg.Provider)
		return adapter, adapter, nil
	}
}
