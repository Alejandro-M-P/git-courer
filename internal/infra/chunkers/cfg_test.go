package chunkers

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
)

// --- walkCFG tests ---

func TestWalkCFG_GoSourceWithBranchLoopReturn(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if", "else", "switch", "case"},
		Loop:   []string{"for"},
		Return: []string{"return"},
		Error:  nil,
	}

	// Go source: 2 if keywords, 1 for keyword, 1 return keyword
	src := []byte(`package main
func foo() {
	if x > 0 {
		for i := 0; i < 10; i++ {
			if y < 5 {
				return
			}
		}
	}
}`)

	count := walkCFG("Go", src, cf)
	if count.Branch != 2 {
		t.Errorf("Branch = %d, want 2 (two 'if' keywords)", count.Branch)
	}
	if count.Loop != 1 {
		t.Errorf("Loop = %d, want 1 (one 'for' keyword)", count.Loop)
	}
	if count.Return != 1 {
		t.Errorf("Return = %d, want 1 (one 'return' keyword)", count.Return)
	}
	if count.Error != 0 {
		t.Errorf("Error = %d, want 0 (no error keywords)", count.Error)
	}
}

func TestWalkCFG_PythonSourceWithAllCategories(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if", "elif", "else"},
		Loop:   []string{"for", "while"},
		Return: []string{"return"},
		Error:  []string{"try", "except"},
	}

	// Python source: 1 if, 1 for, 1 return, 1 try, 1 except
	src := []byte(`
def foo():
    try:
        for x in range(10):
            if x > 5:
                return x
    except ValueError:
        pass
`)

	count := walkCFG("Python", src, cf)
	if count.Branch < 1 {
		t.Errorf("Branch = %d, want >= 1 (at least one 'if')", count.Branch)
	}
	if count.Loop < 1 {
		t.Errorf("Loop = %d, want >= 1 (at least one 'for')", count.Loop)
	}
	if count.Return < 1 {
		t.Errorf("Return = %d, want >= 1 (at least one 'return')", count.Return)
	}
	if count.Error < 1 {
		t.Errorf("Error = %d, want >= 1 (at least one 'try' or 'except')", count.Error)
	}
}

func TestWalkCFG_EmptyControlFlow(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{} // all empty slices

	src := []byte(`package main
func foo() {
	if x > 0 {
		return
	}
}`)

	count := walkCFG("Go", src, cf)
	// All counts should be zero because ControlFlowCategory is empty
	if count.Branch != 0 {
		t.Errorf("Branch = %d, want 0 (empty control flow)", count.Branch)
	}
	if count.Loop != 0 {
		t.Errorf("Loop = %d, want 0 (empty control flow)", count.Loop)
	}
	if count.Return != 0 {
		t.Errorf("Return = %d, want 0 (empty control flow)", count.Return)
	}
	if count.Error != 0 {
		t.Errorf("Error = %d, want 0 (empty control flow)", count.Error)
	}
}

func TestWalkCFG_EmptyLangName(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}
	src := []byte(`package main`)

	count := walkCFG("", src, cf)
	// Empty langName → zero CFGCount (graceful degradation)
	if count != (domain.CFGCount{}) {
		t.Errorf("walkCFG with empty langName = %+v, want zero CFGCount", count)
	}
}

func TestWalkCFG_KeywordWordMatch(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
		Loop:   []string{"for"},
	}

	// This test verifies that 'if' and 'for' keywords are matched
	// by word tokenization of the source
	src := []byte(`package main
func foo() {
	if true {
		for i := 0; i < 5; i++ {}
	}
}`)

	count := walkCFG("Go", src, cf)
	if count.Branch < 1 {
		t.Errorf("Branch = %d, want >= 1 — 'if' keyword must be counted", count.Branch)
	}
	if count.Loop < 1 {
		t.Errorf("Loop = %d, want >= 1 — 'for' keyword must be counted", count.Loop)
	}
}

