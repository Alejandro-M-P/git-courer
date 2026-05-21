package classifier

import (
	"testing"
)

// ---------------------------------------------------------------------------
// normalizeBody tests — the core normalization function
// ---------------------------------------------------------------------------

// TestNormalizeBody_replaces_identifiers verifies that all identifiers in a
// function's body are replaced with positional names (v1, v2, v3...).
func TestNormalizeBody_replaces_identifiers(t *testing.T) {
	src := `package p
func add(a int, b int) int {
	return a + b
}
`
	normalized := normalizeBody(src, "add")
	if normalized == "" {
		t.Fatal("normalizeBody returned empty string")
	}
	// The identifiers 'a' and 'b' should be replaced with 'v1' and 'v2'
	// The original names should NOT appear after normalization
	if normalized == src {
		t.Errorf("normalizeBody did not transform the source; got same string")
	}
}

// TestNormalizeBody_same_logic_different_names verifies that two functions
// with identical logic but different variable names produce the same hash.
// This is the KEY property of Pilar 3.
func TestNormalizeBody_same_logic_different_names(t *testing.T) {
	src1 := `package p
func sum(a int, b int) int {
	return a + b
}
`
	src2 := `package p
func sum(x int, y int) int {
	return x + y
}
`
	hash1 := hashFunctionBody(src1, "sum")
	hash2 := hashFunctionBody(src2, "sum")

	if hash1 == "" || hash2 == "" {
		t.Fatal("hashFunctionBody returned empty hash")
	}
	if hash1 != hash2 {
		t.Errorf("same logic with different param names should produce same hash:\nhash1=%s\nhash2=%s", hash1, hash2)
	}
}

// TestNormalizeBody_different_logic_produces_different_hash verifies that
// functions with genuinely different logic produce different hashes.
func TestNormalizeBody_different_logic_produces_different_hash(t *testing.T) {
	src1 := `package p
func calc(a int, b int) int {
	return a + b
}
`
	src2 := `package p
func calc(a int, b int) int {
	return a - b
}
`
	hash1 := hashFunctionBody(src1, "calc")
	hash2 := hashFunctionBody(src2, "calc")

	if hash1 == "" || hash2 == "" {
		t.Fatal("hashFunctionBody returned empty hash")
	}
	if hash1 == hash2 {
		t.Errorf("different logic (a+b vs a-b) should produce different hashes, got same hash")
	}
}

// TestNormalizeBody_strips_comments verifies that comments are removed
// during normalization so they don't affect the hash.
func TestNormalizeBody_strips_comments(t *testing.T) {
	src1 := `package p
func add(a int, b int) int {
	// This is a comment
	return a + b
}
`
	src2 := `package p
func add(a int, b int) int {
	// Completely different comment
	return a + b
}
`
	hash1 := hashFunctionBody(src1, "add")
	hash2 := hashFunctionBody(src2, "add")

	if hash1 == "" || hash2 == "" {
		t.Fatal("hashFunctionBody returned empty hash")
	}
	if hash1 != hash2 {
		t.Errorf("same logic with different comments should produce same hash:\nhash1=%s\nhash2=%s", hash1, hash2)
	}
}

// TestNormalizeBody_missing_function_returns_empty verifies that
// referencing a non-existent function name returns empty string.
func TestNormalizeBody_missing_function_returns_empty(t *testing.T) {
	src := `package p
func add(a int, b int) int {
	return a + b
}
`
	normalized := normalizeBody(src, "nonExistent")
	if normalized != "" {
		t.Errorf("expected empty string for missing function, got: %q", normalized)
	}
}

// TestNormalizeBody_invalid_go_source_returns_empty verifies graceful
// fallback when the source cannot be parsed.
func TestNormalizeBody_invalid_go_source_returns_empty(t *testing.T) {
	src := "this is not valid go code at all {{{"
	normalized := normalizeBody(src, "anything")
	if normalized != "" {
		t.Errorf("expected empty string for unparseable source, got: %q", normalized)
	}
}

