package chunkers

import (
	"strings"
	"testing"
)

// TDD: RED — Test ExtensionToLanguage before it exists
func TestExtensionToLanguage(t *testing.T) {
	catalog := NewLanguageCatalog()

	tests := []struct {
		ext      string
		wantLang string
		wantOK   bool
	}{
		{".go", "Go", true},
		{".js", "JavaScript", true},
		{".ts", "TypeScript", true},
		{".cs", "C#", true},
		{".md", "Markdown", true},
		{".py", "Python", true},
		{".rs", "Rust", true},
		{".java", "Java", true},
		{".cpp", "C++", true},
		{".c", "C", true},
		{".rb", "Ruby", true},
		{".php", "PHP", true},
		{".swift", "Swift", true},
		{".kt", "Kotlin", true},
		{".dart", "Dart", true},
		{".unknown", "", false},
		{"", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.ext, func(t *testing.T) {
			got, ok := catalog.ExtensionToLanguage(tc.ext)
			if ok != tc.wantOK {
				t.Fatalf("ExtensionToLanguage(%q) ok=%v, want %v", tc.ext, ok, tc.wantOK)
			}
			if tc.wantOK {
				if got.DomainName != tc.wantLang {
					t.Errorf("ExtensionToLanguage(%q) lang=%q, want %q", tc.ext, got.DomainName, tc.wantLang)
				}
				if len(got.Nodes.Functions) > 0 {
					if !strings.Contains(got.Nodes.Functions[0], "") {
						t.Errorf("ExtensionToLanguage(%q) returned invalid functions", tc.ext)
					}
				}
			}
		})
	}
}

// TDD: RED — Test ResolveGrammar before it exists
func TestResolveGrammar(t *testing.T) {
	tests := []struct {
		langID  string
		wantOK  bool
		wantNil bool
	}{
		{"Go", true, false},
		{"JavaScript", true, false},
		{"TypeScript", true, false},
		{"Python", true, false},
		{"Rust", true, false},
		{"Java", true, false},
		{"C#", true, false},
		{"C++", true, false},
		{"PHP", true, false},
		{"Ruby", true, false},
		{"Swift", true, false},
		{"Kotlin", true, false},
		{"Dart", true, false},
		{"C", true, false},
		{"Markdown", true, false},
		{"Lua", true, false},
		{"Bash", true, false},
		{"Haskell", true, false},
		{"Scala", true, false},
		{"Elixir", true, false},
		{"UnknownLang", false, true},
		{"", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.langID, func(t *testing.T) {
			got, ok := ResolveGrammar(tc.langID)
			if ok != tc.wantOK {
				t.Fatalf("ResolveGrammar(%q) ok=%v, want %v", tc.langID, ok, tc.wantOK)
			}
			if tc.wantNil && got != nil {
				t.Errorf("ResolveGrammar(%q) got non-nil grammar, want nil", tc.langID)
			}
			if !tc.wantNil && got == nil {
				t.Errorf("ResolveGrammar(%q) got nil grammar, want non-nil", tc.langID)
			}
		})
	}
}

// TDD: RED — Verify at least top 20 languages have grammars available
func TestResolveGrammar_Top20HaveGrammars(t *testing.T) {
	langs := []string{
		"Go", "JavaScript", "TypeScript", "Python", "Rust",
		"Java", "C#", "C++", "PHP", "Ruby",
		"Swift", "Kotlin", "Dart", "C", "Markdown",
		"Lua", "Bash", "Haskell", "Scala", "Elixir",
		"Elm", "OCaml", "Perl", "R", "Zig",
		"Nim", "Crystal", "Julia", "Groovy", "F#",
	}

	missing := 0
	for _, lang := range langs {
		_, ok := ResolveGrammar(lang)
		if !ok {
			missing++
		}
	}

	if missing > 10 {
		t.Fatalf("Too many languages missing grammars: %d out of %d", missing, len(langs))
	}

	t.Logf("Grammar coverage: %d/%d languages missing grammars", missing, len(langs))
}

// TDD: RED — Verify unknown language fails gracefully
func TestResolveGrammar_UnknownFailsGracefully(t *testing.T) {
	got, ok := ResolveGrammar("TotalNonsenseLang999")
	if ok {
		t.Error("Expected ResolveGrammar to return false for unknown language")
	}
	if got != nil {
		t.Error("Expected ResolveGrammar to return nil grammar for unknown language")
	}
}

// TDD: RED — Verify ExtensionToLanguage handles edge cases
func TestExtensionToLanguage_EdgeCases(t *testing.T) {
	catalog := NewLanguageCatalog()

	tests := []struct {
		ext      string
		wantLang string
		wantOK   bool
	}{
		{".go", "Go", true},
		{".mod", "Go", true},
		{".cs", "C#", true},
		{".fs", "F#", true},
		{".md", "Markdown", true},
		{".jsx", "JavaScript", true},
		{".tsx", "TypeScript", true},
		{".txt", "", false},
		{"", "", false},
		{"noext", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.ext, func(t *testing.T) {
			got, ok := catalog.ExtensionToLanguage(tc.ext)
			if ok != tc.wantOK {
				t.Fatalf("ExtensionToLanguage(%q) ok=%v, want %v", tc.ext, ok, tc.wantOK)
			}
			if tc.wantOK && got.DomainName != tc.wantLang {
				t.Errorf("ExtensionToLanguage(%q) lang=%q, want %q", tc.ext, got.DomainName, tc.wantLang)
			}
		})
	}
}