func TestWalkCFG_InvalidSource(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}

	src := []byte(`{{{invalid:::source`)

	count := walkCFG("Go", src, cf)
	// Should not panic, should return some count (even if zero)
	_ = count
}

// --- ComputeCFGDiff tests ---

func TestComputeCFGDiff_HappyPath_AddedLoop(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if", "else", "switch", "case"},
		Loop:   []string{"for"},
		Return: []string{"return"},
	}

	before := []byte(`package main
func foo() {
	return
}`)

	after := []byte(`package main
func foo() {
	for i := 0; i < 5; i++ {
		return
	}
}`)

	diff := ComputeCFGDiff("Go", before, after, cf)
	if diff.Before.Return != 1 {
		t.Errorf("Before.Return = %d, want 1", diff.Before.Return)
	}
	if diff.After.Loop != 1 {
		t.Errorf("After.Loop = %d, want 1", diff.After.Loop)
	}
	if diff.After.Return != 1 {
		t.Errorf("After.Return = %d, want 1", diff.After.Return)
	}
	if diff.Before == diff.After {
		t.Error("diff should NOT be identical — loop was added")
	}
}

func TestComputeCFGDiff_EmptyLangName(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}
	before := []byte(`package main`)
	after := []byte(`package main`)

	diff := ComputeCFGDiff("", before, after, cf)
	if diff != (domain.CFGDiff{}) {
		t.Errorf("empty langName should return zero CFGDiff, got %+v", diff)
	}
}

func TestComputeCFGDiff_EmptyControlFlow(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{} // all empty
	before := []byte(`package main
func foo() { return }`)
	after := []byte(`package main
func foo() { return }`)

	diff := ComputeCFGDiff("Go", before, after, cf)
	if diff != (domain.CFGDiff{}) {
		t.Errorf("empty ControlFlow should return zero CFGDiff, got %+v", diff)
	}
}

func TestComputeCFGDiff_GarbageSource(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
		Return: []string{"return"},
	}

	// Garbage source — should not panic
	before := []byte(`{{{invalid`)
	after := []byte(`}more-invalid`)

	diff := ComputeCFGDiff("Go", before, after, cf)
	_ = diff // key guarantee: no panic
}

func TestComputeCFGDiff_NilBefore(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Return: []string{"return"},
	}

	// New file: before is nil
	after := []byte(`package main
func foo() { return }`)

	diff := ComputeCFGDiff("Go", nil, after, cf)
	if diff.Before.Return != 0 {
		t.Errorf("Before.Return = %d, want 0 (nil before)", diff.Before.Return)
	}
	if diff.After.Return < 1 {
		t.Errorf("After.Return = %d, want >= 1 (new file with return)", diff.After.Return)
	}
}

func TestComputeCFGDiff_NilAfter(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Return: []string{"return"},
	}

	// Deleted file: after is nil
	before := []byte(`package main
func foo() { return }`)

	diff := ComputeCFGDiff("Go", before, nil, cf)
	if diff.Before.Return < 1 {
		t.Errorf("Before.Return = %d, want >= 1 (deleted file had return)", diff.Before.Return)
	}
	if diff.After.Return != 0 {
		t.Errorf("After.Return = %d, want 0 (nil after)", diff.After.Return)
	}
}

func TestComputeCFGDiff_EmptyBeforeAndAfter(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}

	diff := ComputeCFGDiff("Go", nil, nil, cf)
	if diff != (domain.CFGDiff{}) {
		t.Errorf("nil before and after should return zero CFGDiff, got %+v", diff)
	}
}

func TestComputeCFGDiff_IdenticalFiles(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
		Return: []string{"return"},
	}

	src := []byte(`package main
func foo() {
	if true {
		return
	}
}`)

	diff := ComputeCFGDiff("Go", src, src, cf)
	if diff.Before != diff.After {
		t.Error("identical files should produce identical CFG counts")
	}
}

// --- Integration tests: ProcessWithContent and Annotate with CFG ---

