package mcp

import (
	"encoding/json"
	"testing"

	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestValidateRequiredParam(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		key         string
		command     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty value returns error",
			params:      map[string]any{"name": ""},
			key:         "name",
			command:     "CREATE",
			wantErr:     true,
			errContains: "name is required for CREATE",
		},
		{
			name:        "missing key returns error",
			params:      map[string]any{},
			key:         "name",
			command:     "CREATE",
			wantErr:     true,
			errContains: "name is required for CREATE",
		},
		{
			name:    "non-empty value returns nil",
			params:  map[string]any{"name": "feature"},
			key:     "name",
			command: "CREATE",
			wantErr: false,
		},
		{
			name:    "non-empty value for branch param",
			params:  map[string]any{"branch": "main"},
			key:     "branch",
			command: "MERGE",
			wantErr: false,
		},
		{
			name:        "empty branch returns error for MERGE",
			params:      map[string]any{"branch": ""},
			key:         "branch",
			command:     "MERGE",
			wantErr:     true,
			errContains: "branch is required for MERGE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shared.ValidateRequiredParam(tt.params, tt.key, tt.command)
			if tt.wantErr {
				assert.Nil(t, err, "ValidateRequiredParam should not return a Go error")
				assert.NotNil(t, result, "result should not be nil for validation error")

				// Parse the JSON result to verify structure
				var parsed map[string]any
				text := result.Content[0].(mcpgo.TextContent).Text
				unmarshalErr := json.Unmarshal([]byte(text), &parsed)
				assert.NoError(t, unmarshalErr, "error result should be valid JSON")
				assert.Contains(t, parsed["error"], tt.errContains, "error message should contain expected text")
			} else {
				assert.Nil(t, result, "result should be nil when param is present")
			}
		})
	}
}

func TestSuggestCommand(t *testing.T) {
	validBranch := []string{"CREATE", "DELETE", "RENAME", "REMOTE_DELETE"}

	tests := []struct {
		name     string
		input    string
		valid    []string
		expected string
	}{
		{
			name:     "exact match returns empty (no hint needed)",
			input:    "CREATE",
			valid:    validBranch,
			expected: "CREATE",
		},
		{
			name:     "close typo returns suggestion",
			input:    "CREAT",
			valid:    validBranch,
			expected: "CREATE",
		},
		{
			name:     "prefix match",
			input:    "DELE",
			valid:    validBranch,
			expected: "DELETE",
		},
		{
			name:     "single char edit finds match",
			input:    "DELET",
			valid:    validBranch,
			expected: "DELETE",
		},
		{
			name:     "completely unknown returns empty",
			input:    "ZZZZZ",
			valid:    validBranch,
			expected: "",
		},
		{
			name:     "empty valid commands returns empty",
			input:    "CREATE",
			valid:    []string{},
			expected: "",
		},
		{
			name:     "stash commands suggestion",
			input:    "SAVE",
			valid:    []string{"SAVE", "POP", "APPLY", "DROP", "CLEAR", "SHOW"},
			expected: "SAVE",
		},
		{
			name:     "close typo for POP",
			input:    "PO",
			valid:    []string{"SAVE", "POP", "APPLY", "DROP", "CLEAR", "SHOW"},
			expected: "POP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shared.SuggestCommand(tt.input, tt.valid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		{name: "identical strings", a: "CREATE", b: "CREATE", expected: 0},
		{name: "single deletion", a: "CREATE", b: "CREAT", expected: 1},
		{name: "single insertion", a: "CREAT", b: "CREATE", expected: 1},
		{name: "single substitution", a: "CREATE", b: "CREATF", expected: 1},
		{name: "two edits", a: "kitten", b: "sitting", expected: 3},
		{name: "empty strings", a: "", b: "", expected: 0},
		{name: "one empty string", a: "abc", b: "", expected: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, shared.Levenshtein(tt.a, tt.b))
		})
	}
}

func TestValidateKnownParams(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		allowedKeys []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "all params known returns nil",
			params:      map[string]any{"command": "CREATE", "name": "feat"},
			allowedKeys: []string{"command", "name", "new_name", "force"},
			wantErr:     false,
		},
		{
			name:        "unknown param returns error",
			params:      map[string]any{"command": "CREATE", "nam": "feat"},
			allowedKeys: []string{"command", "name", "new_name", "force"},
			wantErr:     true,
			errContains: "unknown parameter: nam",
		},
		{
			name:        "empty params returns nil",
			params:      map[string]any{},
			allowedKeys: []string{"command", "name"},
			wantErr:     false,
		},
		{
			name:        "arg parameter rejected",
			params:      map[string]any{"command": "CREATE", "arg": "feat"},
			allowedKeys: []string{"command", "name"},
			wantErr:     true,
			errContains: "unknown parameter: arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shared.ValidateKnownParams(tt.params, tt.allowedKeys)
			if tt.wantErr {
				assert.Nil(t, err, "ValidateKnownParams should not return a Go error")
				assert.NotNil(t, result, "result should not be nil for unknown param error")

				var parsed map[string]any
				text := result.Content[0].(mcpgo.TextContent).Text
				unmarshalErr := json.Unmarshal([]byte(text), &parsed)
				assert.NoError(t, unmarshalErr, "error result should be valid JSON")
				assert.Contains(t, parsed["error"], tt.errContains)
			} else {
				assert.Nil(t, result, "result should be nil when all params are known")
			}
		})
	}
}
