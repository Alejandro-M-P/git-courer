package classifier

import (
	"os/exec"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func TestClassifier_ConDiffReal(t *testing.T) {
	t.Skip("manual demo — run without -short")

	// 1. COJO EL DIF REAL
	cmd := exec.Command("git", "diff", "HEAD~1")
	cmd.Dir = "/git-courer"
	out, _ := cmd.CombinedOutput()
	rawDiff := string(out)
	
	if rawDiff == "" {
		t.Skip("No hay dif para analizar")
	}
	
	// 2. CLASIFICO directamente con el dif
	chunk := &domain.DiffChunk{Diff: rawDiff}
	
	c := NewClassifier(nil)
	commitType, confidence := c.Classify(chunk)
	
	// 3. MUESTRO
	t.Log("\n═══════════════════════════════════════════════════════════════════════")
	t.Log("           ANÁLISIS REAL DEL ÚLTIMO COMMIT")
	t.Log("═══════════════════════════════════════════════════════════════════════")
	t.Log()
	t.Log(">>> DIF REAL (primeras 50 líneas):")
	lines := splitLines(rawDiff)
	displayCount := 50
	if len(lines) < displayCount {
		displayCount = len(lines)
	}
	for _, line := range lines[:displayCount] {
		t.Log(line)
	}
	if len(lines) > displayCount {
		t.Logf("... (%d líneas más)", len(lines)-displayCount)
	}
	t.Log("<<<")
	t.Log()
	t.Log("═══════════════════════════════════════════════════════════════════════")
	t.Logf("RESULTADO: %-10s (confianza: %.2f)", commitType, confidence)
	t.Log("═══════════════════════════════════════════════════════════════════════")
	
	t.Log("\n¿Es correcto?")
	t.Log("- 'feat' si es nueva funcionalidad")
	t.Log("- 'fix' si corregimos un bug")
	t.Log("- 'test' si son solo tests")
	t.Log("- 'refactor' si solo reorganizamos")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
