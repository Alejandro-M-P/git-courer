package classifier

import (
	"fmt"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func TestCommitVisual(t *testing.T) {
	// Simulo un commit REAL con chunks separados
	// (como hace el chunker en producción)
	
	c := NewClassifier(nil)
	
	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("     VISUALIZACIÓN DE COMMIT CON CHUNKS")
	fmt.Println("═══════════════════════════════════════════════════════════")
	
	// CHUNK 1: Nuevo archivo (sin test)
	chunk1 := &domain.DiffChunk{
		Files: []string{"internal/infra/classifier/operator_mutation.go"},
		AnnotatedDiff: "📄 internal/infra/classifier/operator_mutation.go\n" +
			"detectOperatorMutation [NEW_FUNC] internal/infra/classifier/operator_mutation.go:1",
		Diff: "+package classifier\n+func detectOperatorMutation(diff string) (string, float64) {\n+    // Detecta operadores\n+}",
	}
	
	t1, c1 := c.Classify(chunk1)
	fmt.Println("\n📦 CHUNK 1: NUEVO ARCHIVO (sin test)")
	fmt.Printf("   Archivo: %s\n", chunk1.Files[0])
	fmt.Printf("   Labels:  NEW_FUNC\n")
	fmt.Printf("   → %s (%.2f)\n", t1, c1)
	fmt.Println("   Esperado: feat")
	
	// CHUNK 2: Modificación + test
	chunk2 := &domain.DiffChunk{
		Files: []string{"internal/infra/classifier/classifier.go", "internal/infra/classifier/classifier_test.go"},
		AnnotatedDiff: "📄 internal/infra/classifier/classifier.go\n" +
			"Classify [MOD_BODY] internal/infra/classifier/classifier.go:50\n" +
			"📄 internal/infra/classifier/classifier_test.go\n" +
			"TestClassify [TEST] internal/infra/classifier/classifier_test.go:1",
		Diff: "-commitType, confidence := c.determineType(labels, chunk.Files, chunk.GoBefore, chunk.GoAfter)\n+commitType, confidence := c.determineType(labels, chunk.Files, chunk.GoBefore, chunk.GoAfter, chunk.Diff)",
	}
	
	t2, c2 := c.Classify(chunk2)
	fmt.Println("\n📦 CHUNK 2: CODE + TEST (pareados)")
	fmt.Printf("   Archivos: %v\n", chunk2.Files)
	fmt.Printf("   Labels:   MOD_BODY + TEST\n")
	fmt.Printf("   → %s (%.2f)\n", t2, c2)
	fmt.Println("   Esperado: fix (por simetría)")
	
	// CHUNK 3: Solo modificación (sin test)
	chunk3 := &domain.DiffChunk{
		Files: []string{"internal/adapters/llm/openai_standard/adapter_commit.go"},
		AnnotatedDiff: "📄 internal/adapters/llm/openai_standard/adapter_commit.go\n" +
			"GenerateChunkMessage [MOD_BODY] internal/adapters/llm/openai_standard/adapter_commit.go:50",
		Diff: "-annotatedDiff := \"\"\n+annotatedDiff := chunk.AnnotatedDiff",
	}
	
	t3, c3 := c.Classify(chunk3)
	fmt.Println("\n📦 CHUNK 3: SOLO CODE (sin test)")
	fmt.Printf("   Archivo: %s\n", chunk3.Files[0])
	fmt.Printf("   Labels:  MOD_BODY\n")
	fmt.Printf("   → %s (%.2f)\n", t3, c3)
	fmt.Println("   Esperado: fix (fallback genérico)")
	
	// CHUNK 4: Rename de variable (misma lógica)
	before := `package p
func add(a int, b int) int {
	return a + b
}`
	after := `package p
func add(x int, y int) int {
	return x + y
}`
	
	chunk4 := &domain.DiffChunk{
		Files: []string{"math.go"},
		AnnotatedDiff: "📄 math.go\nadd [MOD_BODY] math.go:2",
		GoBefore: map[string]string{"math.go": before},
		GoAfter:  map[string]string{"math.go": after},
		Diff: "-func add(a int, b int) int {\n+func add(x int, y int) int {",
	}
	
	t4, c4 := c.Classify(chunk4)
	fmt.Println("\n📦 CHUNK 4: RENAME (misma lógica)")
	fmt.Printf("   Archivo: %s\n", chunk4.Files[0])
	fmt.Printf("   Labels:  MOD_BODY\n")
	fmt.Printf("   GoBefore/After: Sí (AST disponible)\n")
	fmt.Printf("   → %s (%.2f)\n", t4, c4)
	fmt.Println("   Esperado: refactor (mismo hash AST)")
	
	// CHUNK 5: Cambio de operador
	chunk5 := &domain.DiffChunk{
		Files: []string{"validador.go"},
		AnnotatedDiff: "📄 validador.go\nesMayor [MOD_BODY] validador.go:5",
		Diff: "-if edad > 18 {\n+if edad >= 18 {",
	}
	
	t5, c5 := c.Classify(chunk5)
	fmt.Println("\n📦 CHUNK 5: CAMBIO OPERADOR")
	fmt.Printf("   Archivo: %s\n", chunk5.Files[0])
	fmt.Printf("   Labels:  MOD_BODY\n")
	fmt.Printf("   Diff:    > → >=\n")
	fmt.Printf("   → %s (%.2f)\n", t5, c5)
	fmt.Println("   Esperado: fix (Pilar 2)")
	
	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("¿Cuáles están mal?")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
