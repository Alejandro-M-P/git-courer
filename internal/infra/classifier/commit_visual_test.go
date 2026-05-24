package classifier

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func TestCommitVisual(t *testing.T) {
	t.Skip("manual demo")

	c := NewClassifier(nil)

	t.Log("VISUALIZACIÓN DE COMMIT CON CHUNKS")

	// CHUNK 1: Nuevo archivo (sin test)
	chunk1 := &domain.DiffChunk{
		Files: []string{"internal/infra/classifier/operator_mutation.go"},
		AnnotatedDiff: "📄 internal/infra/classifier/operator_mutation.go\n" +
			"detectOperatorMutation [NEW_FUNC] internal/infra/classifier/operator_mutation.go:1",
		Diff: "+package classifier\n+func detectOperatorMutation(diff string) (string, float64) {\n+    // Detecta operadores\n+}",
	}

	t1, c1 := c.Classify(chunk1)
	t.Logf("CHUNK 1 — NUEVO ARCHIVO (sin test): %s (%.2f) [esperado: feat]", t1, c1)

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
	t.Logf("CHUNK 2 — CODE + TEST (pareados): %s (%.2f) [esperado: fix]", t2, c2)

	// CHUNK 3: Solo modificación (sin test)
	chunk3 := &domain.DiffChunk{
		Files: []string{"internal/adapters/llm/openai_standard/adapter_commit.go"},
		AnnotatedDiff: "📄 internal/adapters/llm/openai_standard/adapter_commit.go\n" +
			"GenerateChunkMessage [MOD_BODY] internal/adapters/llm/openai_standard/adapter_commit.go:50",
		Diff: "-annotatedDiff := \"\"\n+annotatedDiff := chunk.AnnotatedDiff",
	}

	t3, c3 := c.Classify(chunk3)
	t.Logf("CHUNK 3 — SOLO CODE (sin test): %s (%.2f) [esperado: fix]", t3, c3)

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
	t.Logf("CHUNK 4 — RENAME (misma lógica): %s (%.2f) [esperado: refactor]", t4, c4)

	// CHUNK 5: Cambio de operador
	chunk5 := &domain.DiffChunk{
		Files: []string{"validador.go"},
		AnnotatedDiff: "📄 validador.go\nesMayor [MOD_BODY] validador.go:5",
		Diff: "-if edad > 18 {\n+if edad >= 18 {",
	}

	t5, c5 := c.Classify(chunk5)
	t.Logf("CHUNK 5 — CAMBIO OPERADOR: %s (%.2f) [esperado: fix]", t5, c5)
}
