//go:build integration

package llm

import (
	"os"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestPromptMatrix runs a suite of prompt tests across multiple models.
// Configure models via environment variable: GC_TEST_MODELS="gemma4:26b,qwen3.5:latest"
func TestPromptMatrix(t *testing.T) {
	modelsEnv := os.Getenv("GC_TEST_MODELS")
	var models []string
	if modelsEnv == "" {
		models = []string{"gemma4:26b"} // Default model
	} else {
		models = strings.Split(modelsEnv, ",")
	}

	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		t.Run(modelName, func(t *testing.T) {
			adapter := New("http://localhost:11434", modelName, "")
			if !adapter.IsAvailable() {
				t.Skipf("Model %s not available or Ollama offline", modelName)
			}

			// 1. Commit Message Accuracy
			t.Run("CommitMessage", func(t *testing.T) {
				chunk := domain.DiffChunk{
					Files: []string{"internal/adapters/llm/ollama.go"},
					Diff:  "+ func cleanJSON(s string) string { return strings.TrimSpace(s) }",
				}
				res, err := adapter.GenerateChunkMessage(chunk)
				if err != nil {
					t.Errorf("Error: %v", err)
				}
				if res == "" {
					t.Error("Empty commit message")
				}
			})

			// 2. JSON Intent Extraction (Merge)
			t.Run("MergeIntent", func(t *testing.T) {
				res, err := adapter.InterpretGitOp("merge", "merge feat/auth into main", map[string]string{
					"CurrentBranch": "main",
					"Branches":      "main\nfeat/auth",
				})
				if err != nil {
					t.Errorf("Error: %v", err)
				}
				if res["source"] != "feat/auth" {
					t.Errorf("Expected source 'feat/auth', got %q", res["source"])
				}
			})

			// 3. User-Facing Changelog
			t.Run("ChangelogGenerate", func(t *testing.T) {
				commits := "feat: add login button\nfix: handle nil pointer in auth"
				res, err := adapter.GenerateChangelog(commits, "", "")
				if err != nil {
					t.Errorf("Error: %v", err)
				}
				if strings.Contains(strings.ToLower(res), "nil pointer") {
					t.Errorf("Changelog contains technical jargon: %q", res)
				}
			})
		})
	}
}
