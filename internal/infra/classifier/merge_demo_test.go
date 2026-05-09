package classifier

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func TestClassifier_ConDiffReal(t *testing.T) {
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
	fmt.Println("\n═══════════════════════════════════════════════════════════════════════")
	fmt.Println("           ANÁLISIS REAL DEL ÚLTIMO COMMIT")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println(">>> DIF REAL (primeras 50 líneas):")
	lines := splitLines(rawDiff)
	displayCount := 50
	if len(lines) < displayCount {
		displayCount = len(lines)
	}
	for _, line := range lines[:displayCount] {
		fmt.Println(line)
	}
	if len(lines) > displayCount {
		fmt.Printf("... (%d líneas más)\n", len(lines)-displayCount)
	}
	fmt.Println("<<<")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("RESULTADO: %-10s (confianza: %.2f)\n", commitType, confidence)
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	
	fmt.Println("\n¿Es correcto?")
	fmt.Println("- 'feat' si es nueva funcionalidad")
	fmt.Println("- 'fix' si corregimos un bug")
	fmt.Println("- 'test' si son solo tests")
	fmt.Println("- 'refactor' si solo reorganizamos")
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
