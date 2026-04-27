package mcp

import (
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestParseCommand tests command parsing.
func TestParseCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantOp    string
		wantPhase string
	}{
		{"commit START", "COMMIT_START", "commit", "start"},
		{"commit APPLY", "COMMIT_APPLY", "commit", "apply"},
		{"commit ABORT", "COMMIT_ABORT", "commit", "abort"},
		{"branch CREATE START", "BRANCH_CREATE_START", "branch_create", "start"},
		{"branch DELETE APPLY", "BRANCH_DELETE_APPLY", "branch_delete", "apply"},
		{"release START", "RELEASE_START", "release", "start"},
		{"release APPLY", "RELEASE_APPLY", "release", "apply"},
		{"merge START", "MERGE_START", "merge", "start"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, phase := parseCommand(tt.command)
			if op != tt.wantOp {
				t.Errorf("parseCommand(%q) op = %q, want %q", tt.command, op, tt.wantOp)
			}
			if phase != tt.wantPhase {
				t.Errorf("parseCommand(%q) phase = %q, want %q", tt.command, phase, tt.wantPhase)
			}
		})
	}
}

// TestProcessingJSON tests the processing response JSON generation.
func TestProcessingJSON(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"normal message", "Commit is being processed"},
		{"empty message", ""},
		{"special chars", "Commit with <test> & 'quotes'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processingJSON(tt.message)

			var parsed map[string]interface{}
			if parseErr := json.Unmarshal([]byte(result), &parsed); parseErr != nil {
				t.Fatalf("processingJSON() returned invalid JSON: %v", parseErr)
			}

	if gotMsg, ok := parsed["status"]; !ok || gotMsg != "pending_approval" {
		t.Errorf("processingJSON() missing or wrong status: %v", gotMsg)
	}
		})
	}
}

// TestFormatStatus tests status formatting.
func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name   string
		status domain.Status
		want   string
	}{
		{"clean status", domain.Status{Branch: "main", IsClean: true}, "Branch: main\nWorking tree clean\n"},
		{"with files", domain.Status{
			Branch: "main",
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M "},
				{Path: "b.txt", Status: "??"},
			},
		}, "Branch: main\nM a.go\n??b.txt\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStatus(tt.status)
			if result != tt.want {
				t.Errorf("formatStatus() = %q, want %q", result, tt.want)
			}
		})
	}
}

// TestParseCommandEdgeCases tests edge cases not covered by the main table.
func TestParseCommandEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantOp    string
		wantPhase string
	}{
		{"TAG_CREATE_START", "TAG_CREATE_START", "tag_create", "start"},
		{"TAG_DELETE_APPLY", "TAG_DELETE_APPLY", "tag_delete", "apply"},
		{"empty command gives unknown", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, phase := parseCommand(tt.command)
			if op != tt.wantOp {
				t.Errorf("parseCommand(%q) op = %q, want %q", tt.command, op, tt.wantOp)
			}
			if phase != tt.wantPhase {
				t.Errorf("parseCommand(%q) phase = %q, want %q", tt.command, phase, tt.wantPhase)
			}
		})
	}
}

// TestProcessingJSONStructure verifies the JSON structure returned by processingJSON.
func TestProcessingJSONStructure(t *testing.T) {
	msg := "operation in progress"
	result := processingJSON(msg)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("processingJSON() returned invalid JSON: %v", err)
	}

	if parsed["status"] != "pending_approval" {
		t.Errorf("processingJSON().status = %v, want %q", parsed["status"], "pending_approval")
	}
	if parsed["preview"] != msg {
		t.Errorf("processingJSON().preview = %v, want %q", parsed["preview"], msg)
	}
}