func TestProcessWithContent_GoFile_SetsCFGDiff(t *testing.T) {
	t.Parallel()
	pass := NewUnifiedASTPass(NewLanguageCatalog())

	before := []byte(`package main
func foo() {
}`)

	after := []byte(`package main
func foo() {
	if x > 0 {
		return
	}
}`)

	labels, cfgDiff, err := pass.ProcessWithContent("test.go", before, after, nil)
	if err != nil {
		t.Fatalf("ProcessWithContent error: %v", err)
	}
	if len(labels) == 0 {
		t.Error("expected labels, got none")
	}
	// Go's ControlFlowCategory has Branch keywords (if, else, switch, case).
	// After has an 'if' keyword that before doesn't.
	if cfgDiff.After.Branch <= cfgDiff.Before.Branch {
		t.Errorf("cfgDiff.After.Branch=%d should be > cfgDiff.Before.Branch=%d (added an 'if')",
			cfgDiff.After.Branch, cfgDiff.Before.Branch)
	}
}

func TestProcessWithContent_NonCodeFile_NilCFGDiff(t *testing.T) {
	t.Parallel()
	pass := NewUnifiedASTPass(NewLanguageCatalog())

	before := []byte(`{"key": "old"}`)
	after := []byte(`{"key": "new"}`)

	labels, cfgDiff, err := pass.ProcessWithContent("config.json", before, after, nil)
	if err != nil {
		t.Fatalf("ProcessWithContent error: %v", err)
	}
	// Non-code files should return a zero cfgDiff (not computed)
	if cfgDiff != (domain.CFGDiff{}) {
		t.Errorf("non-code file cfgDiff = %+v, want zero CFGDiff", cfgDiff)
	}
	// Should still get a label
	found := false
	for _, l := range labels {
		if l.Type == "CONFIG" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CONFIG label, got labels: %+v", labels)
	}
}

func TestAnnotate_SetsCFGBeforeAndAfter(t *testing.T) {
	t.Parallel()
	pass := NewUnifiedASTPass(NewLanguageCatalog())

	before := []byte(`package main
func foo() {
}`)

	after := []byte(`package main
func foo() {
	if x > 0 {
		return
	}
}`)

	chunk := &domain.DiffChunk{
		Files: []string{"test.go"},
		Diff:  "fake diff",
	}

	err := pass.Annotate(chunk, "test.go", before, after)
	if err != nil {
		t.Fatalf("Annotate error: %v", err)
	}
	if chunk.CFGBefore == nil {
		t.Error("CFGBefore should not be nil after Annotate for .go file")
	}
	if chunk.CFGAfter == nil {
		t.Error("CFGAfter should not be nil after Annotate for .go file")
	}
	// After has an 'if' keyword that before doesn't
	if chunk.CFGAfter.Branch <= chunk.CFGBefore.Branch {
		t.Errorf("CFGAfter.Branch=%d should be > CFGBefore.Branch=%d",
			chunk.CFGAfter.Branch, chunk.CFGBefore.Branch)
	}
}

func TestAnnotate_NonCodeFile_NilCFG(t *testing.T) {
	t.Parallel()
	pass := NewUnifiedASTPass(NewLanguageCatalog())

	before := []byte(`old`)
	after := []byte(`new`)

	chunk := &domain.DiffChunk{
		Files: []string{"config.json"},
		Diff:  "fake diff",
	}

	err := pass.Annotate(chunk, "config.json", before, after)
	if err != nil {
		t.Fatalf("Annotate error: %v", err)
	}
	if chunk.CFGBefore != nil {
		t.Errorf("CFGBefore should be nil for .json file, got %+v", chunk.CFGBefore)
	}
	if chunk.CFGAfter != nil {
		t.Errorf("CFGAfter should be nil for .json file, got %+v", chunk.CFGAfter)
	}
}

// --- ComputeEntityCFGDiff tests ---

