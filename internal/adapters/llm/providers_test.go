package llm

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/config"
)

// TestNewLLMAdapter_Ollama verifies that provider "ollama" returns an OpenAIStandardAdapter
// and a non-nil Lifecycle.
func TestNewLLMAdapter_Ollama(t *testing.T) {
	cfg := FactoryConfig{
		Provider: "ollama",
		BaseURL:  "http://localhost:11434",
		Model:    "gemma4:26b",
		Ollama: config.OllamaSubConfig{
			ModelsDir: "/tmp/models",
			AutoStart: false,
		},
	}

	adapter, lifecycle, err := NewLLMAdapter(cfg)
	if err != nil {
		t.Fatalf("NewLLMAdapter(ollama) error = %v", err)
	}
	if adapter == nil {
		t.Fatal("NewLLMAdapter(ollama) adapter = nil, want non-nil")
	}
	if lifecycle == nil {
		t.Error("NewLLMAdapter(ollama) lifecycle = nil, want non-nil (Ollama has lifecycle)")
	}
}

// TestNewLLMAdapter_OpenAICompatible verifies that provider "openai-compatible"
// returns an OpenAIStandardAdapter and non-nil Lifecycle.
func TestNewLLMAdapter_OpenAICompatible(t *testing.T) {
	cfg := FactoryConfig{
		Provider: "openai-compatible",
		BaseURL:  "http://localhost:8080/v1",
		Model:    "test-model",
	}

	adapter, lifecycle, err := NewLLMAdapter(cfg)
	if err != nil {
		t.Fatalf("NewLLMAdapter(openai-compatible) error = %v", err)
	}
	if adapter == nil {
		t.Fatal("NewLLMAdapter(openai-compatible) adapter = nil, want non-nil")
	}
	if lifecycle == nil {
		t.Error("NewLLMAdapter(openai-compatible) lifecycle should be non-nil (OpenAIStandardAdapter implements Lifecycle)")
	}
}

// TestNewLLMAdapter_LMStudio verifies that provider "lmstudio" returns
// an OpenAIStandardAdapter and non-nil Lifecycle.
func TestNewLLMAdapter_LMStudio(t *testing.T) {
	cfg := FactoryConfig{
		Provider: "lmstudio",
		BaseURL:  "http://localhost:1234/v1",
		Model:    "local-model",
	}

	adapter, lifecycle, err := NewLLMAdapter(cfg)
	if err != nil {
		t.Fatalf("NewLLMAdapter(lmstudio) error = %v", err)
	}
	if adapter == nil {
		t.Fatal("NewLLMAdapter(lmstudio) adapter = nil, want non-nil")
	}
	if lifecycle == nil {
		t.Error("NewLLMAdapter(lmstudio) lifecycle should be non-nil (OpenAIStandardAdapter implements Lifecycle)")
	}
}

// TestNewLLMAdapter_VLLM verifies that provider "vllm" returns
// an OpenAIStandardAdapter and non-nil Lifecycle.
func TestNewLLMAdapter_VLLM(t *testing.T) {
	cfg := FactoryConfig{
		Provider: "vllm",
		BaseURL:  "http://localhost:8000/v1",
		Model:    "meta-llama/Llama-3-8b",
	}

	adapter, lifecycle, err := NewLLMAdapter(cfg)
	if err != nil {
		t.Fatalf("NewLLMAdapter(vllm) error = %v", err)
	}
	if adapter == nil {
		t.Fatal("NewLLMAdapter(vllm) adapter = nil, want non-nil")
	}
	if lifecycle == nil {
		t.Error("NewLLMAdapter(vllm) lifecycle should be non-nil (OpenAIStandardAdapter implements Lifecycle)")
	}
}

// TestNewLLMAdapter_LocalAI verifies that provider "localai" returns
// an OpenAIStandardAdapter and non-nil Lifecycle.
func TestNewLLMAdapter_LocalAI(t *testing.T) {
	cfg := FactoryConfig{
		Provider: "localai",
		BaseURL:  "http://localhost:8080/v1",
		Model:    "gpt-3.5-turbo",
	}

	adapter, lifecycle, err := NewLLMAdapter(cfg)
	if err != nil {
		t.Fatalf("NewLLMAdapter(localai) error = %v", err)
	}
	if adapter == nil {
		t.Fatal("NewLLMAdapter(localai) adapter = nil, want non-nil")
	}
	if lifecycle == nil {
		t.Error("NewLLMAdapter(localai) lifecycle should be non-nil (OpenAIStandardAdapter implements Lifecycle)")
	}
}