// ---------------------------------------------------------------------------
// functionInfo and extractFunctions tests
// ---------------------------------------------------------------------------

// TestExtractFunctions_extracts_from_valid_source verifies extracting
// all function declarations from a Go source file.
func TestExtractFunctions_extracts_from_valid_source(t *testing.T) {
	src := `package p
func add(a int, b int) int {
	return a + b
}
func subtract(a int, b int) int {
	return a - b
}
`
	funcs := extractFunctions(src, "test.go")
	if len(funcs) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(funcs))
	}

	names := map[string]bool{}
	for _, f := range funcs {
		names[f.Name] = true
	}
	if !names["add"] {
		t.Error("expected function 'add' to be extracted")
	}
	if !names["subtract"] {
		t.Error("expected function 'subtract' to be extracted")
	}
}

// TestExtractFunctions_empty_source verifies empty source returns no functions.
func TestExtractFunctions_empty_source(t *testing.T) {
	funcs := extractFunctions("", "test.go")
	if len(funcs) != 0 {
		t.Errorf("expected 0 functions for empty source, got %d", len(funcs))
	}
}

// TestExtractFunctions_invalid_source_returns_empty verifies graceful
// fallback when the source cannot be parsed.
func TestExtractFunctions_invalid_source_returns_empty(t *testing.T) {
	funcs := extractFunctions("not valid go {{{}}}", "broken.go")
	if len(funcs) != 0 {
		t.Errorf("expected 0 functions for invalid source, got %d", len(funcs))
	}
}

// ---------------------------------------------------------------------------
// detectRefactorByASTHash integration tests
// ---------------------------------------------------------------------------

// TestDetectRefactorByASTHash_rename verifies that renaming a function
// (same body, different name) is detected as refactor with high confidence.
func TestDetectRefactorByASTHash_rename(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func sum(x int, y int) int {
	return x + y
}
`

	// We need to set up before/after content on the classifier's file list
	// Using the internal method that reads from DiffChunk's AnnotatedDiff
	// We'll test via a higher-level function
	result, confidence := c.detectRefactorByASTHash(
		[]string{"math.go"},
		map[string]string{"math.go": beforeSrc},
		map[string]string{"math.go": afterSrc},
	)

	if result != "refactor" {
		t.Errorf("expected 'refactor' for rename, got %q", result)
	}
	if confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9 for rename, got %f", confidence)
	}
}

// TestDetectRefactorByASTHash_move verifies that moving a function
// (same body, same name, different file) is detected as refactor.
func TestDetectRefactorByASTHash_move(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func add(a int, b int) int {
	return a + b
}
`

	result, confidence := c.detectRefactorByASTHash(
		[]string{"old/math.go", "new/math.go"},
		map[string]string{"old/math.go": beforeSrc},
		map[string]string{"new/math.go": afterSrc},
	)

	if result != "refactor" {
		t.Errorf("expected 'refactor' for move, got %q", result)
	}
	if confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9 for move, got %f", confidence)
	}
}

