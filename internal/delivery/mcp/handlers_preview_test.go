package mcp

import (
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestHandleGitWriteReviewPreviewMode tests the preview mode for non-commit operations
func TestHandleGitWriteReviewPreviewMode(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		instruction string
		preview     bool
		wantStatus  string // "pending_approval" or "completed"
	}{
		{"branch create preview", "BRANCH_CREATE_START", "create branch for login", true, "pending_approval"},
		{"merge preview", "MERGE_START", "merge feat/login into main", true, "pending_approval"},
		{"branch delete preview", "BRANCH_DELETE_START", "delete old branch", true, "pending_approval"},
		{"branch create execute immediate", "BRANCH_CREATE_START", "create branch", false, "completed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a test skeleton that will fail initially
			// We'll implement the behavior later
			if tt.wantStatus != "pending_approval" {
				t.Skip("Test pending implementation")
			}
		})
	}
}

// TestPreviewJSONStructure tests that preview responses have structured JSON with options
func TestPreviewJSONStructure(t *testing.T) {
	wantFields := []string{
		"status",
		"show_to_user",
		"preview",
		"options",            // New field for human confirmation
		"structured_preview", // New structured preview field
		"files",              // Optional for commit preview
		"messages",           // Optional for commit preview
		"reasoning",          // Commit-specific reasoning field
	}

	// Test commitPlanJSON function
	plan := &domain.OperationPlan{
		Operation: "commit",
		Messages:  []string{"feat: add login page"},
		Chunks:    [][]string{{"internal/auth/login.go"}},
		Preview:   "Commit plan: feat: add login page",
		Reasoning: "Add authentication feature",
	}

	result := commitPlanJSON(plan)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("commitPlanJSON returned invalid JSON: %v", err)
	}

	for _, field := range wantFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("commitPlanJSON missing required field %s", field)
		}
	}

	// Verify structured_preview has correct schema
	previewRaw, ok := parsed["structured_preview"].(map[string]interface{})
	if !ok {
		t.Fatal("structured_preview field is not a map")
	}

	previewFields := []string{"header", "sections", "actions"}
	for _, field := range previewFields {
		if _, ok := previewRaw[field]; !ok {
			t.Errorf("structured_preview missing required field %s", field)
		}
	}

	// Verify options array matches actions labels
	options, ok := parsed["options"].([]interface{})
	if !ok {
		t.Fatal("options field is not an array")
	}

	actions, ok := previewRaw["actions"].([]interface{})
	if !ok {
		t.Fatal("structured_preview.actions is not an array")
	}

	if len(options) != len(actions) {
		t.Errorf("options length (%d) != actions length (%d)", len(options), len(actions))
	}
}

// TestParseCommandWithPreviewFlag tests command parsing with preview boolean
func TestParseCommandWithPreviewFlag(t *testing.T) {
	// Mock request with preview flag
	// We'll test the actual handler integration later
}

// TestStructuredPreviewOptions tests that preview JSON includes options array for user actions
func TestStructuredPreviewOptions(t *testing.T) {
	t.Run("commit preview options", func(t *testing.T) {
		plan := &domain.OperationPlan{
			Operation: "commit",
			Messages:  []string{"feat: add login page"},
			Chunks:    [][]string{{"internal/auth/login.go"}},
			Preview:   "Commit plan: feat: add login page",
			Reasoning: "Add authentication feature",
		}

		result := commitPlanJSON(plan)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("commitPlanJSON returned invalid JSON: %v", err)
		}

		options, ok := parsed["options"].([]interface{})
		if !ok {
			t.Fatalf("options field missing or wrong type in preview JSON")
		}

		expectedOptions := []string{"Execute", "Regenerate message", "Edit message", "Cancel"}
		if len(options) != len(expectedOptions) {
			t.Errorf("options array length = %d, want %d", len(options), len(expectedOptions))
		}

		for i, opt := range options {
			if optStr, ok := opt.(string); !ok || optStr != expectedOptions[i] {
				t.Errorf("options[%d] = %v, want %q", i, opt, expectedOptions[i])
			}
		}
	})
}

// TestPreviewModeDoesNotExecute tests that preview mode only returns plan, doesn't execute
func TestPreviewModeDoesNotExecute(t *testing.T) {
	// Integration test mock will be added later
	t.Skip("Integration test requires mock services")
}