// TestComputeEntityCFGDiff_IsolatedBody verifies that ComputeEntityCFGDiff
// computes CFG for only the entity's body byte span, not the whole file.
func TestComputeEntityCFGDiff_IsolatedBody(t *testing.T) {
	cf := data.ControlFlowCategory{
		Branch: []string{"if", "else", "switch", "case"},
		Loop:   []string{"for"},
		Return: []string{"return"},
		Error:  []string{"try", "catch"},
	}

	t.Run("valid_body_span_yields_entity_CFG", func(t *testing.T) {
		// Same byte range in both before and after
		beforeBody := []byte("  return           ") // 20 bytes: only 'return'
		afterBody := []byte("  if { return }   ") // 20 bytes: 'if' + 'return'

		got := ComputeEntityCFGDiff("Go", beforeBody, afterBody, 0, 20, 0, 20, cf)

		if got.Before.Return != 1 {
			t.Errorf("Before.Return = %d, want 1", got.Before.Return)
		}
		if got.Before.Branch != 0 {
			t.Errorf("Before.Branch = %d, want 0", got.Before.Branch)
		}

		if got.After.Branch != 1 {
			t.Errorf("After.Branch = %d, want 1 (contains 'if')", got.After.Branch)
		}
		if got.After.Return != 1 {
			t.Errorf("After.Return = %d, want 1 (contains 'return')", got.After.Return)
		}
	})

	t.Run("unchanged_entity_in_changed_file_gets_zero_diff", func(t *testing.T) {
		// Same body in before and after → identical CFG counts
		body := []byte("  return\n")

		got := ComputeEntityCFGDiff("Go", body, body, 0, len(body), 0, len(body), cf)

		if got.Before != got.After {
			t.Errorf("unchanged entity: Before=%+v should equal After=%+v", got.Before, got.After)
		}
	})

	t.Run("different_offsets_same_entity_in_before_after", func(t *testing.T) {
		// When prior entities grew, the same entity shifts offset in after.
		// We construct before/after with the same entity body at DIFFERENT byte positions.
		// "return" must be a space-separated word for walkCFG's strings.Fields tokenizer.
		fullBefore := []byte("AAAAAAAAAA return       BBBBBBBB")
		beforeBodyStart := 10
		beforeBodyEnd := 24 // " return       "

		fullAfter := []byte("AAAAAAAAAA extra-stuff return       BBBBBBBB")
		afterBodyStart := 22
		afterBodyEnd := 36 // " return       "

		got := ComputeEntityCFGDiff("Go", fullBefore, fullAfter, beforeBodyStart, beforeBodyEnd, afterBodyStart, afterBodyEnd, cf)

		// Both sub-slices contain "return" → 1 "return" keyword each
		if got.Before.Return != 1 {
			t.Errorf("Before.Return = %d, want 1 (entity 'return' in before slice)", got.Before.Return)
		}
		if got.After.Return != 1 {
			t.Errorf("After.Return = %d, want 1 (entity 'return' in after slice)", got.After.Return)
		}
		// Since both are identical → zero diff
		if got.Before != got.After {
			t.Errorf("identical body at different offsets: Before=%+v should equal After=%+v", got.Before, got.After)
		}
	})

	t.Run("invalid_negative_beforeBodyStart_falls_back", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)

		beforeSrc := []byte("package main\nfunc foo() {\n  return\n}\n")
		afterSrc := []byte("package main\nfunc foo() {\n  if x > 0 {\n    return\n  }\n}\n")

		got := ComputeEntityCFGDiff("Go", beforeSrc, afterSrc, -1, 10, 0, 10, cf)
		want := ComputeCFGDiff("Go", beforeSrc, afterSrc, cf)

		if got.Before != want.Before || got.After != want.After {
			t.Errorf("invalid span fallback: got Before=%+v After=%+v, want Before=%+v After=%+v",
				got.Before, got.After, want.Before, want.After)
		}
		if logBuf.Len() == 0 {
			t.Error("expected slog.Debug for invalid byte span, but no log output found")
		}
	})

	t.Run("afterBodyEnd_exceeds_srclen_falls_back", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)

		beforeSrc := []byte("package main\nfunc foo() {\n  return\n}\n")
		afterSrc := []byte("package main\nfunc foo() {\n  if x > 0 {\n    return\n  }\n}\n")

		got := ComputeEntityCFGDiff("Go", beforeSrc, afterSrc, 0, 10, 0, 999, cf)
		want := ComputeCFGDiff("Go", beforeSrc, afterSrc, cf)

		if got.Before != want.Before || got.After != want.After {
			t.Errorf("out-of-bounds span fallback: got Before=%+v After=%+v, want Before=%+v After=%+v",
				got.Before, got.After, want.Before, want.After)
		}
		if logBuf.Len() == 0 {
			t.Error("expected slog.Debug for out-of-bounds byte span, but no log output found")
		}
	})

	t.Run("bodystart_exceeds_srclen_falls_back", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)

		beforeSrc := []byte("package main\nfunc foo() {\n  return\n}\n")
		afterSrc := []byte("package main\nfunc foo() {\n  if x > 0 {\n    return\n  }\n}\n")

		got := ComputeEntityCFGDiff("Go", beforeSrc, afterSrc, 999, 1000, 999, 1000, cf)
		want := ComputeCFGDiff("Go", beforeSrc, afterSrc, cf)

		if got.Before != want.Before || got.After != want.After {
			t.Errorf("bodystart > srclen fallback: got Before=%+v After=%+v, want Before=%+v After=%+v",
				got.Before, got.After, want.Before, want.After)
		}
		if logBuf.Len() == 0 {
			t.Error("expected slog.Debug for bodyStart > srclen, but no log output found")
		}
	})

	t.Run("nonzero_offset_sub_slice", func(t *testing.T) {
		// Verify that a non-zero-starting body span correctly extracts
		// the sub-slice from the middle of the source.
		prefix := []byte("package main\nfunc foo() {")
		beforeBody := []byte("\n  return\n")
		suffix := []byte("}\nfunc bar() {\n  return\n}\n")

		beforeFull := append(append(prefix, beforeBody...), suffix...)

		// After: foo's body now has an "if" branch
		afterBody := []byte("\n  if x > 0 {\n    return\n  }\n")
		afterSuffix := []byte("}\nfunc bar() {\n  return\n}\n")

		afterFull := append(append(prefix, afterBody...), afterSuffix...)

		beforeBodyStart := len(prefix)
		beforeBodyEnd := beforeBodyStart + len(beforeBody)
		afterBodyStart := len(prefix)
		afterBodyEnd := afterBodyStart + len(afterBody)

		got := ComputeEntityCFGDiff("Go", beforeFull, afterFull, beforeBodyStart, beforeBodyEnd, afterBodyStart, afterBodyEnd, cf)

		if got.After.Branch < 1 {
			t.Errorf("After.Branch = %d, want >= 1 (after body contains 'if')", got.After.Branch)
		}
	})
}

func TestComputeCFGDiff_JavaScriptSource(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if", "else", "switch", "case"},
		Loop:   []string{"for", "while"},
		Return: []string{"return"},
		Error:  []string{"try", "catch"},
	}

	before := []byte(`function foo() { return 1; }`)
	after := []byte(`function foo() {
	if (x > 0) {
		for (let i = 0; i < 10; i++) {
			try {
				return i;
			} catch(e) {
				return -1;
			}
		}
	}
	return 0;
}`)

	diff := ComputeCFGDiff("JavaScript", before, after, cf)
	if diff.Before.Branch != 0 {
		t.Errorf("Before.Branch = %d, want 0", diff.Before.Branch)
	}
	if diff.After.Branch < 1 {
		t.Errorf("After.Branch = %d, want >= 1 (has 'if')", diff.After.Branch)
	}
	if diff.After.Loop < 1 {
		t.Errorf("After.Loop = %d, want >= 1 (has 'for')", diff.After.Loop)
	}
	if diff.After.Error < 1 {
		t.Errorf("After.Error = %d, want >= 1 (has 'try' or 'catch')", diff.After.Error)
	}
}