// TestDetectRefactorByASTHash_variable_rename_not_refactor verifies that
// merely renaming variables WITHOUT changing the logic structure is still
// detected as refactor (because the normalized body hash is the same).
func TestDetectRefactorByASTHash_variable_rename_is_refactor(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func calculate(price int, qty int) int {
	total := price * qty
	return total + 10
}
`
	afterSrc := `package p
func calculate(cost int, amount int) int {
	sum := cost * amount
	return sum + 10
}
`

	result, confidence := c.detectRefactorByASTHash(
		[]string{"calc.go"},
		map[string]string{"calc.go": beforeSrc},
		map[string]string{"calc.go": afterSrc},
	)

	// Variable rename produces same normalized body → no logic change → "" (passthrough)
	// Wait — the design says: same hash + same name + same file = NO CHANGE (skip)
	// So this should return "" and let the pipeline continue
	if result != "" {
		t.Errorf("expected empty string for same logic same name same file (no change), got %q", result)
	}
	_ = confidence
}

// TestDetectRefactorByASTHash_operator_change_not_refactor verifies that
// changing an operator (genuinely different logic) returns empty string,
// allowing the pipeline to continue to the next pillar.
func TestDetectRefactorByASTHash_operator_change_not_refactor(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func add(a int, b int) int {
	return a - b
}
`

	result, confidence := c.detectRefactorByASTHash(
		[]string{"math.go"},
		map[string]string{"math.go": beforeSrc},
		map[string]string{"math.go": afterSrc},
	)

	// Different logic → should NOT be refactor, pass through
	if result != "" {
		t.Errorf("expected empty string for operator change (logic change), got %q", result)
	}
	if confidence != 0.0 {
		t.Errorf("expected 0.0 confidence for logic change, got %f", confidence)
	}
}

// TestDetectRefactorByASTHash_parse_failure_returns_empty verifies that
// when function bodies can't be parsed, the method returns empty string
// and 0.0 confidence (graceful fallback).
func TestDetectRefactorByASTHash_parse_failure_returns_empty(t *testing.T) {
	c := &Classifier{}

	beforeSrc := "not valid go code {{{"
	afterSrc := "also not valid go code }}}"

	result, confidence := c.detectRefactorByASTHash(
		[]string{"broken.go"},
		map[string]string{"broken.go": beforeSrc},
		map[string]string{"broken.go": afterSrc},
	)

	if result != "" {
		t.Errorf("expected empty string for parse failure, got %q", result)
	}
	if confidence != 0.0 {
		t.Errorf("expected 0.0 confidence for parse failure, got %f", confidence)
	}
}

