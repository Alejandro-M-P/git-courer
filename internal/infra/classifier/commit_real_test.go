package classifier

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func TestPipeline_CommitReal(t *testing.T) {
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
			annotated.WriteString(fmt.Sprintf("📄 %s\n", filename))
			annotated.WriteString(fmt.Sprintf("newFunc [NEW_FUNC] %s:1\n", filename))
		case "D":
			annotated.WriteString(fmt.Sprintf("📄 %s\n", filename))
			annotated.WriteString(fmt.Sprintf("oldFunc [DELETED_FUNC] %s:1\n", filename))
		case "M":
			annotated.WriteString(fmt.Sprintf("📄 %s\n", filename))
			if strings.Contains(filename, "_test.") {
				annotated.WriteString(fmt.Sprintf("testFunc [TEST] %s:1\n", filename))
			} else {
				annotated.WriteString(fmt.Sprintf("funcName [MOD_BODY] %s:1\n", filename))
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
	
	// 6. TABLA RESULTADO
	fmt.Println("\n═════════════════════════════════════════════════════════════════")
	fmt.Println("              COMMIT REAL DEL REPO")
	fmt.Println("═════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("📁 ARCHIVOS (%d):\n", len(files))
	for i, f := range files {
		fmt.Printf("   %d. %s\n", i+1, f)
	}
	fmt.Println()
	fmt.Println("📋 ANNOTATED DIFF:")
	fmt.Println(annotated.String())
	fmt.Println()
	fmt.Println("═════════════════════════════════════════════════════════════════")
	fmt.Printf("📊 RESULTADO FINAL:   %s (confianza: %.2f)\n", commitType, confidence)
	fmt.Println("═════════════════════════════════════════════════════════════════")
}
