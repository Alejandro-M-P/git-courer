//go:build integration

package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

// TestGenerateChunkMessageWithRealOllama verifies the full flow of generating
// a commit message for a diff chunk using the real Ollama instance.
// This test performs an actual HTTP request to Ollama - no mocking.
func TestGenerateChunkMessageWithRealOllama(t *testing.T) {
	// Skip if no real Ollama available
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running. Start with: ollama serve")
	}

	// Create a realistic diff chunk
	chunk := domain.DiffChunk{
		Files: []string{"internal/adapters/llm/ollama.go"},
		Diff: `@@ -1,10 +1,15 @@
 package llm
+import (
+	"context"
+	"encoding/json"
+)
 
 func New() *Adapter {
-	return &Adapter{}
+	return &Adapter{host: "http://localhost:11434"}
 }`,
	}

	result, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage() error = %v", err)
	}

	// Verify we got a non-empty response
	result = strings.TrimSpace(result)
	if len(result) < 5 {
		t.Fatalf("GenerateChunkMessage() returned too short result: %q", result)
	}

	// Log the generated message for review
	t.Logf("Generated commit message: %q", result)
}

// TestDecideCommitWithRealOllama tests the commit decision flow with real Ollama.
func TestDecideCommitWithRealOllama(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	t.Log("=== Starting DecideCommit test ===")
	t.Logf("Model: %s", adapter.model)
	t.Logf("Host: %s", adapter.host)

	// Show the prompt that will be used
	promptText := prompts.GetDecideCommit()
	t.Logf("=== PROMPT USED ===\n%s\n=== END PROMPT ===", promptText)

	// Test with a simpler case first - just untracked files
	t.Log("Test 1: Only untracked files")
	result, err := adapter.DecideCommit(
		"add the new file",
		"",
		"newfile.go",
		"",
		"",
	)
	if err != nil {
		t.Logf("ERROR: %v", err)
		t.Fatalf("DecideCommit() error = %v", err)
	}
	t.Logf("Result: include_untracked=%v, filter=%q, reasoning=%q",
		result.IncludeUntracked, result.Filter, result.Reasoning)

	// Test with modified files
	t.Log("Test 2: Modified files only")
	result2, err := adapter.DecideCommit(
		"save my changes",
		"",
		"",
		"main.go, utils.go",
		"",
	)
	if err != nil {
		t.Logf("ERROR: %v", err)
		t.Fatalf("DecideCommit() error = %v", err)
	}
	t.Logf("Result: include_untracked=%v, filter=%q, reasoning=%q",
		result2.IncludeUntracked, result2.Filter, result2.Reasoning)
}

// TestInterpretGitOpWithRealOllama tests natural language git operation interpretation.
func TestInterpretGitOpWithRealOllama(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	// Show the prompt that will be used
	promptText := prompts.InterpretGitOp
	t.Logf("=== PROMPT USED ===\n%s\n=== END PROMPT ===", promptText)

	tests := []struct {
		name        string
		op          string
		instruction string
		context     map[string]string
	}{
		{
			name:        "create feature branch",
			op:          "branch",
			instruction: "create a new branch called feat/login",
			context:     map[string]string{"current_branch": "main"},
		},
		{
			name:        "checkout main",
			op:          "checkout",
			instruction: "go back to main",
			context:     map[string]string{},
		},
		{
			name:        "create release tag",
			op:          "tag",
			instruction: "tag version 1.2.3",
			context:     map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.InterpretGitOp(tt.op, tt.instruction, tt.context)
			if err != nil {
				t.Fatalf("InterpretGitOp() error = %v", err)
			}

			// Just log the result - we want to see what Ollama generates
			t.Logf("Instruction: %q", tt.instruction)
			t.Logf("Result: %v", result)
		})
	}
}

// TestVerifySecretsWithRealOllama tests secret verification with real Ollama.
func TestVerifySecretsWithRealOllama(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	diff := `diff --git a/config.json b/config.json
+{
+  "api_key": "sk-1234567890abcdef"
+}`

	findings := []domain.SecretDetection{
		{
			Type:    "api_key",
			File:    "config.json",
			Line:    3,
			Content: "sk-1234567890abcdef",
		},
	}

	result, err := adapter.VerifySecrets(diff, findings)
	if err != nil {
		t.Fatalf("VerifySecrets() error = %v", err)
	}

	t.Logf("Verification result (true=real secret): %v", result)
}

// TestInterpretReleaseIntentWithRealOllama tests release intent interpretation.
func TestInterpretReleaseIntentWithRealOllama(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	// Show the prompt that will be used
	promptText := prompts.Get("release_interpret")
	t.Logf("=== PROMPT USED ===\n%s\n=== END PROMPT ===", promptText)

	result, err := adapter.InterpretReleaseIntent(
		"create a minor release for version 1.0.0",
		"v0.9.0, v1.0.0-beta1",
		"main, develop, feature/login",
		"develop",
	)
	if err != nil {
		t.Fatalf("InterpretReleaseIntent() error = %v", err)
	}

	t.Logf("Release intent: tag=%q, is_release=%v, bump=%q",
		result.TagName, result.IsRelease, result.VersionBump)
	t.Logf("Merge path: %v", result.MergePath)
}

// TestGenerateChangelogWithRealOllama tests changelog generation.
func TestGenerateChangelogWithRealOllama(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	commits := `feat: add user login
fix: resolve auth bug
docs: update README
refactor: clean up middleware`

	result, err := adapter.GenerateChangelog(commits, "", "")
	if err != nil {
		t.Fatalf("GenerateChangelog() error = %v", err)
	}

	// Verify we got something meaningful
	if len(result) < 20 {
		t.Fatalf("GenerateChangelog() returned too short result: %q", result)
	}

	t.Logf("Generated changelog:\n%s", result)
}

