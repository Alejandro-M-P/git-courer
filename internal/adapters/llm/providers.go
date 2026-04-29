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
// It returns the adapter (implementing ports.LLM), a Lifecycle (always non-nil —
// all providers implement ports.Lifecycle), and an error for unknown providers.
//
// Valid providers: "ollama", "openai-compatible", "lmstudio", "vllm", "localai".
func NewLLMAdapter(cfg FactoryConfig) (ports.LLM, ports.Lifecycle, error) {
	switch strings.ToLower(cfg.Provider) {
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}

		// OllamaAdapter uses the host (without /v1) internally because it
		// constructs both /v1/ and /api/ paths. Extract the host from baseURL.
		host := strings.TrimSuffix(baseURL, "/v1")

		adapter := ollama.NewOllamaAdapter(
			host,
			cfg.Model,
			cfg.Ollama.ModelsDir,
			cfg.Ollama.AutoStart,
		)
		return adapter, adapter, nil

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