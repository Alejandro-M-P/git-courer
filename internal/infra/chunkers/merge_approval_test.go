package chunkers

import (
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// TestMergeDiffIntoAnnotations_Approval captures the current behavior of
// MergeDiffIntoAnnotations so it can be safely removed/replaced by the
// structured buildAnnotatedEntries path. This is an approval test: it
// documents what the emoji merge does NOW.
func TestMergeDiffIntoAnnotations_Approval(t *testing.T) {
	diff := "diff --git a/service.go b/service.go\n" +
		"--- a/service.go\n" +
		"+++ b/service.go\n" +
		"@@ -1,7 +1,7 @@\n" +
		" package main\n" +
		" \n" +
		" func Process(x int) error {\n" +
		"-	return nil\n" +
		"+	return fmt.Errorf(\"updated\")\n" +
		" }\n" +
		" \n" +
		"-func OldHelper() {}\n" +
		"+func NewFeature() {}\n"

	chunk := &domain.DiffChunk{
		Files: []string{"service.go"},
		Diff:  diff,
		AnnotatedDiff: "📄 service.go\n" +
			"Process [MOD_SIG] service.go:3\n" +
			"OldHelper [DELETED_FUNC] service.go:6\n" +
			"NewFeature [NEW_FUNC] service.go:7\n",
	}

	MergeDiffIntoAnnotations(chunk, diff)

	out := chunk.AnnotatedDiff
	if !strings.Contains(out, "📄 service.go") {
		t.Errorf("approval: output missing file header; got:\n%s", out)
	}
	if !strings.Contains(out, "[MOD_SIG]") {
		t.Errorf("approval: output missing MOD_SIG label; got:\n%s", out)
	}
	if !strings.Contains(out, "-\treturn nil") {
		t.Errorf("approval: output missing deleted hunk line; got:\n%s", out)
	}
	if !strings.Contains(out, "+\treturn fmt.Errorf") {
		t.Errorf("approval: output missing added hunk line; got:\n%s", out)
	}
}

// TestBuildAnnotatedEntries_ProducesSameContentAsMerge verifies the new
// structured path produces entries whose before/after contain the same hunk
// lines as the legacy MergeDiffIntoAnnotations output. This validates the
// replacement is behavior-equivalent for the hunk-extraction portion.
func TestBuildAnnotatedEntries_ProducesSameContentAsMerge(t *testing.T) {
	diff := "diff --git a/service.go b/service.go\n" +
		"--- a/service.go\n" +
		"+++ b/service.go\n" +
		"@@ -1,7 +1,7 @@\n" +
		" package main\n" +
		" \n" +
		" func Process(x int) error {\n" +
		"-	return nil\n" +
		"+	return fmt.Errorf(\"updated\")\n" +
		" }\n" +
		" \n" +
		"-func OldHelper() {}\n" +
		"+func NewFeature() {}\n"

	labels := []domain.Label{
		{Name: "Process", Type: domain.MOD_SIG, File: "service.go", Line: 3},
		{Name: "OldHelper", Type: domain.DELETED_FUNC, File: "service.go", Line: 6},
		{Name: "NewFeature", Type: domain.NEW_FUNC, File: "service.go", Line: 7},
	}
	hunks := parseDiffHunks(diff)

	entries := buildAnnotatedEntries(labels, hunks)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	bySymbol := map[string]domain.AnnotatedEntry{}
	for _, e := range entries {
		bySymbol[e.Symbol] = e
	}
	if e, ok := bySymbol["Process"]; !ok || !strings.Contains(e.After, "fmt.Errorf") {
		t.Errorf("Process entry missing updated hunk line; got %+v", e)
	}
	if e, ok := bySymbol["OldHelper"]; !ok || !strings.Contains(e.Before, "OldHelper") {
		t.Errorf("OldHelper entry missing deleted hunk line; got %+v", e)
	}
	if e, ok := bySymbol["NewFeature"]; !ok || !strings.Contains(e.After, "NewFeature") {
		t.Errorf("NewFeature entry missing added hunk line; got %+v", e)
	}
}