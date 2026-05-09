package classifier

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestPipeline_TablaReal(t *testing.T) {
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
	
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           CLASIFICACIÓN DEL DIF REAL                       ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Archivos:     %-45s ║\n", fmt.Sprintf("%d archivos", len(files)))
	fmt.Printf("║ Labels:       %-45s ║\n", "MOD_BODY (modificación de cuerpo)")
	fmt.Printf("║ Dif:          %-45s ║\n", fmt.Sprintf("%d líneas", len(strings.Split(diff, "\n"))))
	fmt.Printf("║ Pilar que      %-45s ║\n", "Test detection (archivos *_test.go)")
	fmt.Printf("║  decidió:                                              ║\n")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ → RESULTADO: %-10s (%.2f confianza)               ║\n", commitType, confidence)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	
	fmt.Println("\n¿Es correcto este resultado?")
	fmt.Println("- Esperabas 'fix' porque implementaste Pilar 2?")
	fmt.Println("- O 'test' porque hay archivos *_test.go nuevos?")
	fmt.Println("- O 'feat' porque es funcionalidad nueva?")
}
