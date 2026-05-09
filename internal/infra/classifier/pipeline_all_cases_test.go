package classifier

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
)

// makeDiff genera un diff unificado REAL entre before y after usando git.
func makeDiff(filename string, before, after []byte) string {
	dir, _ := os.MkdirTemp("", "diff-*")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "a.go"), before, 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), after, 0o644)
	cmd := exec.Command("git", "diff", "--no-index", "--", "a.go", "b.go")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	s := string(out)
	// Replace placeholder names with real filename
	s = strings.ReplaceAll(s, "diff --git a/a.go b/b.go",
		fmt.Sprintf("diff --git a/%s b/%s", filename, filename))
	s = strings.ReplaceAll(s, "--- a/a.go", fmt.Sprintf("--- a/%s", filename))
	s = strings.ReplaceAll(s, "+++ b/b.go", fmt.Sprintf("+++ b/%s", filename))
	return s
}

func runCase(name string, t *testing.T, filename string, before, after []byte) {
	diff := makeDiff(filename, before, after)
	c := NewClassifier(nil)
	annotator := chunkers.NewASTAnnotator()

	chunk := &domain.DiffChunk{
		Files: []string{filename},
		Diff:  diff,
	}

	annotator.Annotate(chunk, filename, before, after)
	chunkers.MergeDiffIntoAnnotations(chunk, diff)

	commitType, confidence := c.Classify(chunk)

	fmt.Printf("\n══════════════════════════════════════════════════════\n")
	fmt.Printf("──  %s ──\n", name)
	fmt.Printf("  File: %s\n", filename)
	fmt.Printf("  Classifier: %s (%.0f%%)\n", commitType, confidence*100)

	annotatedWithType := chunk.AnnotatedDiff
	if annotatedWithType != "" {
		annotatedWithType = strings.ReplaceAll(annotatedWithType, "\n[", "\n"+commitType+" [")
		if strings.HasPrefix(annotatedWithType, "[") {
			annotatedWithType = commitType + " " + annotatedWithType
		}
	}
	fmt.Printf("  AnnotatedDiff (input al LLM):\n%s\n", annotatedWithType)
	fmt.Printf("══════════════════════════════════════════════════════\n")
}