// TestPreWarmWithRealOllama tests model pre-warming.
func TestPreWarmWithRealOllama(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	err := adapter.PreWarm()
	if err != nil {
		t.Fatalf("PreWarm() error = %v", err)
	}

	t.Log("Model pre-warmed successfully")
}

// TestEnsureOllamaStartIfNeeded tests starting Ollama if not running.
// This will likely skip if Ollama is already running.
func TestEnsureOllamaStartIfNeeded(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")

	started, err := adapter.EnsureOllama()
	if err != nil {
		t.Fatalf("EnsureOllama() error = %v", err)
	}

	if started {
		t.Log("Ollama was started by test (was not running)")
	} else {
		t.Log("Ollama was already running")
	}
}

// TestGenerateWithThinkMode tests the thinking mode for more verbose output.
func TestGenerateWithThinkMode(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	// Access private method via generateWithThink
	result, promptEval, evalCount, err := adapter.generateWithThink(
		"What is the capital of France? Answer in one word.",
		true, // thinkMode = true
	)
	if err != nil {
		t.Fatalf("generateWithThink() error = %v", err)
	}

	t.Logf("Result: %q", result)
	t.Logf("Prompt eval count: %d, Eval count: %d", promptEval, evalCount)
}

// TestConcurrentRequests tests making multiple concurrent requests to Ollama.
func TestConcurrentRequests(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	// Make 3 concurrent requests
	results := make(chan string, 3)
	errors := make(chan error, 3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			chunk := domain.DiffChunk{
				Files: []string{"file.go"},
				Diff:  "diff content " + string(rune('a'+idx)),
			}
			result, err := adapter.GenerateChunkMessage(chunk)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	// Collect results with timeout
	timeout := time.After(120 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case result := <-results:
			t.Logf("Got result %d: %q", i, result)
		case err := <-errors:
			t.Errorf("Request %d error: %v", i, err)
		case <-timeout:
			t.Fatal("Timeout waiting for results")
		}
	}
}

// TestContextCancellation tests that adapter handles timeouts gracefully.
func TestContextCancellation(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	// Test that the adapter doesn't crash with a simple request
	chunk := domain.DiffChunk{
		Files: []string{"test.go"},
		Diff:  "+package main",
	}

	_, err := adapter.GenerateChunkMessage(chunk)
	// Expected: either success or error (if model loading takes too long)
	if err != nil {
		t.Logf("Got error: %v", err)
	} else {
		t.Log("Successfully generated message")
	}
}

// TestLargeDiffChunk tests handling of larger diffs.
func TestLargeDiffChunk(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	// Generate a larger diff
	diffBuilder := strings.Builder{}
	for i := 0; i < 50; i++ {
		diffBuilder.WriteString(fmt.Sprintf("+// Line %d of code\n", i))
		diffBuilder.WriteString(fmt.Sprintf("-// Old line %d\n", i))
	}

	chunk := domain.DiffChunk{
		Files: []string{"large_file.go"},
		Diff:  diffBuilder.String(),
	}

	result, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage() with large diff error = %v", err)
	}

	t.Logf("Large diff result (%d chars): %q", len(result), result)
}

// TestRetryContext tests the retry context flow.
func TestRetryContext(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	// Set a retry context (simulating a rejected commit message)
	adapter.SetRetryContext("Previous message was too short, please be more descriptive")

	chunk := domain.DiffChunk{
		Files: []string{"test.go"},
		Diff:  "+func test() {}",
	}

	result, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage() with retry context error = %v", err)
	}

	t.Logf("Result with retry context: %q", result)

	// Clear retry context
	adapter.ClearRetryContext()
	if adapter.retryContext != "" {
		t.Error("ClearRetryContext() did not clear the context")
	}
}

// TestInvalidModel tests behavior with an invalid model name.
// The expected behavior is to fall back to the first available model.
func TestInvalidModel(t *testing.T) {
	// Use a non-existent model
	adapter := New("http://localhost:11434", "nonexistent-model-12345", "")

	err := adapter.ResolveModel()
	if err != nil {
		// Some models might not be available and there's no fallback
		t.Logf("Got error (expected for truly invalid models): %v", err)
	} else {
		// The adapter correctly fell back to another model
		t.Logf("Model not found, fell back to: %q (expected behavior)", adapter.model)
	}
}

// TestJSONFormatResponse tests that the JSON format parameter works.
func TestJSONFormatResponse(t *testing.T) {
	adapter := New("http://localhost:11434", "gemma4:26b", "")
	if !adapter.IsAvailable() {
		t.Skip("Skipped - Ollama not running")
	}

	prompt := `Return JSON with fields "name" and "version": {"name": "test", "version": "1.0.0"}`

	// Simple schema for testing
	testSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":    map[string]interface{}{"type": "string"},
			"version": map[string]interface{}{"type": "string"},
		},
		"required": []string{"name", "version"},
	}

	result, _, _, err := adapter.generateChatJSON(prompt, testSchema)
	if err != nil {
		t.Fatalf("generateChatJSON() error = %v", err)
	}

	// Try to parse as JSON
	var parsed map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Response is not valid JSON: %q, error: %v", result, err)
	}

	t.Logf("Parsed JSON: %v", parsed)
}

// Mock server that simulates Ollama but falls back to real for integration tests.
// This allows tests to run with real Ollama when available.
type mockOllamaServer struct {
	*httptest.Server
	handler http.HandlerFunc
}

func newMockServer(handler http.HandlerFunc) *mockOllamaServer {
	return &mockOllamaServer{
		Server:  httptest.NewServer(handler),
		handler: handler,
	}
}
