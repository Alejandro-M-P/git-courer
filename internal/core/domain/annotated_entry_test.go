package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAnnotatedEntry_JSONRoundTrip verifies the new AnnotatedEntry type
// marshals and unmarshals with the expected JSON keys.
func TestAnnotatedEntry_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := AnnotatedEntry{
		File:     "exec_write_commit.go",
		Symbol:   "CommitTree",
		Type:     "MOD_SIG",
		Breaking: true,
		Line:     42,
		Calls:    []string{"exec_write_commit.go/runGit", "git.go/Status"},
		Before:   "- return nil",
		After:    "+ return fmt.Errorf(\"updated\")",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	s := string(data)
	for _, want := range []string{`"file"`, `"symbol"`, `"type"`, `"breaking"`, `"line"`, `"calls"`, `"before"`, `"after"`} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing key %s; got: %s", want, s)
		}
	}

	var restored AnnotatedEntry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored.File != original.File || restored.Symbol != original.Symbol ||
		restored.Type != original.Type || restored.Breaking != original.Breaking ||
		restored.Line != original.Line || restored.Before != original.Before ||
		restored.After != original.After || len(restored.Calls) != len(original.Calls) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", restored, original)
	}
	for i, c := range original.Calls {
		if restored.Calls[i] != c {
			t.Errorf("Calls[%d]: got %q, want %q", i, restored.Calls[i], c)
		}
	}
}

// TestAnnotatedEntry_OmitEmptyCalls verifies calls field is omitted when empty.
func TestAnnotatedEntry_OmitEmptyCalls(t *testing.T) {
	t.Parallel()

	entry := AnnotatedEntry{
		File:   "a.go",
		Symbol: "F",
		Type:   "NEW_FUNC",
		Line:   1,
		After:  "+ func F() {}",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if strings.Contains(string(data), `"calls"`) {
		t.Errorf("empty calls should be omitted; got: %s", data)
	}
}

// TestCallGraphEntry_JSON verifies CallGraphEntry JSON keys.
func TestCallGraphEntry_JSON(t *testing.T) {
	t.Parallel()

	original := CallGraphEntry{
		From:   "exec_write_commit.go",
		To:     "git.go",
		Symbol: "Status",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	for _, want := range []string{`"from"`, `"to"`, `"symbol"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON missing key %s; got: %s", want, data)
		}
	}

	var restored CallGraphEntry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored != original {
		t.Errorf("round-trip mismatch: got %+v, want %+v", restored, original)
	}
}

// TestCFGSummary_JSON verifies CFGSummary nests CFGEntry per category.
func TestCFGSummary_JSON(t *testing.T) {
	t.Parallel()

	original := CFGSummary{
		Conditionals: CFGEntry{Before: 0, After: 1},
		Loops:        CFGEntry{Before: 2, After: 2},
		Returns:      CFGEntry{Before: 1, After: 3},
		Errors:       CFGEntry{Before: 0, After: 0},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	for _, want := range []string{`"conditionals"`, `"loops"`, `"returns"`, `"errors"`, `"before"`, `"after"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON missing key %s; got: %s", want, data)
		}
	}

	var restored CFGSummary
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored != original {
		t.Errorf("round-trip mismatch: got %+v, want %+v", restored, original)
	}
}

// TestDiffChunk_AnnotatedEntriesAndCallGraph verifies the new typed fields on
// DiffChunk survive JSON round-trip and are additive (AnnotatedDiff still works).
func TestDiffChunk_AnnotatedEntriesAndCallGraph(t *testing.T) {
	t.Parallel()

	original := DiffChunk{
		Files:         []string{"exec_write_commit.go"},
		Diff:          "raw diff",
		AnnotatedDiff: "legacy emoji string",
		AnnotatedEntries: []AnnotatedEntry{
			{File: "exec_write_commit.go", Symbol: "CommitTree", Type: "MOD_SIG", Line: 10, Before: "- a", After: "+ b"},
			{File: "exec_write_commit.go", Symbol: "Helper", Type: "NEW_FUNC", Line: 20, After: "+ func Helper() {}"},
		},
		CallGraph: []CallGraphEntry{
			{From: "exec_write_commit.go", To: "git.go", Symbol: "Status"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"annotated_entries"`) {
		t.Errorf("JSON missing annotated_entries key; got: %s", s)
	}
	if !strings.Contains(s, `"call_graph"`) {
		t.Errorf("JSON missing call_graph key; got: %s", s)
	}
	if !strings.Contains(s, `"annotated_diff":"legacy emoji string"`) {
		t.Errorf("AnnotatedDiff must be preserved for backward compat; got: %s", s)
	}

	var restored DiffChunk
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored.AnnotatedDiff != original.AnnotatedDiff {
		t.Errorf("AnnotatedDiff: got %q, want %q", restored.AnnotatedDiff, original.AnnotatedDiff)
	}
	if len(restored.AnnotatedEntries) != 2 {
		t.Fatalf("AnnotatedEntries: got %d, want 2", len(restored.AnnotatedEntries))
	}
	if restored.AnnotatedEntries[0].Symbol != "CommitTree" {
		t.Errorf("AnnotatedEntries[0].Symbol: got %q, want CommitTree", restored.AnnotatedEntries[0].Symbol)
	}
	if len(restored.CallGraph) != 1 || restored.CallGraph[0].Symbol != "Status" {
		t.Errorf("CallGraph mismatch: got %+v, want 1 entry with Status", restored.CallGraph)
	}
}

// TestDiffChunk_OmitEmptyAnnotatedEntries verifies typed slice fields are
// omitted from JSON when empty (zero-value DiffChunk).
func TestDiffChunk_OmitEmptyAnnotatedEntries(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{Files: []string{"a.go"}, Diff: "x"}
	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	s := string(data)
	if strings.Contains(s, `"annotated_entries"`) {
		t.Errorf("empty AnnotatedEntries should be omitted; got: %s", s)
	}
	if strings.Contains(s, `"call_graph"`) {
		t.Errorf("empty CallGraph should be omitted; got: %s", s)
	}
}