func TestPipeline_AllCases(t *testing.T) {
	fmt.Println("══════════════════════════════════════════════════════")
	fmt.Println("  PIPELINE: Todos los casos posibles de AnnotatedDiff")
	fmt.Println("══════════════════════════════════════════════════════")

	// ───────────────────────────────────────────────────────
	// 1. MOD_BODY con cambio visible
	// ───────────────────────────────────────────────────────
	before1 := []byte("package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	after1 := []byte("package main\n\nfunc Add(a, b int) int {\n\tresult := a + b\n\treturn result\n}\n")
	runCase("1. MOD_BODY con cambio visible", t, "calc.go", before1, after1)

	// ───────────────────────────────────────────────────────
	// 2. MOD_BODY sin cambio visible (el diff toca otro lado)
	// ───────────────────────────────────────────────────────
	before2 := []byte("package main\n\nfunc Add(a, b int) int { return a + b }\nfunc Sub(a, b int) int { return a - b }\n")
	after2 := []byte("package main\n\n// Copyright 2024\nfunc Add(a, b int) int { return a + b }\nfunc Sub(a, b int) int { return a - b }\n")
	runCase("2. MOD_BODY sin cambio visible", t, "calc2.go", before2, after2)

	// ───────────────────────────────────────────────────────
	// 3. MOD_TYPE (struct modificado)
	// ───────────────────────────────────────────────────────
	before3 := []byte("package main\n\ntype User struct {\n\tName string\n}\n")
	after3 := []byte("package main\n\ntype User struct {\n\tName string\n\tAge  int\n}\n")
	runCase("3. MOD_TYPE", t, "types.go", before3, after3)

	// ───────────────────────────────────────────────────────
	// 4. MOD_SIG breaking (pública)
	// ───────────────────────────────────────────────────────
	before4 := []byte("package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	after4 := []byte("package main\n\nfunc Add(a, b, c int) int {\n\treturn a + b + c\n}\n")
	runCase("4. MOD_SIG breaking (public)", t, "api.go", before4, after4)

	// ───────────────────────────────────────────────────────
	// 5. NEW_FUNC
	// ───────────────────────────────────────────────────────
	before5 := []byte("package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	after5 := []byte("package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n")
	runCase("5. NEW_FUNC", t, "calc_new.go", before5, after5)

	// ───────────────────────────────────────────────────────
	// 6. NEW_TYPE
	// ───────────────────────────────────────────────────────
	before6 := []byte("package main\n\ntype User struct {\n\tName string\n}\n")
	after6 := []byte("package main\n\ntype User struct {\n\tName string\n}\n\ntype Repository interface {\n\tFindByID(id string) *User\n}\n")
	runCase("6. NEW_TYPE", t, "repository.go", before6, after6)

	// ───────────────────────────────────────────────────────
	// 7. DELETED_FUNC
	// ───────────────────────────────────────────────────────
	before7 := []byte("package main\n\nfunc Add(a, b int) int { return a + b }\nfunc OldHandler(w string) {}\n")
	after7 := []byte("package main\n\nfunc Add(a, b int) int { return a + b }\n")
	runCase("7. DELETED_FUNC", t, "cleanup.go", before7, after7)

	// ───────────────────────────────────────────────────────
	// 8. DELETED_TYPE
	// ───────────────────────────────────────────────────────
	before8 := []byte("package main\n\ntype OldConfig struct {\n\tDebug bool\n}\ntype NewConfig struct {\n\tDebug bool\n}\n")
	after8 := []byte("package main\n\ntype NewConfig struct {\n\tDebug bool\n}\n")
	runCase("8. DELETED_TYPE", t, "config_cleanup.go", before8, after8)

	// ───────────────────────────────────────────────────────
	// 9. DOCS (.md)
	// ───────────────────────────────────────────────────────
	before9 := []byte("# Title\n")
	after9 := []byte("# Title\n\n## Section 1\n\nBody text here.\n")
	runCase("9. DOCS", t, "README.md", before9, after9)

	// ───────────────────────────────────────────────────────
	// 10. Mixto: NEW_FUNC + MOD_BODY
	// ───────────────────────────────────────────────────────
	before10 := []byte("package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	after10 := []byte("package main\n\nfunc Add(a, b int) int {\n\tsum := a + b\n\treturn sum\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n")
	runCase("10. Mixto: NEW_FUNC + MOD_BODY", t, "mixed.go", before10, after10)

	// ───────────────────────────────────────────────────────
	// 11. Reemplazo: DELETED_FUNC + NEW_FUNC
	// ───────────────────────────────────────────────────────
	before11 := []byte("package main\n\nfunc OldHelper(x int) int {\n\treturn x * 2\n}\n")
	after11 := []byte("package main\n\nfunc NewHelper(x int) int {\n\treturn x * 3\n}\n")
	runCase("11. Reemplazo: DELETED_FUNC + NEW_FUNC", t, "replace.go", before11, after11)

	// ───────────────────────────────────────────────────────
	// 12. CONFIG (.json)
	// ───────────────────────────────────────────────────────
	before12 := []byte(`{"name": "app"}`)
	after12 := []byte(`{"name": "app", "version": "1.0"}`)
	runCase("12. CONFIG (.json)", t, "package.json", before12, after12)

	// ───────────────────────────────────────────────────────
	// 13. CI (.yml)
	// ───────────────────────────────────────────────────────
	before13 := []byte("name: CI\non: push\n")
	after13 := []byte("name: CI\non: [push, pull_request]\n")
	runCase("13. CI (workflow)", t, ".github/workflows/ci.yml", before13, after13)

	// ───────────────────────────────────────────────────────
	// 14. MOD_SIG privada (no breaking)
	// ───────────────────────────────────────────────────────
	before14 := []byte("package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}\n")
	after14 := []byte("package main\n\nfunc add(a, b, c int) int {\n\treturn a + b + c\n}\n")
	runCase("14. MOD_SIG privada (NO breaking)", t, "internal.go", before14, after14)

	// ───────────────────────────────────────────────────────
	// 15. MOD_SIG + MOD_BODY en mismo archivo
	// ───────────────────────────────────────────────────────
	before15 := []byte("package main\n\nfunc GetUser(id string) string {\n\treturn db\n}\n\nfunc CreateUser(u string) error {\n\treturn db\n}\n")
	after15 := []byte("package main\n\nfunc GetUser(ctx context.Context, id string) string {\n\treturn db\n}\n\nfunc CreateUser(u string) error {\n\treturn db + u\n}\n")
	runCase("15. MOD_SIG + MOD_BODY en mismo archivo", t, "service.go", before15, after15)

	// ───────────────────────────────────────────────────────
	// 16. BREAKING + operator mutation (> to <=)
	// ───────────────────────────────────────────────────────
	before16 := []byte("package main\n\nfunc Threshold(x int) bool {\n\treturn x > 10\n}\n")
	after16 := []byte("package main\n\nfunc Threshold(x int) bool {\n\treturn x <= 10\n}\n")
	runCase("16. MOD_BODY + operator mutation", t, "operator.go", before16, after16)

	fmt.Println("\n══════════════════════════════════════════════════════")
	fmt.Println("  FIN: Todos los casos mostrados")
	fmt.Println("══════════════════════════════════════════════════════")
}
