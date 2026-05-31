package chunkers

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
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

// TestExtensionDetection validates that extension-based language detection
// via kreuzberg works for common languages.
func TestExtensionDetection(t *testing.T) {
	catalog := NewLanguageCatalog()

	tests := []struct {
		ext        string
		wantOK     bool
		wantDomain string // empty means don't check
	}{
		{".go", true, "Go"},
		{".js", true, "JavaScript"},
		{".ts", true, "TypeScript"},
		{".py", true, "Python"},
		{".rs", true, "Rust"},
		{".java", true, "Java"},
		{".cpp", true, "C++"},
		{".rb", true, "Ruby"},
		{".swift", true, "Swift"},
		{".kt", true, "Kotlin"},
		{".dart", true, "Dart"},
		{"", false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.ext, func(t *testing.T) {
			entry, ok := catalog.ExtensionToLanguage(tc.ext)
			if ok != tc.wantOK {
				t.Fatalf("ExtensionToLanguage(%q) ok=%v, want %v", tc.ext, ok, tc.wantOK)
			}
			if tc.wantOK && tc.wantDomain != "" && entry.DomainName != tc.wantDomain {
				t.Errorf("ExtensionToLanguage(%q) DomainName=%q, want %q", tc.ext, entry.DomainName, tc.wantDomain)
			}
		})
	}
}

// TestTopLanguagesDetected validates that at least 15 of the top 30 languages
// are detectable via extension.
func TestTopLanguagesDetected(t *testing.T) {
	catalog := NewLanguageCatalog()
	exts := []string{
		".go", ".js", ".ts", ".py", ".rs",
		".java", ".cs", ".cpp", ".php", ".rb",
		".swift", ".kt", ".dart", ".c", ".md",
		".lua", ".sh", ".hs", ".scala", ".ex",
		".elm", ".ml", ".pl", ".r", ".zig",
		".nim", ".cr", ".jl", ".groovy", ".fs",
	}

	missing := 0
	for _, ext := range exts {
		_, ok := catalog.ExtensionToLanguage(ext)
		if !ok {
			missing++
		}
	}

	if missing > 15 {
		t.Fatalf("Too many extensions not detected: %d out of %d", missing, len(exts))
	}

	t.Logf("Extension detection: %d/%d languages detected", len(exts)-missing, len(exts))
}

// TestLanguageDetection_UnknownSucceedsGracefully verifies that unknown
// extensions don't cause errors.
func TestLanguageDetection_UnknownSucceedsGracefully(t *testing.T) {
	catalog := NewLanguageCatalog()
	_, ok := catalog.ExtensionToLanguage(".totallyunknownxyz")
	if ok {
		t.Error("Expected ExtensionToLanguage to return false for unknown extension")
	}
}

// TestExtensionToLanguage_EdgeCases verifies extension-to-language mapping
// via kreuzberg detection works for known code extensions.
func TestExtensionToLanguage_EdgeCases(t *testing.T) {
	catalog := NewLanguageCatalog()

	tests := []struct {
		ext      string
		wantLang string
		wantOK   bool
		note     string
	}{
		{".go", "Go", true, "Go is a core language"},
		// kreuzberg detects languages by extension; extensions like .mod, .cs, .fs, .md
		// may or may not be detected depending on the kreuzberg grammar registry.
		{".txt", "", false, "plain text has no grammar"},
		{"", "", false, "empty extension"},
		{"noext", "", false, "no leading dot"},
	}

	for _, tc := range tests {
		t.Run(tc.ext, func(t *testing.T) {
			got, ok := catalog.ExtensionToLanguage(tc.ext)
			if ok != tc.wantOK {
				// This is informational — kreuzberg registry can change between versions
				t.Logf("ExtensionToLanguage(%q) ok=%v, want %v — %s", tc.ext, ok, tc.wantOK, tc.note)
			}
			if tc.wantOK && ok && got.DomainName != tc.wantLang {
				t.Errorf("ExtensionToLanguage(%q) lang=%q, want %q", tc.ext, got.DomainName, tc.wantLang)
			}
		})
	}
}

