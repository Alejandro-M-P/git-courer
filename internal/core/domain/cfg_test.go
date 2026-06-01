package domain

import (
	"encoding/json"
	"testing"
)

func TestCFGCount_ZeroValue(t *testing.T) {
	t.Parallel()
	var c CFGCount
	if c.Branch != 0 || c.Loop != 0 || c.Return != 0 || c.Error != 0 {
		t.Errorf("zero-value CFGCount should have all fields 0, got %+v", c)
	}
}

func TestCFGCount_FieldAssignment(t *testing.T) {
	t.Parallel()
	c := CFGCount{Branch: 2, Loop: 1, Return: 3, Error: 0}
	if c.Branch != 2 {
		t.Errorf("Branch = %d, want 2", c.Branch)
	}
	if c.Loop != 1 {
		t.Errorf("Loop = %d, want 1", c.Loop)
	}
	if c.Return != 3 {
		t.Errorf("Return = %d, want 3", c.Return)
	}
	if c.Error != 0 {
		t.Errorf("Error = %d, want 0", c.Error)
	}
}

func TestDiffChunk_CFGBefore_NilByDefault(t *testing.T) {
	t.Parallel()
	var chunk DiffChunk
	if chunk.CFGBefore != nil {
		t.Error("CFGBefore should be nil by default (not computed)")
	}
	if chunk.CFGAfter != nil {
		t.Error("CFGAfter should be nil by default (not computed)")
	}
}

func TestDiffChunk_CFGBefore_PointerToZero(t *testing.T) {
	t.Parallel()
	// Computed but none found — pointer to zero struct, not nil
	zero := CFGCount{}
	chunk := DiffChunk{
		CFGBefore: &zero,
		CFGAfter:  &CFGCount{},
	}
	if chunk.CFGBefore == nil {
		t.Error("CFGBefore should not be nil when computed-zero")
	}
	if *chunk.CFGBefore != (CFGCount{}) {
		t.Errorf("CFGBefore = %+v, want zero CFGCount", *chunk.CFGBefore)
	}
	if chunk.CFGAfter == nil {
		t.Error("CFGAfter should not be nil when computed-zero")
	}
}

func TestDiffChunk_CFGBefore_ComputedValues(t *testing.T) {
	t.Parallel()
	before := CFGCount{Branch: 3, Loop: 1, Return: 2, Error: 0}
	after := CFGCount{Branch: 4, Loop: 1, Return: 2, Error: 1}
	chunk := DiffChunk{
		CFGBefore: &before,
		CFGAfter:  &after,
	}
	if chunk.CFGBefore.Branch != 3 {
		t.Errorf("CFGBefore.Branch = %d, want 3", chunk.CFGBefore.Branch)
	}
	if chunk.CFGAfter.Branch != 4 {
		t.Errorf("CFGAfter.Branch = %d, want 4", chunk.CFGAfter.Branch)
	}
	if chunk.CFGAfter.Error != 1 {
		t.Errorf("CFGAfter.Error = %d, want 1", chunk.CFGAfter.Error)
	}
}

func TestDiffChunk_CFGFields_JSONOmission(t *testing.T) {
	t.Parallel()
	// Nil pointers should be omitted from JSON with omitempty
	var chunk DiffChunk
	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("json.Marshal DiffChunk: %v", err)
	}
	s := string(data)
	if contains(s, "cfg_before") || contains(s, "cfg_after") {
		t.Errorf("nil CFG fields should be omitted from JSON, got: %s", s)
	}
}

func TestDiffChunk_CFGFields_JSONPresent(t *testing.T) {
	t.Parallel()
	// Non-nil pointer fields should appear in JSON with snake_case tags
	chunk := DiffChunk{
		CFGBefore: &CFGCount{Branch: 1},
		CFGAfter:  &CFGCount{Loop: 2},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("json.Marshal DiffChunk: %v", err)
	}
	s := string(data)
	if !contains(s, "cfg_before") {
		t.Errorf("non-nil CFGBefore should appear in JSON as cfg_before, got: %s", s)
	}
	if !contains(s, "cfg_after") {
		t.Errorf("non-nil CFGAfter should appear in JSON as cfg_after, got: %s", s)
	}
}

// contains checks if s contains substr (simple string search helper).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
