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
	LLMModel  = envOr("OLLAMA_MODEL", "qwen3.5:0.8b")
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

func getProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func GetTelemetryCollector() telemetry.TelemetryCollector {
	telemetryOnce.Do(func() {
		if os.Getenv("TELEMETRY") == "1" {
			root := getProjectRoot()
			dir := filepath.Join(root, ".gcourer", "telemetry")
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

	if collector := GetTelemetryCollector(); collector != nil {
		return telemetry.NewTelemetryDecorator(adapter, collector)
	}

	return adapter
}
