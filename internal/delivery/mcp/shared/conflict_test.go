package shared

import (
	"encoding/json"
	"testing"
)

func TestConflictResultJSON_StructuredFormat(t *testing.T) {
	tests := []struct {
		name              string
		files             []string
		hint              string
		wantStatus        string
		wantMessageKey    string
		wantConflictedKey string
	}{
		{
			name:              "conflict with multiple files",
			files:             []string{"main.go", "README.md"},
			hint:              "Resolve conflicts then stage files",
			wantStatus:        "conflict",
			wantMessageKey:    "message",
			wantConflictedKey: "conflicted_files",
		},
		{
			name:              "conflict with empty files",
			files:             []string{},
			hint:              "No files detected",
			wantStatus:        "conflict",
			wantMessageKey:    "message",
			wantConflictedKey: "conflicted_files",
		},
		{
			name:              "conflict with single file",
			files:             []string{"src/api/handler.go"},
			hint:              "Resolve and continue",
			wantStatus:        "conflict",
			wantMessageKey:    "message",
			wantConflictedKey: "conflicted_files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConflictResultJSON(tt.files, tt.hint)

			// Parse as JSON
			var parsed map[string]any
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("ConflictResultJSON returned invalid JSON: %v", err)
			}

			// Verify status field
			status, ok := parsed["status"].(string)
			if !ok {
				t.Errorf("missing or non-string 'status' field")
			} else if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}

			// Verify message field (used to be "hint")
			_, hasMessage := parsed[tt.wantMessageKey]
			if !hasMessage {
				t.Errorf("missing '%s' field in structured conflict result", tt.wantMessageKey)
			}

			// Verify conflicted_files field (used to be "files")
			filesRaw, hasFiles := parsed[tt.wantConflictedKey]
			if !hasFiles {
				t.Errorf("missing '%s' field in structured conflict result", tt.wantConflictedKey)
			} else {
				filesArr, ok := filesRaw.([]any)
				if !ok {
					t.Errorf("'%s' is not an array", tt.wantConflictedKey)
				}
				// Verify same number of files
				if len(filesArr) != len(tt.files) {
					t.Errorf("conflicted_files length = %d, want %d", len(filesArr), len(tt.files))
				}
			}

			// Old keys should NOT exist anymore
			if _, hasOldConflict := parsed["conflict"]; hasOldConflict {
				t.Errorf("old 'conflict' key should not exist in v2 format")
			}
			if _, hasOldHint := parsed["hint"]; hasOldHint {
				t.Errorf("old 'hint' key should not exist in v2 format, use 'message' instead")
			}
		})
	}
}

// Triangulation: verify the message content matches
func TestConflictResultJSON_MessageContent(t *testing.T) {
	result := ConflictResultJSON([]string{"a.go", "b.go"}, "Fix conflicts and try again")
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)

	msg, _ := parsed["message"].(string)
	if msg != "Fix conflicts and try again" {
		t.Errorf("message = %q, want %q", msg, "Fix conflicts and try again")
	}

	// Verify files array content
	filesRaw, _ := parsed["conflicted_files"].([]any)
	if len(filesRaw) != 2 || filesRaw[0].(string) != "a.go" || filesRaw[1].(string) != "b.go" {
		t.Errorf("conflicted_files has wrong content: %v", filesRaw)
	}
}