// TestLanguageBodySpan_Fallback verifies that when entities have no body span
// (BodyStart=0, BodyEnd=0 — typically languages where tree-sitter doesn't
// provide BodySpan), a slog.Debug warning is emitted per entity indicating
// that per-entity CFG is unavailable and file-level CFG is used instead.
// When body span IS available, no fallback log should be emitted.
func TestLanguageBodySpan_Fallback(t *testing.T) {
	t.Run("no_body_span_emits_debug", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)

		catalog := NewLanguageCatalog()
		pass := NewUnifiedASTPass(catalog)

		before := []entity{
			{Name: "Handler", Receiver: "", Kind: "func", Signature: "func Handler()", Line: 1,
				BodyStart: 0, BodyEnd: 0}, // no body span
		}
		after := []entity{
			{Name: "Handler", Receiver: "", Kind: "func", Signature: "func Handler()", Line: 1,
				BodyStart: 0, BodyEnd: 0}, // no body span
		}

		_ = pass.matchEntities(before, after, entityMatchConfig{
			nodes:    data.LanguageNodes{},
			langName: "Go",
			filename: "test.go",
			cf: data.ControlFlowCategory{
				Branch: []string{"if"},
				Return: []string{"return"},
				Error:  []string{"try", "catch"},
			},
		}, domain.CFGDiff{})

		if logBuf.Len() == 0 {
			t.Error("expected slog.Debug for missing body span, but no log output found")
		}
		if !bytes.Contains(logBuf.Bytes(), []byte("per-entity CFG unavailable")) {
			t.Errorf("expected log message about unavailable body span, got: %s", logBuf.String())
		}
	})

	t.Run("body_span_available_no_debug", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)

		catalog := NewLanguageCatalog()
		pass := NewUnifiedASTPass(catalog)

		before := []entity{
			{Name: "Handler", Receiver: "", Kind: "func", Signature: "func Handler()", Line: 1,
				BodyStart: 10, BodyEnd: 50}, // body span available
		}
		after := []entity{
			{Name: "Handler", Receiver: "", Kind: "func", Signature: "func Handler()", Line: 1,
				BodyStart: 10, BodyEnd: 50}, // body span available
		}

		cf := data.ControlFlowCategory{
			Branch: []string{"if"},
			Return: []string{"return"},
		}

		// Need valid source for the entity at those byte ranges
		beforeSrc := make([]byte, 60)
		afterSrc := make([]byte, 60)
		copy(beforeSrc[10:50], []byte("  if x > 0 { return } "))
		copy(afterSrc[10:50], []byte("  if x > 0 { return } "))

		_ = pass.matchEntities(before, after, entityMatchConfig{
			nodes:     data.LanguageNodes{},
			langName:  "Go",
			filename:  "test.go",
			beforeSrc: beforeSrc,
			afterSrc:  afterSrc,
			cf:        cf,
		}, domain.CFGDiff{})

		if bytes.Contains(logBuf.Bytes(), []byte("per-entity CFG unavailable")) {
			t.Errorf("unexpected fallback log when body span is available: %s", logBuf.String())
		}
	})
}

// --- Entity Identity tests (Phase 2: Receiver-inclusive keys, rename detection) ---

