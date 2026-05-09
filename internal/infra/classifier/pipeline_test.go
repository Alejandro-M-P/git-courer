package classifier

import (
	"fmt"
	"strings"
	"testing"
)

func TestPipeline_TablaVisualDetallada(t *testing.T) {
	c := NewClassifier(nil)
	
	type resultado struct {
		caso       string
		files      string
		labels     string
		diff       string
		goBefore   string
		goAfter    string
		pilar      string
		actual     string
		confidence float64
		expected   string
	}
	var resultados []resultado

	classify := func(name string, files []string, annotated string, diff string, before, after map[string]string, expected string, pilarEsperado string) {
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = files
		if diff != "" { chunk.Diff = diff }
		if before != nil { chunk.GoBefore = before }
		if after != nil { chunk.GoAfter = after }
		
		commitType, conf := c.Classify(chunk)
		
		// Resumir inputs
		filesStr := "-"
		if len(files) > 0 {
			filesStr = fmt.Sprintf("%d archivo(s): %s", len(files), strings.Join(files, ", "))
		}
		
		goBeforeStr := "No"
		if before != nil && len(before) > 0 {
			goBeforeStr = fmt.Sprintf("Sí (%d)", len(before))
		}
		
		goAfterStr := "No"
		if after != nil && len(after) > 0 {
			goAfterStr = fmt.Sprintf("Sí (%d)", len(after))
		}
		
		diffStr := "No"
		if diff != "" {
			lines := strings.Split(diff, "\n")
			if len(lines) > 2 {
				diffStr = fmt.Sprintf("Sí (%d líneas)", len(lines))
			} else {
				diffStr = fmt.Sprintf("Sí: %s", strings.ReplaceAll(diff, "\n", " | "))
			}
		}
		
		resultados = append(resultados, resultado{
			caso:     name,
			files:    filesStr,
			labels:   strings.Split(annotated, "\n")[1], // primera línea con label
			diff:     diffStr,
			goBefore: goBeforeStr,
			goAfter:  goAfterStr,
			pilar:    pilarEsperado,
			actual:   commitType,
			confidence: conf,
			expected: expected,
		})
	}

	// ============ CASOS ============
	
	// 1. OBVIOS - clasificación inmediata
	classify("CONFIG", nil, 
		"📄 go.mod\ngo.mod [CONFIG] go.mod", 
		"", nil, nil, "chore", "Obvio")
	
	classify("NEW_FUNC", nil, 
		"📄 api.go\nHandle [NEW_FUNC] api.go:42", 
		"", nil, nil, "feat", "Obvio")
	
	classify("TEST", []string{"test.go"}, 
		"📄 test.go\nTest [TEST] test.go:1", 
		"", nil, nil, "test", "Obvio")
	
	// 2. PILAR 1 - Code-Test Symmetry
	classify("Symmetry", []string{"a.go", "a_test.go"}, 
		"📄 a.go\nFunc [MOD_BODY] a.go:1\n📄 a_test.go\nTest [TEST] a_test.go:1", 
		"", nil, nil, "fix", "Pilar 1")
	
	// 3. PILAR 3 - AST Identity (misma lógica = refactor)
	classify("AST_rename", []string{"math.go"}, 
		"📄 math.go\nadd [MOD_BODY] math.go:2", 
		"",
		map[string]string{"math.go": `package p; func add(a int) int { return a }`},
		map[string]string{"math.go": `package p; func add(x int) int { return x }`},
		"refactor", "Pilar 3")
	
	// 4. PILAR 3 - AST Identity (lógica cambió = cae a fallback)
	classify("AST_logic", []string{"math.go"}, 
		"📄 math.go\ncalc [MOD_BODY] math.go:2", 
		"",
		map[string]string{"math.go": `package p; func calc(a int) int { return a + 1 }`},
		map[string]string{"math.go": `package p; func calc(a int) int { return a - 1 }`},
		"fix", "Fallback")
	
	// 5. PILAR 2 - Operator Mutation
	classify("Op_>→>=", []string{"f.go"}, 
		"📄 f.go\nFunc [MOD_BODY] f.go:1", 
		"- if x > 10\n+ if x >= 10", 
		nil, nil, "fix", "Pilar 2")
	
	classify("Op_&&→||", []string{"f.go"}, 
		"📄 f.go\nFunc [MOD_BODY] f.go:1", 
		"- a && b\n+ a || b", 
		nil, nil, "fix", "Pilar 2")
	
	classify("Op_==→!=", []string{"f.go"}, 
		"📄 f.go\nFunc [MOD_BODY] f.go:1", 
		"- if s == \"ok\"\n+ if s != \"ok\"", 
		nil, nil, "fix", "Pilar 2")
	
	// 6. FALLBACK - sin nada específico
	classify("Fallback", []string{"f.go"}, 
		"📄 f.go\nFunc [MOD_BODY] f.go:1", 
		"- hello\n+ world", 
		nil, nil, "fix", "Fallback")

	// ============ IMPRIMIR TABLA ============
	fmt.Println("\n═════════════════════════════════════════════════════════════════════")
	fmt.Println("  PIPELINE DE CLASIFICACIÓN - INPUTS & OUTPUTS")
	fmt.Println("═════════════════════════════════════════════════════════════════════")
	
	for i, r := range resultados {
		ok := "✓ Bien"
		if r.expected != r.actual {
			ok = "✗ MAL"
		}
		
		fmt.Printf("\n┌─ CASO %d: %-20s %s\n", i+1, r.caso, ok)
		fmt.Printf("│\n")
		fmt.Printf("│  ENTRA:\n")
		fmt.Printf("│    Archivos:    %s\n", r.files)
		fmt.Printf("│    Labels:      %s\n", r.labels)
		fmt.Printf("│    Diff:        %s\n", r.diff)
		fmt.Printf("│    GoBefore:    %s\n", r.goBefore)
		fmt.Printf("│    GoAfter:     %s\n", r.goAfter)
		fmt.Printf("│\n")
		fmt.Printf("│  DECIDE (%s):\n", r.pilar)
		fmt.Printf("│    → %s (%.2f confianza)\n", r.actual, r.confidence)
		fmt.Printf("│\n")
		fmt.Printf("│  ESPERADO: %s\n", r.expected)
		fmt.Printf("└─────────────────────────────────────────────────────────────────\n")
	}
	
	fmt.Println("\n¿Cuáles están mal? Decime el número.")
}
