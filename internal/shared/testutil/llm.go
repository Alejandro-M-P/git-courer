package testutil

import (
	"os"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

var (
	LLMHost   = envOr("OLLAMA_HOST", "http://localhost:11434")
	LLMModel  = envOr("OLLAMA_MODEL", "qwen3.5:0.8b")
	LLMApiKey = os.Getenv("OLLAMA_API_KEY")
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func RequireOllama(t *testing.T) ports.LLM {
	t.Helper()
	host := LLMHost
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	baseURL := strings.TrimSuffix(host, "/") + "/v1"
	var opts []openai_standard.ClientOption
	if LLMApiKey != "" {
		opts = append(opts, openai_standard.WithAPIKey(LLMApiKey))
	}
	adapter := openai_standard.NewOpenAIStandardAdapter(baseURL, LLMModel, opts...)
	if _, err := adapter.EnsureRunning(); err != nil {
		t.Skipf("Real LLM not running at %s (error: %v) — start with your backend (model: %s)", baseURL, err, LLMModel)
	}
	if err := adapter.PreWarm(); err != nil {
		t.Skipf("model %q not available for testing: %v", LLMModel, err)
	}
	return adapter
}
