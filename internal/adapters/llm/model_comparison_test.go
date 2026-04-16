//go:build integration

package llm

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestModelAutoDetect tests JSON support auto-detection on all models
func TestModelAutoDetect(t *testing.T) {
	models := []string{"gemma4:26b", "qwen3.5:latest", "qwen3.5:0.8b"}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			adapter := New("http://localhost:11434", model, "")
			if !adapter.IsAvailable() {
				t.Skip("Ollama not running")
			}

			err := adapter.ResolveModel()
			if err != nil {
				t.Logf("Model %s not available: %v", model, err)
				return
			}

			t.Logf("Model: %s | supportsJSON: %v", adapter.model, adapter.supportsJSON)
		})
	}
}

// TestCommitMessageAllModels tests commit message generation on all models
func TestCommitMessageAllModels(t *testing.T) {
	models := []string{"gemma4:26b", "qwen3.5:latest"}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			adapter := New("http://localhost:11434", model, "")
			if !adapter.IsAvailable() {
				t.Skip("Ollama not running")
			}

			adapter.ResolveModel()

			chunk := domain.DiffChunk{
				Files: []string{"test.go"},
				Diff:  "+func test() {}",
			}

			result, err := adapter.GenerateChunkMessage(chunk)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}

			t.Logf("Model: %s | supportsJSON: %v | Result: %s", adapter.model, adapter.supportsJSON, result)
		})
	}
}

// TestDecideCommitAllModels tests decide commit on all models
func TestDecideCommitAllModels(t *testing.T) {
	models := []string{"qwen3.5:latest", "qwen3.5:0.8b", "gemma4:26b"}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			adapter := New("http://localhost:11434", model, "")
			if !adapter.IsAvailable() {
				t.Skip("Ollama not running")
			}

			adapter.ResolveModel()

			result, err := adapter.DecideCommit("add new file", "", "new.go", "", "")
			if err != nil {
				t.Logf("Error: %v", err)
			} else {
				t.Logf("Model: %s | supportsJSON: %v | include_untracked: %v", adapter.model, adapter.supportsJSON, result.IncludeUntracked)
			}
		})
	}
}
