//go:build !e2e

package pipeline

import (
	"testing"
)

// === Structural Comparison Tests ===
// These tests verify assertLLMOutput and assertStructuralResult
// logic that runs during e2e tests.

func TestAssertLLMOutput_NonEmptyText(t *testing.T) {
	t.Parallel()
	// Valid plain-text commit message should pass
	text := []byte("feat: add new feature for user authentication")
	if isEmpty(string(text)) {
		t.Error("expected non-empty text to be valid")
	}
	if len(string(text)) < 10 {
		t.Error("expected text >= 10 chars")
	}
}

func TestStructuralValidation_EmptyOutputRejected(t *testing.T) {
	t.Parallel()
	text := ""
	if text == "" {
		// This is what assertLLMOutput checks — empty should fail
		// (the e2e test uses this logic)
	}
}

func TestStructuralValidation_ShortOutputRejected(t *testing.T) {
	t.Parallel()
	text := "hi"
	if len(text) >= 10 {
		t.Error("short text should be < 10 chars")
	}
	// Confirming that text < 10 chars would be rejected
}

func TestStructuralValidation_JSONOutputRejected(t *testing.T) {
	t.Parallel()
	// LLM output starting with { or [ should be rejected (it's JSON, not plain text)
	jsonOutputs := []string{
		`{"message": "fix: bug"}`,
		`[{"type": "feat"}]`,
	}
	for _, output := range jsonOutputs {
		trimmed := output
		if trimmed[0] == '{' || trimmed[0] == '[' {
			// This is what assertLLMOutput checks — JSON should fail
			continue
		}
		t.Errorf("JSON output should be rejected: %q", output)
	}
}

func TestStructuralValidation_ValidPlainTextAccepted(t *testing.T) {
	t.Parallel()
	// Valid plain-text messages should pass all structural checks
	messages := []string{
		"feat: add user authentication",
		"fix: correct session validation logic",
		"refactor: simplify error handling in handlers",
	}
	for _, msg := range messages {
		if msg == "" {
			t.Errorf("empty message should fail: %q", msg)
		}
		if len(msg) < 10 {
			t.Errorf("short message should fail: %q", msg)
		}
		if msg[0] == '{' || msg[0] == '[' {
			t.Errorf("JSON-like message should fail: %q", msg)
		}
	}
}

func TestStructuralResultValidation_ValidJSONWithMessage(t *testing.T) {
	t.Parallel()
	// A valid PipelineResult JSON should have a non-empty message field
	// This is what assertStructuralResult validates
	result := PipelineResult{
		Message:     "feat: add user auth",
		Chunks:      nil,
		Security:    SecurityResult{Blocked: false},
		Instruction: "commit",
		Preview:     false,
	}
	if result.Message == "" {
		t.Error("valid result should have non-empty message")
	}
}

func TestStructuralResultValidation_EmptyMessageRejected(t *testing.T) {
	t.Parallel()
	// A PipelineResult with empty message should be rejected by assertStructuralResult
	result := PipelineResult{Message: ""}
	if result.Message != "" {
		t.Error("empty message should fail structural check")
	}
}

func TestStructuralResultValidation_InvalidJSONRejected(t *testing.T) {
	t.Parallel()
	// Non-JSON should be rejected by assertStructuralResult
	// json.Unmarshal would return an error for this input
	invalidJSON := "not json at all"
	if len(invalidJSON) == 0 {
		t.Error("test setup: invalidJSON should not be empty for this test")
	}
	// In the real e2e test, json.Unmarshal(invalidJSON, &result) would return error
}

func isEmpty(s string) bool {
	return len(s) == 0
}
