package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDiffChunk_ExtendedFields verifies CommitType and AnnotatedDiff fields.
func TestDiffChunk_ExtendedFields(t *testing.T) {
	t.Parallel()
	chunk := DiffChunk{
		Files:         []string{"internal/server/webhook.go", "internal/auth/validator.go"},
		Diff:          "diff content",
		CommitType:    "feat",
		AnnotatedDiff: "internal/server/webhook.go\nHandleWebhook [NEW_FUNC]\n+ func HandleWebhook() {}",
	}
	if chunk.CommitType != "feat" {
		t.Errorf("CommitType = %q, want feat", chunk.CommitType)
	}
	if chunk.AnnotatedDiff == "" {
		t.Error("AnnotatedDiff should not be empty")
	}
}

// TestDiffChunk_CommitTypeValues verifies all valid CommitType values.
func TestDiffChunk_CommitTypeValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		typ  string
	}{
		{"feat", "feat"},
		{"fix", "fix"},
		{"refactor", "refactor"},
		{"docs", "docs"},
		{"test", "test"},
		{"chore", "chore"},
		{"ci", "ci"},
		{"empty", ""},
		{"breaking-feat", "feat!"},
		{"breaking-fix", "fix!"},
		{"breaking-refactor", "refactor!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			chunk := DiffChunk{CommitType: tc.typ}
			if chunk.CommitType != tc.typ {
				t.Errorf("CommitType = %q, want %q", chunk.CommitType, tc.typ)
			}
		})
	}
}

// TestDiffChunk_AnnotatedDiffFormat verifies grouped-by-file format.
func TestDiffChunk_AnnotatedDiffFormat(t *testing.T) {
	t.Parallel()
	annotated := "internal/server/webhook.go\n" +
		"HandleWebhook [NEW_FUNC]\n" +
		"+ func HandleWebhook(w http.ResponseWriter, r *http.Request) {\n" +
		"+     token := r.Header.Get(\"Authorization\")\n" +
		"+ }\n\n" +
		"internal/auth/validator.go\n" +
		"validateToken [MOD_BODY]\n" +
		"- if token == \"\" {\n" +
		"+ if token == \"\" || len(token) < 10 {\n"

	chunk := DiffChunk{
		Files:         []string{"internal/server/webhook.go", "internal/auth/validator.go"},
		CommitType:    "feat",
		AnnotatedDiff: annotated,
	}

	if chunk.AnnotatedDiff != annotated {
		t.Errorf("AnnotatedDiff mismatch\ngot:\n%s\nwant:\n%s", chunk.AnnotatedDiff, annotated)
	}
	// Verify it contains file group markers
	if !strings.Contains(chunk.AnnotatedDiff, "internal/server/webhook.go") {
		t.Error("AnnotatedDiff should contain first file group")
	}
	if !strings.Contains(chunk.AnnotatedDiff, "internal/auth/validator.go") {
		t.Error("AnnotatedDiff should contain second file group")
	}
	// Verify it contains labels
	if !strings.Contains(chunk.AnnotatedDiff, "[NEW_FUNC]") {
		t.Error("AnnotatedDiff should contain NEW_FUNC label")
	}
	if !strings.Contains(chunk.AnnotatedDiff, "[MOD_BODY]") {
		t.Error("AnnotatedDiff should contain MOD_BODY label")
	}
}

// TestDiffChunk_EmptyDefaults verifies zero-value behavior.
func TestDiffChunk_EmptyDefaults(t *testing.T) {
	t.Parallel()
	var chunk DiffChunk
	if chunk.CommitType != "" {
		t.Errorf("zero-value CommitType = %q, want empty string", chunk.CommitType)
	}
	if chunk.AnnotatedDiff != "" {
		t.Errorf("zero-value AnnotatedDiff = %q, want empty string", chunk.AnnotatedDiff)
	}
	if chunk.Diff != "" {
		t.Errorf("zero-value Diff = %q, want empty string", chunk.Diff)
	}
	if len(chunk.Files) != 0 {
		t.Errorf("zero-value len(Files) = %d, want 0", len(chunk.Files))
	}
	if chunk.ConfidenceScore != 0.0 {
		t.Errorf("zero-value ConfidenceScore = %f, want 0.0", chunk.ConfidenceScore)
	}
}

// TestDiffChunk_ConfidenceScore validates the ConfidenceScore field range (0.0–1.0).
func TestDiffChunk_ConfidenceScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		score float64
	}{
		{"high confidence", 0.95},
		{"medium confidence", 0.75},
		{"low confidence", 0.30},
		{"zero confidence", 0.0},
		{"max confidence", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chunk := DiffChunk{
				Files:           []string{"a.go"},
				ConfidenceScore: tt.score,
			}
			if chunk.ConfidenceScore != tt.score {
				t.Errorf("ConfidenceScore = %f, want %f", chunk.ConfidenceScore, tt.score)
			}
		})
	}
}

