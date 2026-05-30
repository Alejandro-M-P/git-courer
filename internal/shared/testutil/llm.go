package testutil

import (
	"os"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

var (
	// LLMBaseURL is the base URL for the LLM service.
	// Checks LLM_BASE_URL first, falls back to OLLAMA_HOST for backward compatibility,
	// then defaults to http://localhost:11434.
	LLMBaseURL = EnvOr2("LLM_BASE_URL", "OLLAMA_HOST", "http://localhost:11434")

	// LLMModel is the model name for the LLM service.
	// Checks LLM_MODEL first, falls back to OLLAMA_MODEL for backward compatibility,
	// then defaults to phi3:latest.
	LLMModel = EnvOr2("LLM_MODEL", "OLLAMA_MODEL", "phi3:latest")

	// LLMAPIKey is the optional API key for the LLM service.
	// Checks LLM_API_KEY first, falls back to OLLAMA_API_KEY for backward compatibility.
	LLMAPIKey = EnvOr2("LLM_API_KEY", "OLLAMA_API_KEY", "")
)

// EnvOr2 checks primary env var first, then fallback, then returns default.
// This enables backward-compatible migration from OLLAMA_* to LLM_* env vars.
func EnvOr2(primary, fallback, defaultVal string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	if v := os.Getenv(fallback); v != "" {
		return v
	}
	return defaultVal
}

// envOr checks a single env var with a fallback value.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// RequireLLM creates an LLM adapter for testing, skipping the test if the
// LLM service is not available. It uses the LLM_* env vars (with OLLAMA_* fallback).
func RequireLLM(t *testing.T) ports.LLM {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping LLM integration test in short mode")
	}
	host := LLMBaseURL
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	baseURL := strings.TrimSuffix(host, "/") + "/v1"
	var opts []openai_standard.ClientOption
	if LLMAPIKey != "" {
		opts = append(opts, openai_standard.WithAPIKey(LLMAPIKey))
	}
	adapter := openai_standard.NewOpenAIStandardAdapter(baseURL, LLMModel, opts...)
	if _, err := adapter.EnsureRunning(); err != nil {
		t.Skipf("LLM service not running at %s (error: %v) — start with your backend (model: %s)", baseURL, err, LLMModel)
	}
	if err := adapter.PreWarm(); err != nil {
		t.Skipf("model %q not available for testing: %v", LLMModel, err)
	}
	return adapter
}

// RequireOllama is a deprecated alias for RequireLLM.
// Use RequireLLM instead — this function will be removed in a future release.
func RequireOllama(t *testing.T) ports.LLM {
	t.Helper()
	t.Log("DEPRECATED: RequireOllama is deprecated, use RequireLLM instead")
	return RequireLLM(t)
}
