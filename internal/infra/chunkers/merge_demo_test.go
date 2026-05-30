package chunkers

import (
	"fmt"
	"os"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
)

func TestMergeDemo(t *testing.T) {
	catalog := NewLanguageCatalog()
	if err := data.LoadLanguagesFromBytes([]byte(domain.FixtureJSON)); err != nil {
		os.Exit(1)
	}

	diff := "diff --git a/service.go b/service.go\n" +
		"index abc..def 100644\n" +
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

	u := NewUnifiedASTPass(catalog)
	chunk := &domain.DiffChunk{
		Files: []string{"service.go"},
		Diff:  diff,
	}
	before := []byte("package main\n\nfunc Process(x int) error {\n\treturn nil\n}\n\nfunc OldHelper() {}\n")
	after := []byte("package main\n\nfunc Process(x int) error {\n\treturn fmt.Errorf(\"updated\")\n}\n\nfunc NewFeature() {}\n")

	labels, _, _ := u.ProcessWithContent(chunk.Files[0], before, after, nil)
	for _, l := range labels {
		if chunk.AnnotatedDiff != "" {
			chunk.AnnotatedDiff += "\n"
		}
		chunk.AnnotatedDiff += fmt.Sprintf("📄 %s\n%s [%s] %s:%d\n", l.File, l.Name, l.Type, l.File, l.Line)
	}

	t.Logf(">>> ANTES (solo labels):\n%s<<<", chunk.AnnotatedDiff)

	MergeDiffIntoAnnotations(chunk, diff)
	t.Logf(">>> DESPUES (labels + diff):\n%s<<<", chunk.AnnotatedDiff)
}