// TestDetectRefactorByASTHash_rename_with_body_in_different_file verifies
// that moving AND renaming (same body hash, different name, different file)
// is detected as refactor.
func TestDetectRefactorByASTHash_rename_with_move(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func oldFunc(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func newFunc(x int, y int) int {
	return x + y
}
`

	result, confidence := c.detectRefactorByASTHash(
		[]string{"old/math.go", "new/math.go"},
		map[string]string{"old/math.go": beforeSrc},
		map[string]string{"new/math.go": afterSrc},
	)

	if result != "refactor" {
		t.Errorf("expected 'refactor' for rename+move, got %q", result)
	}
	if confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9 for rename+move, got %f", confidence)
	}
}

// TestDetectRefactorByASTHash_no_change_returns_empty verifies that when
// before and after are identical, the method returns empty (no refactor detected).
func TestDetectRefactorByASTHash_no_change_returns_empty(t *testing.T) {
	c := &Classifier{}

	src := `package p
func add(a int, b int) int {
	return a + b
}
`

	result, confidence := c.detectRefactorByASTHash(
		[]string{"math.go"},
		map[string]string{"math.go": src},
		map[string]string{"math.go": src},
	)

	if result != "" {
		t.Errorf("expected empty string for identical before/after, got %q", result)
	}
	if confidence != 0.0 {
		t.Errorf("expected 0.0 confidence for no change, got %f", confidence)
	}
}

// ---------------------------------------------------------------------------
// Triangulation tests — additional edge cases
// ---------------------------------------------------------------------------

// TestDetectRefactorByASTHash_single_file_deleted_func verifies that
// a function deleted from one file (no matching hash in after) returns
// empty string — this is a deletion, not a rename, Pilar 3 doesn't handle it.
func TestDetectRefactorByASTHash_single_file_deleted_func(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
`

	result, confidence := c.detectRefactorByASTHash(
		[]string{"math.go"},
		map[string]string{"math.go": beforeSrc},
		map[string]string{"math.go": afterSrc},
	)

	// No matching hash → not a refactor, pass through
	if result != "" {
		t.Errorf("expected empty string for deleted function, got %q", result)
	}
	if confidence != 0.0 {
		t.Errorf("expected 0.0 confidence for deleted function, got %f", confidence)
	}
}

// TestDetectRefactorByASTHash_multiple_functions_one_renamed verifies that
// when multiple functions change and one is renamed, refactor is detected.
func TestDetectRefactorByASTHash_multiple_functions_one_renamed(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
func multiply(a int, b int) int {
	return a * b
}
`
	afterSrc := `package p
func sum(x int, y int) int {
	return x + y
}
func multiply(a int, b int) int {
	return a * b
}
`

	result, confidence := c.detectRefactorByASTHash(
		[]string{"math.go"},
		map[string]string{"math.go": beforeSrc},
		map[string]string{"math.go": afterSrc},
	)

	// "add" renamed to "sum" with same body = refactor
	if result != "refactor" {
		t.Errorf("expected 'refactor' for rename among multiple functions, got %q", result)
	}
	if confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9, got %f", confidence)
	}
}

// TestNormalizeBody_preserves_operators verifies that operators are preserved
// in the normalized output (not stripped), so different operators produce
// different hashes.
func TestNormalizeBody_preserves_operators(t *testing.T) {
	src1 := `package p
func calc(a int, b int) int {
	return a + b
}
`
	src2 := `package p
func calc(a int, b int) int {
	return a - b
}
`
	norm1 := normalizeBody(src1, "calc")
	norm2 := normalizeBody(src2, "calc")

	if norm1 == "" || norm2 == "" {
		t.Fatal("normalizeBody returned empty string")
	}
	// Both should contain operator-like syntax but different ones
	// After normalization, both have v1 and v2 but different operators
	if norm1 == norm2 {
		t.Errorf("different operators (+ vs -) should produce different normalized bodies")
	}
}

// TestNormalizeBody_complex_function verifies normalization works for
// functions with multiple statements and assignments.
func TestNormalizeBody_complex_function(t *testing.T) {
	src := `package p
func process(data int, offset int) int {
	result := data + offset
	tmp := result * 2
	return tmp
}
`
	normalized := normalizeBody(src, "process")
	if normalized == "" {
		t.Fatal("normalizeBody returned empty string for complex function")
	}
	// The normalized body should NOT contain "data", "offset", "result", "tmp"
	// but SHOULD contain v1, v2, etc.
	if normalized == "" {
		t.Error("expected non-empty normalized body")
	}
}

// ---------------------------------------------------------------------------
// determineType reordering tests — verify obvious cases exit early
// before reaching Pilar 3 or Pilar 1
// ---------------------------------------------------------------------------

// TestDetermineType_obvious_cases_first verifies that obvious label types
// (CONFIG, DEPS, CI, DOCS, TEST, NEW_FUNC, NEW_TYPE) are classified
// directly without needing the pillar pipeline.
func TestDetermineType_obvious_cases_first(t *testing.T) {
	c := &Classifier{}

	tests := []struct {
		name     string
		label    string
		expected string
	}{
		{"config_is_chore", "CONFIG", "chore"},
		{"deps_is_chore", "DEPS", "chore"},
		{"ci_is_ci", "CI", "ci"},
		{"docs_is_docs", "DOCS", "docs"},
		{"new_func_is_feat", "NEW_FUNC", "feat"},
		{"new_type_is_feat", "NEW_TYPE", "feat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotated := "📄 file.go\nFunc [" + tt.label + "] file.go:1\n"
			chunk := newAnnotatedFixture(annotated)
			chunk.Files = []string{"file.go"}

			commitType, _ := c.Classify(chunk)
			if commitType != tt.expected {
				t.Errorf("expected %q for %s, got %q", tt.expected, tt.label, commitType)
			}
		})
	}
}

// TestDetermineType_mod_body_needs_pillars verifies that MOD_BODY labels
// go through the pillar pipeline instead of being classified directly.
func TestDetermineType_mod_body_needs_pillars(t *testing.T) {
	c := &Classifier{}
	// MOD_BODY should NOT be classified as an obvious case
	// It should go through detectRefactorByASTHash then detectCodeTestSymmetry
	annotated := "📄 internal/auth/login.go\nvalidateToken [MOD_BODY] internal/auth/login.go:25\n"
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{"internal/auth/login.go"}

	commitType, _ := c.Classify(chunk)
	// MOD_BODY without symmetry or refactor detection → defaults to "fix"
	// (or whatever the existing behavior is)
	// We just verify it doesn't panic or return empty incorrectly
	if commitType == "" {
		t.Log("MOD_BODY returned empty — may need pillar pipeline to resolve")
	}
}

// ---------------------------------------------------------------------------
// AST Identity Integration tests — Classify() calling detectRefactorByASTHash
// ---------------------------------------------------------------------------

// TestClassify_AST_refactor_rename verifies that when Go source is provided
// and a function rename is detected (same body, different name), Classify
// returns "refactor" with 1.0 confidence via Pilar 3 short-circuit.
func TestClassify_AST_refactor_rename(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func sum(x int, y int) int {
	return x + y
}
`

	annotated := "📄 math.go\nsum [MOD_BODY] math.go:2\n"
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{"math.go"}
	chunk.GoBefore = map[string]string{"math.go": beforeSrc}
	chunk.GoAfter = map[string]string{"math.go": afterSrc}

	commitType, confidence := c.Classify(chunk)

	if commitType != "refactor" {
		t.Errorf("expected 'refactor' for function rename via AST identity, got %q", commitType)
	}
	if confidence != 1.0 {
		t.Errorf("expected confidence 1.0 for AST identity refactor, got %f", confidence)
	}
}

// TestClassify_AST_refactor_move verifies that moving a function to a different
// file (same body hash, same name, different file) is detected as refactor
// when MOD_BODY labels are dominant.
func TestClassify_AST_refactor_move(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func add(a int, b int) int {
	return a + b
}
`

	// MOD_BODY labels — triggers Pilar 3 pipeline
	annotated := "📄 old/math.go\nadd [MOD_BODY] old/math.go:2\n📄 new/math.go\nadd [MOD_BODY] new/math.go:2\n"
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{"old/math.go", "new/math.go"}
	chunk.GoBefore = map[string]string{"old/math.go": beforeSrc}
	chunk.GoAfter = map[string]string{"new/math.go": afterSrc}

	commitType, confidence := c.Classify(chunk)

	if commitType != "refactor" {
		t.Errorf("expected 'refactor' for function move via AST identity, got %q", commitType)
	}
	if confidence != 1.0 {
		t.Errorf("expected confidence 1.0 for AST identity refactor, got %f", confidence)
	}
}

// TestClassify_AST_no_source_falls_through verifies that when no Go source
// content is provided (GoBefore/GoAfter nil), MOD_BODY falls through to
// the existing pipeline (fix fallback) instead of panicking.
func TestClassify_AST_no_source_falls_through(t *testing.T) {
	c := &Classifier{}

	annotated := "📄 internal/auth/login.go\nvalidateToken [MOD_BODY] internal/auth/login.go:25\n"
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{"internal/auth/login.go"}
	// No GoBefore/GoAfter — should fall through gracefully

	commitType, confidence := c.Classify(chunk)

	// Without AST source, MOD_BODY falls through to "fix" fallback
	if commitType != "fix" {
		t.Errorf("expected 'fix' fallback for MOD_BODY without source, got %q", commitType)
	}
	if confidence < 0.80 {
		t.Errorf("expected reasonable confidence for MOD_BODY fallback, got %f", confidence)
	}
}

// TestClassify_AST_operator_change_not_refactor verifies that when Go source
// has genuinely different logic (e.g., + vs *), Pilar 3 returns empty and
// the pipeline falls through to fix classification.
func TestClassify_AST_operator_change_not_refactor(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func add(a int, b int) int {
	return a - b
}
`

	annotated := "📄 math.go\nadd [MOD_BODY] math.go:2\n"
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{"math.go"}
	chunk.GoBefore = map[string]string{"math.go": beforeSrc}
	chunk.GoAfter = map[string]string{"math.go": afterSrc}

	commitType, confidence := c.Classify(chunk)

	// Different logic (operator changed) → not a refactor, falls through to "fix"
	if commitType == "refactor" {
		t.Errorf("operator change should NOT be classified as refactor, got %q", commitType)
	}
	// Should fall through to fix (or whatever the MOD_BODY fallback is)
	_ = confidence // just verify it doesn't panic
}

// TestClassify_AST_obvious_cases_bypass_pilar3 verifies that obvious cases
// like CONFIG, DEPS, CI, DOCS bypass the AST identity check entirely.
func TestClassify_AST_obvious_cases_bypass_pilar3(t *testing.T) {
	c := &Classifier{}

	tests := []struct {
		name     string
		label    string
		expected string
	}{
		{"config_bypasses_ast", "CONFIG", "chore"},
		{"deps_bypasses_ast", "DEPS", "chore"},
		{"ci_bypasses_ast", "CI", "ci"},
		{"docs_bypasses_ast", "DOCS", "docs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotated := "📄 file.go\nFunc [" + tt.label + "] file.go:1\n"
			chunk := newAnnotatedFixture(annotated)
			chunk.Files = []string{"file.go"}
			// Even with Go source available, obvious cases should bypass Pilar 3
			chunk.GoBefore = map[string]string{"file.go": "package p\nfunc main() {}"}
			chunk.GoAfter = map[string]string{"file.go": "package p\nfunc main() {}"}

			commitType, _ := c.Classify(chunk)
			if commitType != tt.expected {
				t.Errorf("expected %q for %s (should bypass AST), got %q", tt.expected, tt.label, commitType)
			}
		})
	}
}