// TestNewLLMAdapter_InvalidProvider verifies that an unknown provider
// returns an error listing valid providers.
func TestNewLLMAdapter_InvalidProvider(t *testing.T) {
	cfg := FactoryConfig{
		Provider: "unknown",
		BaseURL:  "http://localhost:9999",
		Model:    "test",
	}

	adapter, lifecycle, err := NewLLMAdapter(cfg)
	if err == nil {
		t.Fatal("NewLLMAdapter(unknown) should return an error")
	}
	if adapter != nil {
		t.Error("NewLLMAdapter(unknown) adapter should be nil on error")
	}
	if lifecycle != nil {
		t.Error("NewLLMAdapter(unknown) lifecycle should be nil on error")
	}

	// Error message should list valid providers
	errMsg := err.Error()
	for _, valid := range []string{"ollama", "openai-compatible", "lmstudio", "vllm", "localai"} {
		if !strings.Contains(errMsg, valid) {
			t.Errorf("Error message should mention valid provider %q, got: %s", valid, errMsg)
		}
	}
}

// TestNewLLMAdapter_MissingBaseURL verifies that an empty BaseURL for Ollama
// defaults to "http://localhost:11434/v1".
func TestNewLLMAdapter_MissingBaseURL(t *testing.T) {
	cfg := FactoryConfig{
		Provider: "ollama",
		BaseURL:  "", // empty — should default to http://localhost:11434/v1
		Model:    "gemma4:26b",
	}

	adapter, lifecycle, err := NewLLMAdapter(cfg)
	if err != nil {
		t.Fatalf("NewLLMAdapter with empty BaseURL error = %v", err)
	}
	if adapter == nil {
		t.Fatal("NewLLMAdapter with empty BaseURL adapter = nil, want non-nil")
	}
	if lifecycle == nil {
		t.Error("NewLLMAdapter with empty BaseURL lifecycle should be non-nil for ollama")
	}
}

// TestNewLLMAdapter_ConfigMigration verifies that a FactoryConfig built from
// ResolveLLMConfig() works correctly through the factory.
func TestNewLLMAdapter_ConfigMigration(t *testing.T) {
	// Simulate a legacy config with only Ollama fields
	cfg := &config.Config{
		Ollama: config.OllamaConfig{
			Host:          "http://myserver:11434",
			Model:         "legacy-model:7b",
			ContextWindow: 4096,
			AutoStart:     true,
			ModelsDir:     "/custom/models",
		},
	}

	// ResolveLLMConfig merges legacy into LLM config
	resolved := cfg.ResolveLLMConfig()

	// Build FactoryConfig from resolved
	factoryCfg := FactoryConfig{
		Provider:      resolved.Provider,
		BaseURL:       resolved.BaseURL,
		Model:         resolved.Model,
		ContextWindow: resolved.ContextWindow,
		Ollama:        resolved.Ollama,
	}

	adapter, lifecycle, err := NewLLMAdapter(factoryCfg)
	if err != nil {
		t.Fatalf("NewLLMAdapter from ResolveLLMConfig error = %v", err)
	}
	if adapter == nil {
		t.Fatal("NewLLMAdapter from ResolveLLMConfig adapter = nil, want non-nil")
	}
	if lifecycle == nil {
		t.Error("NewLLMAdapter from ResolveLLMConfig lifecycle should be non-nil for ollama")
	}
}

// TestNewLLMAdapter_AllProvidersReturnLifecycle verifies that ALL valid providers
// return a non-nil Lifecycle (no provider should return nil anymore).
func TestNewLLMAdapter_AllProvidersReturnLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		baseURL  string
		model    string
		ollama   config.OllamaSubConfig
	}{
		{
			name:     "ollama",
			provider: "ollama",
			baseURL:  "http://localhost:11434",
			model:    "gemma4:26b",
			ollama:   config.OllamaSubConfig{ModelsDir: "/tmp/models", AutoStart: false},
		},
		{
			name:     "openai-compatible",
			provider: "openai-compatible",
			baseURL:  "http://localhost:8080/v1",
			model:    "test-model",
		},
		{
			name:     "lmstudio",
			provider: "lmstudio",
			baseURL:  "http://localhost:1234/v1",
			model:    "local-model",
		},
		{
			name:     "vllm",
			provider: "vllm",
			baseURL:  "http://localhost:8000/v1",
			model:    "meta-llama/Llama-3-8b",
		},
		{
			name:     "localai",
			provider: "localai",
			baseURL:  "http://localhost:8080/v1",
			model:    "gpt-3.5-turbo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := FactoryConfig{
				Provider: tt.provider,
				BaseURL:  tt.baseURL,
				Model:    tt.model,
				Ollama:   tt.ollama,
			}

			llm, lifecycle, err := NewLLMAdapter(cfg)
			if err != nil {
				t.Fatalf("NewLLMAdapter(%s) error = %v", tt.provider, err)
			}
			if llm == nil {
				t.Errorf("NewLLMAdapter(%s) LLM = nil, want non-nil", tt.provider)
			}
			// Only Ollama returns a lifecycle (process management).
			// Other backends are managed externally.
			if tt.provider == "ollama" && lifecycle == nil {
				t.Errorf("NewLLMAdapter(%s) Lifecycle = nil, want non-nil (Ollama needs process management)", tt.provider)
			}
		})
	}
}
