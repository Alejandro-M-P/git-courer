// Package llm provides the LLM adapter factory for creating provider-specific adapters.
package llm

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/ollama"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// FactoryConfig holds the configuration for creating an LLM adapter via the factory.
type FactoryConfig struct {
	Provider      string
	BaseURL       string
	Model         string
	APIKey        string
	ContextWindow int
	Ollama        config.OllamaSubConfig
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
		lifecycle := ollama.NewOllamaLifecycle(host, cfg.Ollama.ModelsDir, cfg.Ollama.AutoStart,
			ollama.WithPreWarm(adapter.PreWarm),
		)
		return adapter, lifecycle, nil

	case "openai-compatible", "lmstudio", "vllm", "localai":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			return nil, nil, fmt.Errorf("base_url is required for provider %q", cfg.Provider)
		}

		var opts []openai_standard.ClientOption
		if cfg.APIKey != "" {
			opts = append(opts, openai_standard.WithAPIKey(cfg.APIKey))
		}

		adapter := openai_standard.NewOpenAIStandardAdapter(baseURL, cfg.Model, opts...)
		return adapter, adapter, nil

	default:
		validProviders := []string{"ollama", "openai-compatible", "lmstudio", "vllm", "localai"}
		return nil, nil, fmt.Errorf("unknown LLM provider %q; valid providers: %s",
			cfg.Provider, strings.Join(validProviders, ", "))
	}
}
