package telemetry

import (
	"testing"
)

func TestQualityEvaluator_EvaluateCommitMessage(t *testing.T) {
	eval := NewQualityEvaluator()

	tests := []struct {
		name     string
		message  string
		minScore float64
	}{
		{
			name:     "perfect conventional commit",
			message:  "feat: add telemetry support",
			minScore: 0.9,
		},
		{
			name:     "fix with scope",
			message:  "fix(git): resolve merge conflict",
			minScore: 0.9,
		},
		{
			name:     "non-conventional commit",
			message:  "fixed the bug and updated some things",
			minScore: 0.0,
		},
		{
			name:     "multi-line conventional commit",
			message:  "feat: add telemetry\n\nThis adds a new telemetry system to track LLM performance.",
			minScore: 0.9,
		},
		{
			name:     "breaking change conventional commit",
			message:  "feat!: breaking change message",
			minScore: 0.9,
		},
		{
			name:     "too short message",
			message:  "feat: x",
			minScore: 0.5, // Should maybe penalize?
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.EvaluateCommitMessage(tt.message)
			if result.Score < tt.minScore {
				t.Errorf("expected score >= %v, got %v for message %q", tt.minScore, result.Score, tt.message)
			}
		})
	}
}