// TestStructuredPreviewInCommitJSON verifies the structured preview field in commit JSON
func TestStructuredPreviewInCommitJSON(t *testing.T) {
	plan := &domain.OperationPlan{
		Operation: "commit",
		Messages:  []string{"feat: add login page", "fix: typo in README"},
		Chunks:    [][]string{{"internal/auth/login.go"}, {"README.md"}},
		Preview:   "Commit plan: authentication feature",
		Reasoning: "Adding login capability for users",
	}

	result := commitPlanJSON(plan)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("commitPlanJSON returned invalid JSON: %v", err)
	}

	// Verify reasoning field is present
	if reasoning, ok := parsed["reasoning"].(string); !ok || reasoning != plan.Reasoning {
		t.Errorf("reasoning field missing or incorrect: got %q, want %q", reasoning, plan.Reasoning)
	}

	// Verify structured_preview has commit-specific sections
	previewRaw, ok := parsed["structured_preview"].(map[string]interface{})
	if !ok {
		t.Fatal("structured_preview field is not a map")
	}

	sections, ok := previewRaw["sections"].([]interface{})
	if !ok {
		t.Fatal("sections field is not an array")
	}

	// Should have Operation, Messages, Files, Reasoning, Impact sections (at least 5)
	if len(sections) < 5 {
		t.Errorf("expected at least 5 sections in commit preview, got %d", len(sections))
	}

	// Check for specific section titles
	sectionTitles := make([]string, 0, len(sections))
	for _, s := range sections {
		if section, ok := s.(map[string]interface{}); ok {
			if title, ok := section["title"].(string); ok {
				sectionTitles = append(sectionTitles, title)
			}
		}
	}

	expectedTitles := []string{"Operation", "Messages", "Files", "Reasoning", "Impact"}
	foundCount := 0
	for _, expected := range expectedTitles {
		for _, found := range sectionTitles {
			if expected == found {
				foundCount++
				break
			}
		}
	}

	if foundCount < 4 { // At least 4 of 5 expected titles should be present
		t.Errorf("missing expected section titles: found %d of %d, titles: %v", foundCount, len(expectedTitles), sectionTitles)
	}
}

// TestStructuredPreviewInReleaseJSON verifies the structured preview field in release JSON
func TestStructuredPreviewInReleaseJSON(t *testing.T) {
	intent := &domain.ReleaseIntent{
		TagName:     "v1.2.0",
		VersionBump: "minor",
	}
	changelog := "## Added\n- Login feature\n- User profile page"
	warnings := []string{"No tests found for login feature", "Missing documentation"}
	ghAuth := "github token configured"

	result := releasePlanJSON(intent, changelog, warnings, ghAuth)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("releasePlanJSON returned invalid JSON: %v", err)
	}

	// Verify impact field is present
	if impact, ok := parsed["impact"].(string); !ok || impact == "" {
		t.Errorf("impact field missing or empty: %v", impact)
	}

	// Verify structured_preview has release-specific sections
	previewRaw, ok := parsed["structured_preview"].(map[string]interface{})
	if !ok {
		t.Fatal("structured_preview field is not a map")
	}

	sections, ok := previewRaw["sections"].([]interface{})
	if !ok {
		t.Fatal("sections field is not an array")
	}

	// Should have Operation, Version, Changelog, GitHub Auth, Warnings, Impact sections
	if len(sections) < 4 {
		t.Errorf("expected at least 4 sections in release preview, got %d", len(sections))
	}

	// Check for specific section titles based on provided data
	sectionTitles := make([]string, 0, len(sections))
	for _, s := range sections {
		if section, ok := s.(map[string]interface{}); ok {
			if title, ok := section["title"].(string); ok {
				sectionTitles = append(sectionTitles, title)
			}
		}
	}

	// Should contain these based on test data
	expectedInTitles := []string{"Operation", "Version", "Changelog", "GitHub Auth", "Warnings", "Impact"}
	foundCount := 0
	for _, expected := range expectedInTitles {
		for _, found := range sectionTitles {
			if expected == found {
				foundCount++
				break
			}
		}
	}

	// Should find at least Operation, Version, and Impact
	if foundCount < 3 {
		t.Errorf("missing core section titles: found %d of at least 3, titles: %v", foundCount, sectionTitles)
	}
}

