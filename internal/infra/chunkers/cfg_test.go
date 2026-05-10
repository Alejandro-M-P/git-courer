package chunkers

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

// --- walkCFG tests ---

func TestWalkCFG_GoSourceWithBranchLoopReturn(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
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

	count := walkCFG(lang, src, cf)
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
	lang := tsgrammars.PythonLanguage()
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

	count := walkCFG(lang, src, cf)
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
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{} // all empty slices

	src := []byte(`package main
func foo() {
	if x > 0 {
		return
	}
}`)

	count := walkCFG(lang, src, cf)
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

func TestWalkCFG_NilGrammar(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}
	src := []byte(`package main`)

	count := walkCFG(nil, src, cf)
	// Nil grammar → zero CFGCount (graceful degradation)
	if count != (domain.CFGCount{}) {
		t.Errorf("walkCFG with nil grammar = %+v, want zero CFGCount", count)
	}
}

func TestWalkCFG_UnnamedKeywordMatch(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
		Loop:   []string{"for"},
	}

	// This test verifies that 'if' (an unnamed keyword token) is matched
	// by walking ALL nodes, not just named children
	src := []byte(`package main
func foo() {
	if true {
		for i := 0; i < 5; i++ {}
	}
}`)

	count := walkCFG(lang, src, cf)
	if count.Branch < 1 {
		t.Errorf("Branch = %d, want >= 1 — unnamed 'if' keyword must be counted", count.Branch)
	}
	if count.Loop < 1 {
		t.Errorf("Loop = %d, want >= 1 — unnamed 'for' keyword must be counted", count.Loop)
	}
}

func TestWalkCFG_InvalidSource(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}

	// Completely invalid source that should still parse (tree-sitter is error-recovery)
	// but may produce error nodes
	src := []byte(`{{{invalid:::source`)

	count := walkCFG(lang, src, cf)
	// Should not panic, should return some count (even if zero)
	// The key guarantee: no panic, no error return
	_ = count
}

// --- ComputeCFGDiff tests ---

func TestComputeCFGDiff_HappyPath_AddedLoop(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
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

	diff := ComputeCFGDiff(lang, before, after, cf)
	if diff.Before.Return != 1 {
		t.Errorf("Before.Return = %d, want 1", diff.Before.Return)
	}
	if diff.After.Loop != 1 {
		t.Errorf("After.Loop = %d, want 1", diff.After.Loop)
	}
	if diff.After.Return != 1 {
		t.Errorf("After.Return = %d, want 1", diff.After.Return)
	}
	if diff.IsIdentical() {
		t.Error("diff should NOT be identical — loop was added")
	}
}

func TestComputeCFGDiff_NilGrammar(t *testing.T) {
	t.Parallel()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}
	before := []byte(`package main`)
	after := []byte(`package main`)

	diff := ComputeCFGDiff(nil, before, after, cf)
	if diff != (domain.CFGDiff{}) {
		t.Errorf("nil grammar should return zero CFGDiff, got %+v", diff)
	}
}

func TestComputeCFGDiff_EmptyControlFlow(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{} // all empty
	before := []byte(`package main
func foo() { return }`)
	after := []byte(`package main
func foo() { return }`)

	diff := ComputeCFGDiff(lang, before, after, cf)
	if diff != (domain.CFGDiff{}) {
		t.Errorf("empty ControlFlow should return zero CFGDiff, got %+v", diff)
	}
}

func TestComputeCFGDiff_ParseFailure(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
		Return: []string{"return"},
	}

	// Garbage source — tree-sitter will produce error nodes but shouldn't crash
	before := []byte(`{{{invalid`)
	after := []byte(`}more-invalid`)

	// Should not panic, should return some valid CFGCount (possibly zero)
	diff := ComputeCFGDiff(lang, before, after, cf)
	_ = diff // key guarantee: no panic
}

func TestComputeCFGDiff_NilBefore(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{
		Return: []string{"return"},
	}

	// New file: before is nil
	after := []byte(`package main
func foo() { return }`)

	diff := ComputeCFGDiff(lang, nil, after, cf)
	if diff.Before.Return != 0 {
		t.Errorf("Before.Return = %d, want 0 (nil before)", diff.Before.Return)
	}
	if diff.After.Return < 1 {
		t.Errorf("After.Return = %d, want >= 1 (new file with return)", diff.After.Return)
	}
}

func TestComputeCFGDiff_NilAfter(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{
		Return: []string{"return"},
	}

	// Deleted file: after is nil
	before := []byte(`package main
func foo() { return }`)

	diff := ComputeCFGDiff(lang, before, nil, cf)
	if diff.Before.Return < 1 {
		t.Errorf("Before.Return = %d, want >= 1 (deleted file had return)", diff.Before.Return)
	}
	if diff.After.Return != 0 {
		t.Errorf("After.Return = %d, want 0 (nil after)", diff.After.Return)
	}
}

func TestComputeCFGDiff_EmptyBeforeAndAfter(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
	cf := data.ControlFlowCategory{
		Branch: []string{"if"},
	}

	diff := ComputeCFGDiff(lang, nil, nil, cf)
	if diff != (domain.CFGDiff{}) {
		t.Errorf("nil before and after should return zero CFGDiff, got %+v", diff)
	}
}

func TestComputeCFGDiff_IdenticalFiles(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.GoLanguage()
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

	diff := ComputeCFGDiff(lang, src, src, cf)
	if !diff.IsIdentical() {
		t.Error("identical files should produce identical CFG counts")
	}
}

func TestComputeCFGDiff_JavaScriptSource(t *testing.T) {
	t.Parallel()
	lang := tsgrammars.JavascriptLanguage()
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

	diff := ComputeCFGDiff(lang, before, after, cf)
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