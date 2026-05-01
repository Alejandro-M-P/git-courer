package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/telemetry"
)

var (
	LLMHost   = envOr("OLLAMA_HOST", "http://localhost:11434")
	LLMModel  = envOr("OLLAMA_MODEL", "qwen3:0.8b")
	LLMApiKey = os.Getenv("OLLAMA_API_KEY")
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var (
	telemetryOnce      sync.Once
	telemetryCollector telemetry.TelemetryCollector
)

func GetTelemetryCollector() telemetry.TelemetryCollector {
	telemetryOnce.Do(func() {
		if os.Getenv("TELEMETRY") == "1" {
			dir := filepath.Join(".gcourer", "telemetry")
			collector, err := telemetry.NewJSONCollector(dir)
			if err != nil {
				fmt.Printf("warning: failed to initialize telemetry collector: %v\n", err)
				return
			}
			telemetryCollector = collector
		}
	})
	return telemetryCollector
}

func RequireOllama(t *testing.T) ports.LLM {
	t.Helper()
	baseURL := strings.TrimSuffix(LLMHost, "/") + "/v1"
	var opts []openai_standard.ClientOption
	if LLMApiKey != "" {
		opts = append(opts, openai_standard.WithAPIKey(LLMApiKey))
	}
	adapter := openai_standard.NewOpenAIStandardAdapter(baseURL, LLMModel, opts...)
	if !adapter.IsAvailable() {
		t.Skipf("Ollama not running at %s — start with: ollama serve (model: %s)", baseURL, LLMModel)
	}
	if err := adapter.PreWarm(); err != nil {
		t.Logf("pre-warm warning: %v", err)
	}

	if collector := GetTelemetryCollector(); collector != nil {
		return telemetry.NewTelemetryDecorator(adapter, collector)
	}

	return adapter
}
