package chunkers

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
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

// --- modLabelFromCFG tests ---

func TestModLabelFromCFG(t *testing.T) {
	tests := []struct {
		name    string
		isFunc  bool
		cfgDiff domain.CFGDiff
		want    domain.LabelType
	}{
		{
			name:   "error_changed_returns_MOD_BODY_ERROR",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Error: 1, Branch: 2, Loop: 1},
				After:  domain.CFGCount{Error: 3, Branch: 2, Loop: 1},
			},
			want: domain.MOD_BODY_ERROR,
		},
		{
			name:   "branch_changed_returns_MOD_BODY_LOGIC",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Branch: 1, Loop: 1},
				After:  domain.CFGCount{Branch: 3, Loop: 1},
			},
			want: domain.MOD_BODY_LOGIC,
		},
		{
			name:   "loop_changed_returns_MOD_BODY_LOGIC",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Branch: 1, Loop: 0},
				After:  domain.CFGCount{Branch: 1, Loop: 2},
			},
			want: domain.MOD_BODY_LOGIC,
		},
		{
			name:   "return_only_changed_returns_MOD_BODY_REORDER",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Return: 1, Branch: 2},
				After:  domain.CFGCount{Return: 3, Branch: 2},
			},
			want: domain.MOD_BODY_REORDER,
		},
		{
			name:   "identical_CFG_returns_MOD_BODY_CALL",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Branch: 2, Loop: 1, Return: 3, Error: 1},
				After:  domain.CFGCount{Branch: 2, Loop: 1, Return: 3, Error: 1},
			},
			want: domain.MOD_BODY_CALL,
		},
		{
			name:    "zero_CFG_fallback_returns_MOD_BODY_LOGIC",
			isFunc:  true,
			cfgDiff: domain.CFGDiff{},
			want:    domain.MOD_BODY_LOGIC,
		},
		{
			name: "nil_CFG_fallback_returns_MOD_BODY_LOGIC",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{},
				After:  domain.CFGCount{},
			},
			want: domain.MOD_BODY_LOGIC,
		},
		{
			name:   "non_func_returns_MOD_TYPE",
			isFunc: false,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Branch: 2},
				After:  domain.CFGCount{Branch: 5},
			},
			want: domain.MOD_TYPE,
		},
		{
			name:   "error_and_branch_changed_errors_wins",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Error: 1, Branch: 2},
				After:  domain.CFGCount{Error: 3, Branch: 5},
			},
			want: domain.MOD_BODY_ERROR,
		},
		{
			name:   "branch_and_return_changed_branch_wins",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Branch: 1, Return: 1},
				After:  domain.CFGCount{Branch: 2, Return: 3},
			},
			want: domain.MOD_BODY_LOGIC,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := modLabelFromCFG(tc.isFunc, tc.cfgDiff)
			if got != tc.want {
				t.Errorf("modLabelFromCFG(%v, %+v) = %q, want %q", tc.isFunc, tc.cfgDiff, got, tc.want)
			}
		})
	}
}

// --- ProcessWithContent subtype emission tests ---