// TestReadyJSONStructuredPreview verifies readyJSON includes structured preview
func TestReadyJSONStructuredPreview(t *testing.T) {
	tests := []struct {
		name    string
		preview string
	}{
		{"branch create preview", "Create branch feat/login from main"},
		{"merge preview", "Merge feat/login into develop"},
		{"tag delete preview", "Delete tag v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := readyJSON(tt.preview)
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("readyJSON returned invalid JSON: %v", err)
			}

			// Verify structured_preview field exists
			if _, ok := parsed["structured_preview"]; !ok {
				t.Fatal("readyJSON missing structured_preview field")
			}

			// Verify options field exists
			options, ok := parsed["options"].([]interface{})
			if !ok {
				t.Fatal("options field missing or wrong type")
			}

			// Generic operations should have Continue, Cancel, View details
			if len(options) < 2 {
				t.Errorf("generic operations should have at least 2 options, got %d", len(options))
			}

			// Verify structured_preview has generic sections
			previewRaw, ok := parsed["structured_preview"].(map[string]interface{})
			if !ok {
				t.Fatal("structured_preview field is not a map")
			}

			sections, ok := previewRaw["sections"].([]interface{})
			if !ok {
				t.Fatal("sections field is not an array")
			}

			// Should have at least Operation and Preview sections
			if len(sections) < 2 {
				t.Errorf("readyJSON should have at least 2 sections, got %d", len(sections))
			}

			// Verify that options labels match actions labels
			actions, ok := previewRaw["actions"].([]interface{})
			if !ok {
				t.Fatal("actions field is not an array")
			}

			if len(options) != len(actions) {
				t.Errorf("options length (%d) != actions length (%d)", len(options), len(actions))
			}
		})
	}
}

// TestOptionsMapping verifies that options labels correspond with structured_preview.actions
func TestOptionsMapping(t *testing.T) {
	t.Run("commit options mapping", func(t *testing.T) {
		plan := &domain.OperationPlan{
			Operation: "commit",
			Messages:  []string{"test commit"},
			Chunks:    [][]string{{"test.go"}},
			Preview:   "Test",
		}

		result := commitPlanJSON(plan)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("commitPlanJSON returned invalid JSON: %v", err)
		}

		options, ok := parsed["options"].([]interface{})
		if !ok {
			t.Fatal("options field is not an array")
		}

		previewRaw, ok := parsed["structured_preview"].(map[string]interface{})
		if !ok {
			t.Fatal("structured_preview field is not a map")
		}

		actions, ok := previewRaw["actions"].([]interface{})
		if !ok {
			t.Fatal("structured_preview.actions is not an array")
		}

		// Ensure same length
		if len(options) != len(actions) {
			t.Errorf("options length (%d) != actions length (%d)", len(options), len(actions))
		}

		// Verify each option label matches action label
		for i, opt := range options {
			optLabel, ok := opt.(string)
			if !ok {
				t.Errorf("options[%d] is not a string: %v", i, opt)
				continue
			}

			action, ok := actions[i].(map[string]interface{})
			if !ok {
				t.Errorf("actions[%d] is not a map: %v", i, actions[i])
				continue
			}

			actionLabel, ok := action["label"].(string)
			if !ok {
				t.Errorf("actions[%d].label is not a string: %v", i, action["label"])
				continue
			}

			if optLabel != actionLabel {
				t.Errorf("options[%d] label %q != actions[%d].label %q", i, optLabel, i, actionLabel)
			}
		}
	})

	t.Run("release options mapping", func(t *testing.T) {
		intent := &domain.ReleaseIntent{
			TagName:     "v1.0.0",
			VersionBump: "major",
		}

		result := releasePlanJSON(intent, "", nil, "")
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("releasePlanJSON returned invalid JSON: %v", err)
		}

		options, ok := parsed["options"].([]interface{})
		if !ok {
			t.Fatal("options field is not an array")
		}

		previewRaw, ok := parsed["structured_preview"].(map[string]interface{})
		if !ok {
			t.Fatal("structured_preview field is not a map")
		}

		actions, ok := previewRaw["actions"].([]interface{})
		if !ok {
			t.Fatal("structured_preview.actions is not an array")
		}

		// Ensure same length
		if len(options) != len(actions) {
			t.Errorf("options length (%d) != actions length (%d)", len(options), len(actions))
		}

		// Release should have 2 options: Execute and Cancel
		if len(options) != 2 {
			t.Errorf("release should have 2 options, got %d", len(options))
		}
	})
}
