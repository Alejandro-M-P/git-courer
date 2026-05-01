package telemetry

import (
	"testing"
	"time"
)

func TestTelemetryStructs(t *testing.T) {
	now := time.Now()

	// LLMCall instantiation
	call := LLMCall{
		Timestamp: now,
		Operation: "GenerateCommitMessage",
		Model:     "ollama/llama3",
		Latency:   500 * time.Millisecond,
		Prompt:    "diff...",
		Response:  "feat: add telemetry",
		Success:   true,
	}

	if call.Operation != "GenerateCommitMessage" {
		t.Errorf("expected operation GenerateCommitMessage, got %s", call.Operation)
	}

	// Metric instantiation
	metric := Metric{
		Name:   "llm_latency_ms",
		Value:  500.0,
		Labels: map[string]string{"model": "llama3"},
	}

	if metric.Name != "llm_latency_ms" {
		t.Errorf("expected name llm_latency_ms, got %s", metric.Name)
	}

	// QualityResult instantiation
	result := QualityResult{
		Score:   0.85,
		Rules:   []string{"conventional_commit", "subject_length"},
		Summary: "Valid conventional commit message",
	}

	if result.Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", result.Score)
	}
}
