//go:build integration

package llm

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestPromptQuality verifies that the LLM generated messages are grounded,
// accurate, and follow security/safety best practices.
func TestPromptQuality(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Ollama not running")
	}

	t.Run("Grounding: No hallucinations", func(t *testing.T) {
		chunk := domain.DiffChunk{
			Files: []string{"math.go"},
			Diff: `--- math.go
+++ math.go
- func Add(a, b int) int {
+ func Sum(a, b int) int {`,
		}

		msg, err := adapter.GenerateChunkMessage(chunk)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		msgLower := strings.ToLower(msg)
		forbidden := []string{"performance", "readability", "optimization", "user", "feature"}
		for _, word := range forbidden {
			if strings.Contains(msgLower, word) {
				t.Errorf("Hallucination detected! Message contains %q which is not in the diff: %q", word, msg)
			}
		}
	})

	t.Run("Intent: Security Fix detection", func(t *testing.T) {
		chunk := domain.DiffChunk{
			Files: []string{"internal/workflow/commit.go"},
			Diff: `--- internal/workflow/commit.go
+++ internal/workflow/commit.go
@@ -10,0 +10,3 @@
+	if s.git == nil {
+		return "", fmt.Errorf("git adapter is required")
+	}
@@ -50,3 +50,0 @@
-	if s.git == nil {
-		return "", fmt.Errorf("git adapter is required")
-	}`,
		}

		msg, err := adapter.GenerateChunkMessage(chunk)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		if !strings.HasPrefix(strings.ToLower(msg), "fix:") {
			t.Errorf("Expected 'fix:' prefix for safety-check relocation, got: %q", msg)
		}
	})

	t.Run("Format: Breaking Change detection", func(t *testing.T) {
		chunk := domain.DiffChunk{
			Files: []string{"internal/core/ports/git.go"},
			Diff: `--- internal/core/ports/git.go
-	ListBranches() (string, error)
+	ListBranches(includeRemote bool) (string, error)`,
		}

		msg, err := adapter.GenerateChunkMessage(chunk)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		if !strings.Contains(msg, "!") && !strings.Contains(strings.ToUpper(msg), "BREAKING CHANGE") {
			t.Errorf("Expected breaking change notation (! or footer), got: %q", msg)
		}
	})
}
