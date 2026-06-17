package mcp

import (
	"encoding/json"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
)

func TestFormatStatusJSON_SemanticKeys(t *testing.T) {
	status := domain.Status{
		Branch:  "main",
		IsClean: false,
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "M ", Staged: true},
		},
		Staged:    1,
		Modified:  0,
		Untracked: 0,
	}

	result := shared.FormatStatusJSON(status, 10, 0, "", "", "", "", "")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify semantic keys
	keys := []string{"branch", "clean", "total", "returned", "files", "staged"}
	for _, key := range keys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("Missing expected semantic key: %s", key)
		}
	}

	// Verify old short keys are NOT present
	shortKeys := []string{"b", "c", "t", "r", "f", "s"}
	for _, key := range shortKeys {
		if _, ok := parsed[key]; ok {
			// Some might overlap by chance, but 'b', 'c', 'f' shouldn't
			if key == "b" || key == "c" || key == "f" {
				t.Errorf("Found legacy short key: %s", key)
			}
		}
	}

	files := parsed["files"].([]interface{})
	if len(files) > 0 {
		f := files[0].(map[string]interface{})
		if _, ok := f["path"]; !ok {
			t.Error("Missing 'path' key in file item")
		}
		if _, ok := f["p"]; ok {
			t.Error("Found legacy 'p' key in file item")
		}
	}
}