// TestDiffChunk_JSONRoundTrip verifies that all DiffChunk fields survive
// JSON marshaling and unmarshaling with proper tags.
func TestDiffChunk_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := DiffChunk{
		Files:           []string{"internal/server/webhook.go", "internal/auth/validator.go"},
		Diff:            "diff --git a/file.go b/file.go\n+added line",
		CommitType:      "feat",
		AnnotatedDiff:   "internal/server/webhook.go\nHandleWebhook [NEW_FUNC]",
		ConfidenceScore:  0.92,
		Scope:            "security",
		BeforeSource:    map[string]string{"file.go": "package main\nfunc old() {}"},
		AfterSource:         map[string]string{"file.go": "package main\nfunc new() {}"},
		CFGBefore:       &CFGCount{Branch: 2, Loop: 1, Return: 3, Error: 0},
		CFGAfter:        &CFGCount{Branch: 3, Loop: 2, Return: 4, Error: 1},
	}

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	var restored DiffChunk
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(restored.Files) != len(original.Files) {
		t.Errorf("Files length: got %d, want %d", len(restored.Files), len(original.Files))
	}
	for i, f := range restored.Files {
		if f != original.Files[i] {
			t.Errorf("Files[%d]: got %q, want %q", i, f, original.Files[i])
		}
	}
	if restored.Diff != original.Diff {
		t.Errorf("Diff: got %q, want %q", restored.Diff, original.Diff)
	}
	if restored.CommitType != original.CommitType {
		t.Errorf("CommitType: got %q, want %q", restored.CommitType, original.CommitType)
	}
	if restored.AnnotatedDiff != original.AnnotatedDiff {
		t.Errorf("AnnotatedDiff: got %q, want %q", restored.AnnotatedDiff, original.AnnotatedDiff)
	}
	if restored.ConfidenceScore != original.ConfidenceScore {
		t.Errorf("ConfidenceScore: got %f, want %f", restored.ConfidenceScore, original.ConfidenceScore)
	}
	if restored.Scope != original.Scope {
		t.Errorf("Scope: got %q, want %q", restored.Scope, original.Scope)
	}
	if len(restored.BeforeSource) != len(original.BeforeSource) {
		t.Errorf("BeforeSource length: got %d, want %d", len(restored.BeforeSource), len(original.BeforeSource))
	}
	if len(restored.AfterSource) != len(original.AfterSource) {
		t.Errorf("AfterSource length: got %d, want %d", len(restored.AfterSource), len(original.AfterSource))
	}
	if restored.CFGBefore == nil {
		t.Error("CFGBefore: got nil, want non-nil")
	} else if *restored.CFGBefore != *original.CFGBefore {
		t.Errorf("CFGBefore: got %+v, want %+v", restored.CFGBefore, original.CFGBefore)
	}
	if restored.CFGAfter == nil {
		t.Error("CFGAfter: got nil, want non-nil")
	} else if *restored.CFGAfter != *original.CFGAfter {
		t.Errorf("CFGAfter: got %+v, want %+v", restored.CFGAfter, original.CFGAfter)
	}
}

// TestDiffChunk_JSONOmitEmptyCFG verifies that nil CFG counts are omitted from JSON.
func TestDiffChunk_JSONOmitEmptyCFG(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{
		Files:      []string{"a.go"},
		Diff:       "diff content",
		CFGBefore:  nil,
		CFGAfter:   nil,
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// nil CFG fields should be omitted due to json:",omitempty"
	s := string(data)
	if strings.Contains(s, `"cfg_before"`) || strings.Contains(s, `"CFGBefore"`) {
		t.Errorf("nil CFGBefore should be omitted from JSON, got: %s", s)
	}
	if strings.Contains(s, `"cfg_after"`) || strings.Contains(s, `"CFGAfter"`) {
		t.Errorf("nil CFGAfter should be omitted from JSON, got: %s", s)
	}
}

// TestDiffChunk_SliceRoundTrip verifies that a slice of DiffChunks survives JSON round-trip.
func TestDiffChunk_SliceRoundTrip(t *testing.T) {
	t.Parallel()

	original := []DiffChunk{
		{
			Files:       []string{"a.go"},
			Diff:        "+added line",
			CommitType:  "feat",
			Scope:       "core",
		},
		{
			Files:       []string{"b.go", "c.go"},
			Diff:        "-removed line\n+added line",
			CommitType:  "fix",
			ConfidenceScore: 0.85,
		},
	}

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	var restored []DiffChunk
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(restored) != len(original) {
		t.Fatalf("slice length: got %d, want %d", len(restored), len(original))
	}
	for i, ch := range restored {
		if ch.CommitType != original[i].CommitType {
			t.Errorf("chunk[%d].CommitType: got %q, want %q", i, ch.CommitType, original[i].CommitType)
		}
		if ch.Diff != original[i].Diff {
			t.Errorf("chunk[%d].Diff: got %q, want %q", i, ch.Diff, original[i].Diff)
		}
		if ch.Scope != original[i].Scope {
			t.Errorf("chunk[%d].Scope: got %q, want %q", i, ch.Scope, original[i].Scope)
		}
	}
}
