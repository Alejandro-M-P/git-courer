package classifier

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

func TestPipeline_CommitReal(t *testing.T) {
	t.Skip("manual demo")

	// 1. COJO INFO DEL ÚLTIMO COMMIT
	cmd := exec.Command("git", "diff", "--name-status", "HEAD~1")
	cmd.Dir = "/git-courer"
	out, _ := cmd.CombinedOutput()

	// 2. GENERO ANNOTATED DIFF REAL
	var annotated strings.Builder
	annotated.WriteString("📄 Commit actual\n\n")

	files := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		status := parts[0]
		filename := parts[1]
		files = append(files, filename)

		switch status {
		case "A":
			annotated.WriteString("📄 " + filename + "\n")
			annotated.WriteString("newFunc [NEW_FUNC] " + filename + ":1\n")
		case "D":
			annotated.WriteString("📄 " + filename + "\n")
			annotated.WriteString("oldFunc [DELETED_FUNC] " + filename + ":1\n")
		case "M":
			annotated.WriteString("📄 " + filename + "\n")
			if strings.Contains(filename, "_test.") {
				annotated.WriteString("testFunc [TEST] " + filename + ":1\n")
			} else {
				annotated.WriteString("funcName [MOD_BODY] " + filename + ":1\n")
			}
		}
	}

	// 3. COJO DIF REAL
	cmdDiff := exec.Command("git", "diff", "HEAD~1")
	cmdDiff.Dir = "/git-courer"
	diffOut, _ := cmdDiff.CombinedOutput()
	diff := string(diffOut)

	// 4. CREO CHUNK
	chunk := &domain.DiffChunk{
		Files:         files,
		AnnotatedDiff: annotated.String(),
		Diff:          diff,
	}

	// 5. CLASIFICO
	c := NewClassifier(nil)
	commitType, confidence := c.Classify(chunk)

	// 6. RESULTADO
	t.Logf("ARCHIVOS (%d):", len(files))
	for i, f := range files {
		t.Logf("  %d. %s", i+1, f)
	}
	t.Logf("RESULTADO FINAL: %s (confianza: %.2f)", commitType, confidence)
}
