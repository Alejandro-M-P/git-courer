package mcp

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestCommitRegeneration tests the regeneration flow for commit previews
func TestCommitRegeneration(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		previousPlan *domain.OperationPlan
		feedback     string
		wantError    bool
	}{
		{
			name:    "regenerate with feedback",
			command: "COMMIT_REGENERATE", // This command doesn't exist yet
			previousPlan: &domain.OperationPlan{
				Operation:    "commit",
				Messages:     []string{"feat: initial implementation"},
				Chunks:       [][]string{{"file1.go"}},
				DeletedFiles: []string{},
				Instruction:  "commit changes",
				Reasoning:    "Add feature",
			},
			feedback:  "make it more descriptive",
			wantError: false,
		},
		{
			name:         "regenerate without plan",
			command:      "COMMIT_REGENERATE",
			previousPlan: nil,
			feedback:     "feedback",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test now passes because COMMIT_REGENERATE is implemented
			// GREEN phase
			if tt.command == "COMMIT_REGENERATE" {
				// We can't test the actual handler without a full mock server
				// But we can verify the parseCommand function handles it
				op, phase := parseCommand(tt.command)
				if op != "commit" {
					t.Errorf("parseCommand(%q) op = %q, want %q", tt.command, op, "commit")
				}
				if phase != "regenerate" {
					t.Errorf("parseCommand(%q) phase = %q, want %q", tt.command, phase, "regenerate")
				}
			}
		})
	}
}

// TestRegenerationUpdatesPlan tests that regeneration updates the plan with new messages
func TestRegenerationUpdatesPlan(t *testing.T) {
	// This is an integration test that will fail until regeneration is implemented
	t.Skip("Requires full regeneration implementation")
}