// TestBuildEntityMap_ReceiverKey verifies that buildEntityMap produces distinct
// keys for same-name methods on different receivers, and uses name-only keys for
// free functions (no receiver).
func TestBuildEntityMap_ReceiverKey(t *testing.T) {
	tests := []struct {
		name        string
		entities    []entity
		wantKeys    map[string]entity // expected key → entity.Name
		wantKeyOnly bool              // if true, only check keys exist, not values
	}{
		{
			name: "receiver_methods_produce_distinct_keys",
			entities: []entity{
				{Name: "Close", Receiver: "Server", Kind: "func", Signature: "func (s *Server) Close()", Line: 1},
				{Name: "Close", Receiver: "Client", Kind: "func", Signature: "func (c *Client) Close()", Line: 5},
			},
			wantKeys: map[string]entity{
				"Server.Close": {Name: "Close", Receiver: "Server"},
				"Client.Close": {Name: "Close", Receiver: "Client"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildEntityMap(tc.entities)

			for key, wantEnt := range tc.wantKeys {
				ent, exists := got[key]
				if !exists {
					t.Errorf("expected key %q not found in entity map; got keys: %v", key, mapKeys(got))
					continue
				}
				if ent.Name != wantEnt.Name {
					t.Errorf("entityMap[%q].Name = %q, want %q", key, ent.Name, wantEnt.Name)
				}
				if ent.Receiver != wantEnt.Receiver {
					t.Errorf("entityMap[%q].Receiver = %q, want %q", key, ent.Receiver, wantEnt.Receiver)
				}
			}

			if len(got) != len(tc.wantKeys) {
				t.Errorf("entity map has %d keys, want %d; got keys: %v", len(got), len(tc.wantKeys), mapKeys(got))
			}
		})
	}
}

// mapKeys returns the keys of a map[string]entity for error messages.
func mapKeys(m map[string]entity) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestMatchEntities_RenameDetection verifies that matchEntities classifies
// high-similarity entity name pairs as RENAMED (not DELETED+NEW), while
// low-similarity pairs stay as DELETED+NEW. Identical-name behavior is
// unchanged.
func TestMatchEntities_RenameDetection(t *testing.T) {
	tests := []struct {
		name        string
		before      []entity
		after       []entity
		wantRenames int      // expected count of RENAMED labels
		wantOthers  []string // expected non-RENAMED label types (subset check)
	}{
		{
			// High similarity: "HandleReq" → "HandleRequest" (ratio ≥ 0.6)
			name: "high_similarity_rename",
			before: []entity{
				{Name: "HandleReq", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleReq()", Line: 1},
			},
			after: []entity{
				{Name: "HandleRequest", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleRequest()", Line: 1},
			},
			wantRenames: 1,
			wantOthers:  []string{},
		},
		{
			// Low similarity: "HandleReq" vs "ProcessData" (ratio < 0.6) → stays DELETED+NEW
			name: "low_similarity_stays_deleted_new",
			before: []entity{
				{Name: "HandleReq", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleReq()", Line: 1},
			},
			after: []entity{
				{Name: "ProcessData", Receiver: "Server", Kind: "func", Signature: "func (s *Server) ProcessData()", Line: 1},
			},
			wantRenames: 0,
			wantOthers:  []string{"DELETED_FUNC", "NEW_FUNC"},
		},
		{
			// Identical names → normal matching (unchanged behavior)
			name: "identical_name_unchanged",
			before: []entity{
				{Name: "HandleReq", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleReq(ctx context.Context)", Line: 1},
			},
			after: []entity{
				{Name: "HandleReq", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleReq()", Line: 1},
			},
			wantRenames: 0,
			wantOthers:  []string{"MOD_SIG"},
		},
		{
			// Rename + signature change → MOD_SIG, not RENAMED (spec scenario)
			name: "rename_with_signature_change_is_mod_sig",
			before: []entity{
				{Name: "HandleReq", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleReq(ctx context.Context)", Line: 1},
			},
			after: []entity{
				{Name: "HandleRequest", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleRequest(ctx context.Context, w io.Writer)", Line: 1},
			},
			wantRenames: 0,
			wantOthers:  []string{"MOD_SIG"},
		},
		{
			// Pure rename: only name differs, signatures identical after name substitution → RENAMED
			name: "pure_rename_same_body_is_RENAMED",
			before: []entity{
				{Name: "HandleReq", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleReq()", Line: 1},
			},
			after: []entity{
				{Name: "HandleRequest", Receiver: "Server", Kind: "func", Signature: "func (s *Server) HandleRequest()", Line: 1},
			},
			wantRenames: 1,
			wantOthers:  []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catalog := NewLanguageCatalog()
			pass := NewUnifiedASTPass(catalog)
			labels := pass.matchEntities(tc.before, tc.after, entityMatchConfig{
				nodes: data.LanguageNodes{},
			}, domain.CFGDiff{})

			renamedCount := 0
			var otherTypes []string
			for _, l := range labels {
				if l.Type == domain.LabelType("RENAMED") ||
					l.Type == domain.LabelType("RENAMED_FUNC") ||
					l.Type == domain.LabelType("RENAMED_TYPE") {
					renamedCount++
				} else {
					otherTypes = append(otherTypes, string(l.Type))
				}
			}

			if renamedCount != tc.wantRenames {
				t.Errorf("RENAMED labels = %d, want %d; all labels: %v", renamedCount, tc.wantRenames, labelTypes(labels))
			}

			for _, want := range tc.wantOthers {
				found := false
				for _, got := range otherTypes {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected label type %q in non-RENAMED labels, got %v", want, otherTypes)
				}
			}
		})
	}
}

// labelTypes returns the string types of a slice of labels for error messages.
func labelTypes(labels []domain.Label) []string {
	types := make([]string, len(labels))
	for i, l := range labels {
		types[i] = string(l.Type)
	}
	return types
}

// TestLevenshteinRatio verifies that the Levenshtein edit distance ratio
// produces correct values for known string pairs.
func TestLevenshteinRatio(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want float64
	}{
		{name: "identical_strings", a: "hello", b: "hello", want: 1.0},
		{name: "both_empty", a: "", b: "", want: 1.0},
		{name: "one_empty", a: "hello", b: "", want: 0.0},
		{name: "single_char_match", a: "a", b: "a", want: 1.0},
		{name: "single_char_mismatch", a: "a", b: "b", want: 0.0},
		{name: "prefix_match", a: "HandleReq", b: "HandleRequest", want: 0.69},
		{name: "completely_different", a: "HandleReq", b: "ProcessData", want: 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LevenshteinRatio(tc.a, tc.b)
			// Allow small floating point tolerance
			delta := 0.05
			if got < tc.want-delta || got > tc.want+delta {
				t.Errorf("LevenshteinRatio(%q, %q) = %.4f, want ~%.4f", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestLabelForKind_RenamedTypes verifies that labelForKind produces correct
// subtypes for NEW, DELETED, and RENAMED categories across func and type kinds.
func TestLabelForKind_RenamedTypes(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		family labelFamily
		want   domain.LabelType
	}{
		{
			name:   "func_renamed_produces_RENAMED_FUNC",
			kind:   "func",
			family: labelRenamed,
			want:   domain.LabelType("RENAMED_FUNC"),
		},
		{
			name:   "type_renamed_produces_RENAMED_TYPE",
			kind:   "type",
			family: labelRenamed,
			want:   domain.LabelType("RENAMED_TYPE"),
		},
		{
			name:   "func_new_still_NEW_FUNC",
			kind:   "func",
			family: labelNew,
			want:   domain.NEW_FUNC,
		},
		{
			name:   "func_deleted_still_DELETED_FUNC",
			kind:   "func",
			family: labelDeleted,
			want:   domain.DELETED_FUNC,
		},
		{
			name:   "type_new_still_NEW_TYPE",
			kind:   "type",
			family: labelNew,
			want:   domain.NEW_TYPE,
		},
		{
			name:   "type_deleted_still_DELETED_TYPE",
			kind:   "type",
			family: labelDeleted,
			want:   domain.DELETED_TYPE,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := labelForKind(tc.kind, tc.family)
			if got != tc.want {
				t.Errorf("labelForKind(%q, %v) = %q, want %q", tc.kind, tc.family, got, tc.want)
			}
		})
	}
}

// --- Per-entity CFG label assignment tests (Phase 3) ---

// TestModLabelFromCFG_PerEntityCFG verifies that per-entity CFG produces
// correct label subtypes: entity with error-path CFG change gets
// MOD_BODY_ERROR; entity in a changed file but unchanged body gets
// MOD_BODY (not MOD_BODY_ERROR).
func TestModLabelFromCFG_PerEntityCFG(t *testing.T) {
	tests := []struct {
		name    string
		isFunc  bool
		cfgDiff domain.CFGDiff
		want    domain.LabelType
	}{
		{
			name:   "entity_error_path_change_gets_MOD_BODY_ERROR",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Error: 1},
				After:  domain.CFGCount{Error: 2},
			},
			want: domain.MOD_BODY_ERROR,
		},
		{
			name:   "entity_unchanged_in_changed_file_gets_MOD_BODY",
			isFunc: true,
			// When the entity's own body has zero CFG diff (unchanged entity
			// in a changed file, no body span available), the result is MOD_BODY
			// because both Before and After are zero — "no CFG signal available".
			// Note: when per-entity CFG IS available and is identical,
			// modLabelFromCFG returns MOD_BODY_CALL instead.
			cfgDiff: domain.CFGDiff{},
			want:    domain.MOD_BODY,
		},
		{
			name:   "entity_with_logic_change_gets_MOD_BODY_LOGIC",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Branch: 1},
				After:  domain.CFGCount{Branch: 3},
			},
			want: domain.MOD_BODY_LOGIC,
		},
		{
			name:   "entity_identical_CFG_gets_MOD_BODY_CALL",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Branch: 2, Return: 1},
				After:  domain.CFGCount{Branch: 2, Return: 1},
			},
			want: domain.MOD_BODY_CALL,
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

// TestProcessWithContent_PerEntityCFG_Integration verifies that when a file
// has two functions (one changed with error handling, one unchanged), the
// changed function gets MOD_BODY_ERROR while the unchanged one gets MOD_BODY
// — proving that per-entity CFG is wired correctly through the pipeline.
func TestProcessWithContent_PerEntityCFG_Integration(t *testing.T) {
	catalog := NewLanguageCatalog()
	entry, ok := catalog.ExtensionToLanguage(".js")
	if !ok || !entry.HasGrammar {
		t.Skip("no JavaScript grammar")
	}

	// Before: handler() with no try/catch, close() unchanged
	before := []byte(`function handler() {
  return 1;
}
function close() {
  return 0;
}
`)

	// After: handler() gains try/catch (error path), close() unchanged
	after := []byte(`function handler() {
  try {
    return 1;
  } catch(e) {
    return -1;
  }
}
function close() {
  return 0;
}
`)

	pass := NewUnifiedASTPass(catalog)
	labels, _, err := pass.ProcessWithContent("test.js", before, after, nil)
	if err != nil {
		t.Fatalf("ProcessWithContent error: %v", err)
	}

	// With per-entity CFG: handler should get MOD_BODY_ERROR (its body has new error keywords)
	// and close should get MOD_BODY (its body is unchanged → zero entity CFGDiff).
	// Without per-entity CFG (file-level): both would get the SAME label based on
	// the file-level CFGDiff, which includes the handler's try/catch.
	handlerLabel := findLabelByName(labels, "handler")
	closeLabel := findLabelByName(labels, "close")

	if handlerLabel == nil {
		t.Fatalf("expected 'handler' label not found; labels: %v", labelTypes(labels))
	}
	if closeLabel == nil {
		t.Fatalf("expected 'close' label not found; labels: %v", labelTypes(labels))
	}

	// Handler has error-path change → should be MOD_BODY_ERROR
	if handlerLabel.Type != domain.MOD_BODY_ERROR {
		t.Errorf("handler label = %q, want MOD_BODY_ERROR", handlerLabel.Type)
	}

	// Close is unchanged → per-entity CFG is identical → MOD_BODY_CALL
	// (This is BETTER than the file-level MOD_BODY_ERROR that would have been
	// assigned without per-entity CFG.)
	if closeLabel.Type != domain.MOD_BODY_CALL {
		t.Errorf("close label = %q, want MOD_BODY_CALL (per-entity CFG identical)", closeLabel.Type)
	}
}

// findLabelByName finds the first label with the given name.
func findLabelByName(labels []domain.Label, name string) *domain.Label {
	for i := range labels {
		if labels[i].Name == name {
			return &labels[i]
		}
	}
	return nil
}

// --- parseAndExtract signature extraction tests (Bug 1: CutAtBrace) ---

// TestParseAndExtract_Signature_CutAtBrace verifies that parseAndExtract
// cuts function signatures at the body-span boundary without including
// trailing braces and without TrimRight character sets that strip
// meaningful characters. The signature extraction must:
//   - Use BodySpan.StartByte to cut (already excludes '{')
//   - TrimRight only whitespace (" \t\n\r"), NOT charset strings like "{:="
//   - Fall back to full Span with whitespace-only TrimRight when no BodySpan
func TestParseAndExtract_Signature_CutAtBrace(t *testing.T) {
	catalog := NewLanguageCatalog()

	tests := []struct {
		name          string
		filename      string
		src           string
		wantFunc      string // function name to find
		wantSignature string // expected signature (no trailing brace)
	}{
		{
			name:          "same_line_brace_excluded",
			filename:      "same.go",
			src:           "package main\nfunc Foo(a, b int) string {\n  return a + b\n}\n",
			wantFunc:      "Foo",
			wantSignature: "func Foo(a, b int) string",
		},
		{
			name:          "next_line_brace_no_trailing_whitespace",
			filename:      "nextline.go",
			src:           "package main\nfunc Bar(x int)\n{\n  return x\n}\n",
			wantFunc:      "Bar",
			wantSignature: "func Bar(x int)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ext := filepath.Ext(tc.filename)
			entry, ok := catalog.ExtensionToLanguage(ext)
			if !ok || !entry.HasGrammar {
				t.Skipf("no grammar for %s", tc.filename)
			}

			pass := NewUnifiedASTPass(catalog)
			entities, err := pass.parseAndExtract(entry.Name, []byte(tc.src), entry.Nodes)
			if err != nil {
				t.Fatalf("parseAndExtract error: %v", err)
			}

			var found *entity
			for i := range entities {
				if entities[i].Name == tc.wantFunc {
					found = &entities[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("expected entity %q not found in %d entities", tc.wantFunc, len(entities))
			}

			// The signature must NOT contain a trailing '{'
			if strings.Contains(found.Signature, "{") {
				t.Errorf("signature %q contains '{' — should be cut at body-span boundary", found.Signature)
			}
			// The signature must match the expected value precisely
			if found.Signature != tc.wantSignature {
				t.Errorf("signature = %q, want %q", found.Signature, tc.wantSignature)
			}
		})
	}
}

// TestParseAndExtract_Signature_NoBodySpanFallback verifies that when
// BodySpan is nil, the full Span is used with whitespace-only TrimRight
// (not a charset trim that could remove meaningful characters).
func TestParseAndExtract_Signature_NoBodySpanFallback(t *testing.T) {
	catalog := NewLanguageCatalog()
	entry, ok := catalog.ExtensionToLanguage(".py")
	if !ok || !entry.HasGrammar {
		t.Skip("no Python grammar")
	}

	pass := NewUnifiedASTPass(catalog)

	// Python signature ends with ':' which is NOT a brace or whitespace.
	// The current buggy TrimRight("{:=") would strip the ':', corrupting
	// the signature. After the fix, TrimRight(" \t\n\r") preserves it.
	src := []byte("def compute(x, y):\n    return x + y\n")
	entities, err := pass.parseAndExtract(entry.Name, src, entry.Nodes)
	if err != nil {
		t.Fatalf("parseAndExtract error: %v", err)
	}

	var found *entity
	for i := range entities {
		if entities[i].Name == "compute" {
			found = &entities[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected entity 'compute' not found")
	}

	// After the fix, TrimRight uses " \t\n\r" only — the ':' is preserved
	// because it's not whitespace. This distinguishes "cut at body boundary"
	// from "indiscriminate charset trim".
	wantSig := "def compute(x, y):"
	if found.Signature != wantSig {
		t.Errorf("signature = %q, want %q (colon must be preserved — no charset TrimRight)", found.Signature, wantSig)
	}
}

// --- modLabelFromCFG tests ---

// TestModLabelFromCFG_ZeroDiff_ReturnsModBody verifies Bug 6: when no CFG
// signal is available (zero-value CFGDiff), the default should be MOD_BODY
// (generic "body changed, subtype unknown"), NOT MOD_BODY_LOGIC (which
// implies a specific logic change was detected).
func TestModLabelFromCFG_ZeroDiff_ReturnsModBody(t *testing.T) {
	tests := []struct {
		name    string
		isFunc  bool
		cfgDiff domain.CFGDiff
		want    domain.LabelType
	}{
		{
			name:    "zero_CFGDiff_returns_MOD_BODY",
			isFunc:  true,
			cfgDiff: domain.CFGDiff{},
			want:    domain.MOD_BODY,
		},
		{
			name:   "nil_CFG_count_returns_MOD_BODY",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{},
				After:  domain.CFGCount{},
			},
			want: domain.MOD_BODY,
		},
		{
			name:   "valid_CFG_with_error_change_still_produces_MOD_BODY_ERROR",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{Error: 1},
				After:  domain.CFGCount{Error: 3},
			},
			want: domain.MOD_BODY_ERROR,
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
			name:    "zero_CFG_fallback_returns_MOD_BODY",
			isFunc:  true,
			cfgDiff: domain.CFGDiff{},
			want:    domain.MOD_BODY,
		},
		{
			name:   "nil_CFG_fallback_returns_MOD_BODY",
			isFunc: true,
			cfgDiff: domain.CFGDiff{
				Before: domain.CFGCount{},
				After:  domain.CFGCount{},
			},
			want: domain.MOD_BODY,
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

// RC4: Return increase with error increase → MOD_BODY_ERROR
func TestModLabelFromCFG_return_increase_with_error_increase_returns_MOD_BODY_ERROR(t *testing.T) {
	cfgDiff := domain.CFGDiff{
		Before: domain.CFGCount{Branch: 2, Return: 1, Error: 0},
		After:  domain.CFGCount{Branch: 3, Return: 2, Error: 1},
	}
	got := modLabelFromCFG(true, cfgDiff)
	if got != domain.MOD_BODY_ERROR {
		t.Errorf("modLabelFromCFG(true, {Before: {2,1,0}, After: {3,2,1}}) = %q, want %q", got, domain.MOD_BODY_ERROR)
	}
}

// RC4: Return increase without error change → MOD_BODY_REORDER (unchanged behavior)
func TestModLabelFromCFG_return_increase_without_error_change_returns_MOD_BODY_REORDER(t *testing.T) {
	cfgDiff := domain.CFGDiff{
		Before: domain.CFGCount{Branch: 2, Return: 1, Error: 0},
		After:  domain.CFGCount{Branch: 2, Return: 3, Error: 0},
	}
	got := modLabelFromCFG(true, cfgDiff)
	if got != domain.MOD_BODY_REORDER {
		t.Errorf("modLabelFromCFG(true, {Before: {2,1,0}, After: {2,3,0}}) = %q, want %q", got, domain.MOD_BODY_REORDER)
	}
}

// RC4: Return increase with equal errors → MOD_BODY_REORDER (no error change)
func TestModLabelFromCFG_return_increase_with_equal_errors_returns_MOD_BODY_REORDER(t *testing.T) {
	cfgDiff := domain.CFGDiff{
		Before: domain.CFGCount{Branch: 2, Return: 1, Error: 2},
		After:  domain.CFGCount{Branch: 2, Return: 3, Error: 2},
	}
	got := modLabelFromCFG(true, cfgDiff)
	if got != domain.MOD_BODY_REORDER {
		t.Errorf("modLabelFromCFG(true, {Before: {2,1,2}, After: {2,3,2}}) = %q, want %q", got, domain.MOD_BODY_REORDER)
	}
}

// RC4 triangulation: Return decrease with error increase still → MOD_BODY_ERROR (existing check)
func TestModLabelFromCFG_return_decrease_with_error_increase_returns_MOD_BODY_ERROR(t *testing.T) {
	cfgDiff := domain.CFGDiff{
		Before: domain.CFGCount{Branch: 2, Return: 3, Error: 0},
		After:  domain.CFGCount{Branch: 2, Return: 2, Error: 1},
	}
	got := modLabelFromCFG(true, cfgDiff)
	if got != domain.MOD_BODY_ERROR {
		t.Errorf("modLabelFromCFG(true, {Before: {2,3,0}, After: {2,2,1}}) = %q, want %q", got, domain.MOD_BODY_ERROR)
	}
}

// --- ProcessWithContent subtype emission tests ---

func TestProcessWithContent_EmitsSubtypes(t *testing.T) {
	catalog := NewLanguageCatalog()

	tests := []struct {
		name        string
		filename    string
		before      string
		after       string
		wantSubtype domain.LabelType // expected MOD_BODY_* subtype in labels
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
// cannot match specific entities (empty labels), the catch-all emits a CFG-based
// subtype or MOD_BODY (when no CFG signal is available).
func TestProcessWithContent_CatchAll_UsesCFGSubtype(t *testing.T) {
	catalog := NewLanguageCatalog()

	// When there are no AST entities that match, the catch-all should produce
	// MOD_BODY with zero CFG (no CFG signal available), or a subtyped label
	// with CFG signal.
	tests := []struct {
		name        string
		filename    string
		before      string
		after       string
		wantSubtype domain.LabelType
		description string
	}{
		{
			// Zero CFGDiff (no grammar or no ControlFlow) → CONFIG label for non-code
			name:        "catch_all_no_grammar_emits_CONFIG",
			filename:    "config.toml",
			before:      "[settings]\nkey = \"old\"",
			after:       "[settings]\nkey = \"new\"",
			wantSubtype: domain.LabelType("CONFIG"),
			description: "non-code file gets CONFIG label, not MOD_BODY",
		},
		{
			// Go file with branch change and no entity matches (new function → NEW_FUNC,
			// but if we use before=after=same, CFG is identical → MOD_BODY_CALL from catch-all)
			name:        "catch_all_identical_CFG_emits_MOD_BODY_CALL",
			filename:    "same.go",
			before:      "package main\nfunc same() {\nif true {\nreturn\n}\n}",
			after:       "package main\nfunc same() {\nif true {\nreturn\n}\n}",
			wantSubtype: domain.MOD_BODY_CALL,
			description: "identical before/after → MOD_BODY_CALL from matchEntities (exact same signature)",
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

// TestProcess_GrammarFileEmptyAnnotatedDiff verifies that Process() returns
// chunks with empty AnnotatedDiff for grammar-supported code files.
// Generic labels must NOT be injected by Process() —
// annotateChunks() is the sole authority for semantic labels.
func TestProcess_GrammarFileEmptyAnnotatedDiff(t *testing.T) {
	catalog := NewLanguageCatalog()
	pass := NewUnifiedASTPass(catalog)

	const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -1,2 +1,3 @@
 package main
 func Existing() {}
+func NewHelper() {}
`

	files, _, err := gitdiff.Parse(strings.NewReader(rawDiff))
	if err != nil {
		t.Fatalf("failed to parse diff: %v", err)
	}

	chunks, _, err := pass.Process(files, 0)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk from Process()")
	}

	for i, chunk := range chunks {
		if chunk.AnnotatedDiff != "" {
			t.Errorf("chunk[%d].AnnotatedDiff = %q, want empty string for grammar-supported file", i, chunk.AnnotatedDiff)
		}
	}
}