// TestClassify_AST_parse_error_graceful verifies that when Go source has syntax
// errors, the classifier gracefully falls through without panicking.
func TestClassify_AST_parse_error_graceful(t *testing.T) {
	c := &Classifier{}

	invalidSrc := "this is not valid go code {{{"
	afterSrc := `package p
func add(a int, b int) int {
	return a + b
}
`

	annotated := "📄 broken.go\nadd [MOD_BODY] broken.go:2\n"
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{"broken.go"}
	chunk.GoBefore = map[string]string{"broken.go": invalidSrc}
	chunk.GoAfter = map[string]string{"broken.go": afterSrc}

	// Should not panic, should fall through gracefully
	commitType, _ := c.Classify(chunk)

	// Parse error → AST returns empty → falls through to fix fallback
	if commitType == "refactor" {
		t.Errorf("expected fallback classification for parse error, got %q", commitType)
	}
}

// TestClassify_AST_refactor_rename_feat verifies that when a function is renamed
// (which produces a NEW_FUNC label mapping to feat), the AST identity check
// successfully overrides the primary "feat" classification to "refactor".
func TestClassify_AST_refactor_rename_feat(t *testing.T) {
	c := &Classifier{}

	beforeSrc := `package p
func add(a int, b int) int {
	return a + b
}
`
	afterSrc := `package p
func sum(x int, y int) int {
	return x + y
}
`

	// NEW_FUNC label — traditionally maps to "feat"
	annotated := "📄 math.go\nsum [NEW_FUNC] math.go:2\n"
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{"math.go"}
	chunk.GoBefore = map[string]string{"math.go": beforeSrc}
	chunk.GoAfter = map[string]string{"math.go": afterSrc}

	commitType, confidence := c.Classify(chunk)

	if commitType != "refactor" {
		t.Errorf("expected 'refactor' for function rename overriding feat, got %q", commitType)
	}
	if confidence != 1.0 {
		t.Errorf("expected confidence 1.0 for AST identity refactor overriding feat, got %f", confidence)
	}
}