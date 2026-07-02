package chunkers

import (
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// TestBuildAnnotatedEntries_ModifiedSymbol verifies a MOD_SIG label produces
// one entry with hunk-only before/after and the correct type.
func TestBuildAnnotatedEntries_ModifiedSymbol(t *testing.T) {
	diff := "diff --git a/service.go b/service.go\n" +
		"--- a/service.go\n" +
		"+++ b/service.go\n" +
		"@@ -1,5 +1,5 @@\n" +
		" package main\n" +
		" \n" +
		" func Process(x int) error {\n" +
		"-	return nil\n" +
		"+	return fmt.Errorf(\"updated\")\n" +
		" }\n"

	labels := []domain.Label{
		{Name: "Process", Type: domain.MOD_SIG, File: "service.go", Line: 3},
	}
	hunks := parseDiffHunks(diff)

	entries := buildAnnotatedEntries(labels, hunks)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.File != "service.go" {
		t.Errorf("File: got %q, want service.go", e.File)
	}
	if e.Symbol != "Process" {
		t.Errorf("Symbol: got %q, want Process", e.Symbol)
	}
	if e.Type != "MOD_SIG" {
		t.Errorf("Type: got %q, want MOD_SIG", e.Type)
	}
	if !strings.Contains(e.Before, "-\treturn nil") {
		t.Errorf("Before should contain deleted hunk line; got %q", e.Before)
	}
	if !strings.Contains(e.After, "+\treturn fmt.Errorf") {
		t.Errorf("After should contain added hunk line; got %q", e.After)
	}
	if e.Line != 3 {
		t.Errorf("Line: got %d, want 3", e.Line)
	}
}

// TestBuildAnnotatedEntries_NewAndDeletedSymbols verifies NEW_FUNC leaves
// before empty and DELETED_FUNC leaves after empty.
func TestBuildAnnotatedEntries_NewAndDeletedSymbols(t *testing.T) {
	diff := "diff --git a/svc.go b/svc.go\n" +
		"--- a/svc.go\n" +
		"+++ b/svc.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		" \n" +
		"-func OldHelper() {}\n" +
		"+func NewFeature() {}\n" +
		"+func Another() {}\n"

	labels := []domain.Label{
		{Name: "OldHelper", Type: domain.DELETED_FUNC, File: "svc.go", Line: 3},
		{Name: "NewFeature", Type: domain.NEW_FUNC, File: "svc.go", Line: 4},
	}
	hunks := parseDiffHunks(diff)

	entries := buildAnnotatedEntries(labels, hunks)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	bySymbol := map[string]domain.AnnotatedEntry{}
	for _, e := range entries {
		bySymbol[e.Symbol] = e
	}

	oldE, ok := bySymbol["OldHelper"]
	if !ok {
		t.Fatalf("missing OldHelper entry")
	}
	if oldE.Before == "" {
		t.Errorf("OldHelper (DELETED_FUNC) Before should be populated; got empty")
	}
	if oldE.After != "" {
		t.Errorf("OldHelper (DELETED_FUNC) After should be empty; got %q", oldE.After)
	}

	newE, ok := bySymbol["NewFeature"]
	if !ok {
		t.Fatalf("missing NewFeature entry")
	}
	if newE.Before != "" {
		t.Errorf("NewFeature (NEW_FUNC) Before should be empty; got %q", newE.Before)
	}
	if newE.After == "" {
		t.Errorf("NewFeature (NEW_FUNC) After should be populated; got empty")
	}
}

// TestBuildAnnotatedEntries_BreakingFlag verifies the breaking flag is
// propagated to the entry.
func TestBuildAnnotatedEntries_BreakingFlag(t *testing.T) {
	diff := "diff --git a/api.go b/api.go\n" +
		"--- a/api.go\n" +
		"+++ b/api.go\n" +
		"@@ -1,4 +1,4 @@\n" +
		" package main\n" +
		" \n" +
		" func Add(a, b int) int {\n" +
		"-	return a + b\n" +
		"+	return a + b + c\n" +
		" }\n"

	labels := []domain.Label{
		{Name: "Add", Type: domain.MOD_SIG, File: "api.go", Line: 3, Breaking: true},
	}
	hunks := parseDiffHunks(diff)

	entries := buildAnnotatedEntries(labels, hunks)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !entries[0].Breaking {
		t.Errorf("Breaking: got false, want true")
	}
}

// TestBuildAnnotatedEntries_EmptyLabelsReturnsNil verifies no labels produces
// no entries.
func TestBuildAnnotatedEntries_EmptyLabelsReturnsNil(t *testing.T) {
	diff := "diff --git a/x.go b/x.go\n+++ b/x.go\n@@ -1,1 +1,1 @@\n-a\n+b\n"
	hunks := parseDiffHunks(diff)
	entries := buildAnnotatedEntries(nil, hunks)
	if len(entries) != 0 {
		t.Errorf("empty labels: got %d entries, want 0", len(entries))
	}
}

// TestBuildAnnotatedEntries_NoHunksForFile verifies a label for a file with no
// hunks produces an entry with empty before/after (still listed).
func TestBuildAnnotatedEntries_NoHunksForFile(t *testing.T) {
	labels := []domain.Label{
		{Name: "F", Type: domain.MOD_BODY, File: "nonexistent.go", Line: 5},
	}
	entries := buildAnnotatedEntries(labels, nil)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Before != "" || entries[0].After != "" {
		t.Errorf("no hunks: Before/After should be empty; got Before=%q After=%q", entries[0].Before, entries[0].After)
	}
}

// TestParseDiffHunks_ExtractsHunks verifies the moved parseDiffHunks still
// parses a unified diff correctly.
func TestParseDiffHunks_ExtractsHunks(t *testing.T) {
	diff := "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n@@ -1,2 +1,2 @@\n-a\n+b\n c\n"
	hunks := parseDiffHunks(diff)
	if len(hunks) != 1 {
		t.Fatalf("got %d files, want 1", len(hunks))
	}
	h, ok := hunks["f.go"]
	if !ok {
		t.Fatalf("missing f.go in hunks: %v", hunks)
	}
	if len(h) != 1 {
		t.Fatalf("got %d hunks for f.go, want 1", len(h))
	}
	if len(h[0].lines) != 3 {
		t.Errorf("hunk lines: got %d, want 3", len(h[0].lines))
	}
}

// TestHunkLinesForLabel_ModSig verifies the extracted hunkLinesForLabel returns
// the changed lines for a MOD_SIG label.
func TestHunkLinesForLabel_ModSig(t *testing.T) {
	diff := "diff --git a/s.go b/s.go\n--- a/s.go\n+++ b/s.go\n@@ -1,5 +1,5 @@\n package main\n \n func F() {\n-x\n+y\n }\n"
	hunks := parseDiffHunks(diff)
	label := domain.Label{Name: "F", Type: domain.MOD_SIG, File: "s.go", Line: 3}
	lines := hunkLinesForLabel(label, hunksFor("s.go", hunks), 0)
	if len(lines) == 0 {
		t.Errorf("expected non-zero hunk lines for MOD_SIG label")
	}
}

// TestTrimContext_RemovesBlankContext verifies trimContext drops blank context
// lines and trims to the change window.
func TestTrimContext_RemovesBlankContext(t *testing.T) {
	lines := []hunkLine{
		{op: gitdiff.OpContext, content: ""},          // blank context — should be dropped
		{op: gitdiff.OpContext, content: "package"},  // context kept (near change)
		{op: gitdiff.OpDelete, content: "old"},      // delete
		{op: gitdiff.OpAdd, content: "new"},      // add
		{op: gitdiff.OpContext, content: ""},         // blank context — dropped
		{op: gitdiff.OpContext, content: "trailing"}, // far context
		{op: gitdiff.OpContext, content: "more"},     // far context
	}
	got := trimContext(lines)
	for _, l := range got {
		if l.op == 0 && strings.TrimSpace(l.content) == "" {
			t.Errorf("blank context line should be dropped; got %+v", l)
		}
	}
}