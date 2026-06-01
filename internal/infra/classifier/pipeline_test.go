package classifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeline_ClassificationTable(t *testing.T) {
	c := NewClassifier(nil)

	type testCase struct {
		name      string
		files     []string
		annotated string
		diff      string
		before    map[string]string
		after     map[string]string
		expected  string
	}

	tests := []testCase{
		// 1. OBVIOS — clasificación inmediata
		{name: "CONFIG", annotated: "go.mod [CONFIG] go.mod", expected: "chore"},
		{name: "NEW_FUNC", annotated: "api.go\nHandle [NEW_FUNC] api.go:42", expected: "feat"},
		{name: "TEST", files: []string{"test.go"}, annotated: "test.go\nTest [TEST] test.go:1", expected: "test"},

		// 2. PILAR 1 — Code-Test Symmetry
		{name: "Symmetry", files: []string{"a.go", "a_test.go"},
			annotated: "a.go\nFunc [MOD_BODY] a.go:1\na_test.go\nTest [TEST] a_test.go:1", expected: "fix"},

		// 3. PILAR 3 — AST Identity (misma lógica = refactor)
		{name: "AST_rename", files: []string{"math.go"},
			annotated: "math.go\nsum [MOD_BODY] math.go:2",
			before:    map[string]string{"math.go": `package p; func add(a int) int { return a }`},
			after:     map[string]string{"math.go": `package p; func sum(x int) int { return x }`},
			expected:  "refactor"},

		// 4. PILAR 3 — AST Identity (lógica cambió = cae a fallback)
		{name: "AST_logic", files: []string{"math.go"},
			annotated: "math.go\ncalc [MOD_BODY] math.go:2",
			before:    map[string]string{"math.go": `package p; func calc(a int) int { return a + 1 }`},
			after:     map[string]string{"math.go": `package p; func calc(a int) int { return a - 1 }`},
			expected:  "fix"},

		// 5. PILAR 2 — Operator Mutation
		{name: "Op_>→>=", files: []string{"f.go"},
			annotated: "f.go\nFunc [MOD_BODY] f.go:1",
			diff:      "- if x > 10\n+ if x >= 10",
			expected:  "fix"},
		{name: "Op_&&→||", files: []string{"f.go"},
			annotated: "f.go\nFunc [MOD_BODY] f.go:1",
			diff:      "- a && b\n+ a || b",
			expected:  "fix"},
		{name: "Op_==→!=", files: []string{"f.go"},
			annotated: "f.go\nFunc [MOD_BODY] f.go:1",
			diff:      "- if s == \"ok\"\n+ if s != \"ok\"",
			expected:  "fix"},

		// 6. FALLBACK — sin nada específico
		{name: "Fallback", files: []string{"f.go"},
			annotated: "f.go\nFunc [MOD_BODY] f.go:1",
			diff:      "- hello\n+ world",
			expected:  "fix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := newAnnotatedFixture(tt.annotated)
			chunk.Files = tt.files
			if tt.diff != "" {
				chunk.Diff = tt.diff
			}
			if tt.before != nil {
				chunk.BeforeSource = tt.before
			}
			if tt.after != nil {
				chunk.AfterSource = tt.after
			}

			commitType, confidence := c.Classify(chunk)
			assert.Equal(t, tt.expected, commitType, "commit type should match expected")
			assert.Greater(t, confidence, 0.0, "confidence should be > 0")
		})
	}
}