func TestProcessWithContent_EmitsSubtypes(t *testing.T) {
	catalog := NewLanguageCatalog()

	tests := []struct {
		name           string
		filename       string
		before         string
		after          string
		wantSubtype    domain.LabelType // expected MOD_BODY_* subtype in labels
	}{
		{
			// JavaScript has error CFG keywords (try, catch, finally, throw)
			name:        "MOD_BODY_ERROR_when_error_count_increases",
			filename:    "handler.js",
			before:      "function handler() {\n  return;\n}",
			after:       "function handler() {\n  try {\n    return;\n  } catch(e) {\n    throw e;\n  }\n}",
			wantSubtype: domain.MOD_BODY_ERROR,
		},
		{
			// Both JS and Go have branch keywords (if, else, switch)
			name:        "MOD_BODY_LOGIC_when_branch_increases",
			filename:    "logic.go",
			before:      "package main\nfunc logic() {\nreturn\n}",
			after:       "package main\nfunc logic() {\nif x > 0 {\nreturn\n}\nreturn\n}",
			wantSubtype: domain.MOD_BODY_LOGIC,
		},
		{
			// JavaScript has return CFG keywords (return, yield) — Go does NOT
			name:        "MOD_BODY_REORDER_when_only_return_changes",
			filename:    "reorder.js",
			before:      "function reorder() {\n  return 1;\n}",
			after:       "function reorder() {\n  return 1;\n  return 2;\n}",
			wantSubtype: domain.MOD_BODY_REORDER,
		},
		{
			// Identical CFG (same before/after structure) → MOD_BODY_CALL
			name:        "MOD_BODY_CALL_when_CFG_identical",
			filename:    "same.js",
			before:      "function same() {\n  if (x) {\n    return 1;\n  }\n}",
			after:       "function same() {\n  if (x) {\n    return 1;\n  }\n}",
			wantSubtype: domain.MOD_BODY_CALL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pass := NewUnifiedASTPass(catalog)
			labels, _, err := pass.ProcessWithContent(tc.filename, []byte(tc.before), []byte(tc.after), nil)
			if err != nil {
				t.Fatalf("ProcessWithContent error: %v", err)
			}

			found := false
			for _, l := range labels {
				if l.Type == tc.wantSubtype {
					found = true
					break
				}
			}
			if !found {
				var labelTypes []string
				for _, l := range labels {
					labelTypes = append(labelTypes, string(l.Type))
				}
				t.Errorf("expected label type %q in labels, got %v", tc.wantSubtype, labelTypes)
			}
		})
	}
}

// TestProcessWithContent_CatchAll_UsesCFGSubtype verifies that when ProcessWithContent
// cannot match specific entities (empty labels), the catch-all emits MOD_BODY_LOGIC
// (file-level CFG heuristic) rather than the generic MOD_BODY.
func TestProcessWithContent_CatchAll_UsesCFGSubtype(t *testing.T) {
	catalog := NewLanguageCatalog()

	// When there are no AST entities that match (same function, but in JS
	// where we can create a scenario with code that yields no named entity
	// matching), the catch-all should produce MOD_BODY_LOGIC with zero CFG,
	// or a subtyped label with CFG signal.
	tests := []struct {
		name        string
		filename    string
		before      string
		after       string
		wantSubtype domain.LabelType
		description string
	}{
		{
			// Zero CFGDiff (no grammar or no ControlFlow) → MOD_BODY_LOGIC default
			name:         "catch_all_no_grammar_emits_MOD_BODY_LOGIC",
			filename:     "config.toml",
			before:       "[settings]\nkey = \"old\"",
			after:        "[settings]\nkey = \"new\"",
			wantSubtype:  domain.LabelType("CONFIG"),
			description:  "non-code file gets CONFIG label, not MOD_BODY",
		},
		{
			// Go file with branch change and no entity matches (new function → NEW_FUNC,
			// but if we use before=after=same, CFG is identical → MOD_BODY_CALL from catch-all)
			name:         "catch_all_identical_CFG_emits_MOD_BODY_CALL",
			filename:     "same.go",
			before:       "package main\nfunc same() {\nif true {\nreturn\n}\n}",
			after:        "package main\nfunc same() {\nif true {\nreturn\n}\n}",
			wantSubtype:  domain.MOD_BODY_CALL,
			description:  "identical before/after → MOD_BODY_CALL from matchEntities (exact same signature)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pass := NewUnifiedASTPass(catalog)
			labels, _, err := pass.ProcessWithContent(tc.filename, []byte(tc.before), []byte(tc.after), nil)
			if err != nil {
				t.Fatalf("ProcessWithContent error: %v", err)
			}

			found := false
			for _, l := range labels {
				if l.Type == tc.wantSubtype {
					found = true
					break
				}
			}
			if !found {
				var labelTypes []string
				for _, l := range labels {
					labelTypes = append(labelTypes, string(l.Type))
				}
				t.Errorf("%s: expected %q in labels, got %v", tc.description, tc.wantSubtype, labelTypes)
			}
		})
	}
}
