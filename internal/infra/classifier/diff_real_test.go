package classifier

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPipeline_TablaReal(t *testing.T) {
	t.Skip("manual demo")

	cmd := exec.Command("git", "diff", "--name-only", "HEAD~1")
	cmd.Dir = "/git-courer"
	out, _ := cmd.Output()
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	
	cmdDiff := exec.Command("git", "diff", "HEAD~1")
	cmdDiff.Dir = "/git-courer"
	outDiff, _ := cmdDiff.Output()
	diff := string(outDiff)
	
	c := NewClassifier(nil)
	chunk := newAnnotatedFixture("📄 cambios\nFunciones [MOD_BODY] cambios:1")
	chunk.Files = files
	chunk.Diff = diff
	
	commitType, confidence := c.Classify(chunk)
	
	t.Logf("Archivos: %d archivos", len(files))
	t.Logf("Labels: MOD_BODY (modificación de cuerpo)")
	t.Logf("Dif: %d líneas", len(strings.Split(diff, "\n")))
	t.Logf("→ RESULTADO: %s (%.2f confianza)", commitType, confidence)
}
