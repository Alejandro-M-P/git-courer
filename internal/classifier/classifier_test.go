package classifier

import (
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		input   string
		wantCat Category
		wantOp  string
	}{
		// ReadOnly
		{"show status", ReadOnly, "status"},
		{"muéstrame el estado", ReadOnly, "status"},
		{"show log", ReadOnly, "log"},
		{"show diff", ReadOnly, "diff"},
		{"show history", ReadOnly, "log"},
		{"status", ReadOnly, "status"},
		{"log", ReadOnly, "log"},
		{"diff", ReadOnly, "diff"},
		{"dame el diff", ReadOnly, "diff"},
		{"ver historial", ReadOnly, "log"},

		// SimpleWrite
		{"create branch feature/auth", SimpleWrite, "branch"},
		{"crea una rama para el login", SimpleWrite, "branch"},
		{"checkout main", SimpleWrite, "checkout"},
		{"switch to develop", SimpleWrite, "checkout"},
		{"stash changes", SimpleWrite, "stash"},
		{"git reset", SimpleWrite, "reset"},

		// ComplexWrite
		{"commit all changes", ComplexWrite, "commit"},
		{"commit and push", ComplexWrite, "commit"},
		{"push to remote", ComplexWrite, "push"},
		{"pull latest", ComplexWrite, "pull"},
		{"merge develop into main", ComplexWrite, "merge"},
		{"rebase onto main", ComplexWrite, "rebase"},
		{"save my changes", ComplexWrite, "commit"},
		{"subir los cambios", ComplexWrite, "push"},
		{"guardar todo y subir", ComplexWrite, "commit"},
		{"commit all changes and push", ComplexWrite, "commit"},

		// Non-Latin scripts → Unknown (will use Ollama)
		{"提交所有更改", Unknown, ""},
		{"коммит", Unknown, ""},
		{"サブミット", Unknown, ""},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			result := Classify(tc.input)
			if result.Category != tc.wantCat {
				t.Errorf("Classify(%q): category = %v, want %v", tc.input, result.Category, tc.wantCat)
			}
			if result.Operation != tc.wantOp {
				t.Errorf("Classify(%q): operation = %q, want %q", tc.input, result.Operation, tc.wantOp)
			}
		})
	}
}
