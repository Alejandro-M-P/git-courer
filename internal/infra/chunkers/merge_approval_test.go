package chunkers

import (
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// TestBuildAnnotatedEntries_ProducesSameContentAsMerge verifies the new
// structured path produces entries whose before/after contain the same hunk
// lines as the legacy MergeDiffIntoAnnotations output did. This validates the
// replacement is behavior-equivalent for the hunk-extraction portion.
// (The legacy MergeDiffIntoAnnotations was removed in Phase 2 once this
// equivalence was proven and the new path was fully wired.)
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