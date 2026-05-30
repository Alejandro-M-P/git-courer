package domain

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/data"
)

// TestNewLanguageCatalog_ConstructsWithMaps verifies that NewLanguageCatalog creates
// a catalog from pre-built language maps.
func TestNewLanguageCatalog_ConstructsWithMaps(t *testing.T) {
	t.Parallel()

	byName := map[string]data.LanguageNodes{
		"Go": {Functions: []string{"function_declaration"}, Types: []string{"type_declaration"}},
	}
	testIndex := map[string][]data.TestPattern{
		"Go": {{Type: "suffix", Value: "_test.go"}},
	}

	catalog := NewLanguageCatalog(byName, testIndex, nil)

	if catalog == nil {
		t.Fatal("NewLanguageCatalog returned nil")
	}
}

// TestLanguageCatalog_ByName verifies ByName returns the correct LanguageNodes.
func TestLanguageCatalog_ByName(t *testing.T) {
	t.Parallel()

	byName := map[string]data.LanguageNodes{
		"Go": {Functions: []string{"function_declaration"}, Types: []string{"type_declaration"}},
	}
	catalog := NewLanguageCatalog(byName, nil, nil)

	nodes, ok := catalog.ByName("Go")
	if !ok {
		t.Fatal("ByName(\"Go\") returned false, want true")
	}
	if len(nodes.Functions) == 0 {
		t.Error("ByName(\"Go\").Functions is empty, expected non-empty")
	}

	_, ok = catalog.ByName("NonExistent")
	if ok {
		t.Error("ByName(\"NonExistent\") returned true, want false")
	}
}

// TestLanguageCatalog_IsTestFile verifies IsTestFile matches known test patterns.
func TestLanguageCatalog_IsTestFile(t *testing.T) {
	t.Parallel()

	testIndex := map[string][]data.TestPattern{
		"Go": {{Type: "suffix", Value: "_test.go"}},
	}
	byName := map[string]data.LanguageNodes{
		"Go": {},
	}
	catalog := NewLanguageCatalog(byName, testIndex, nil)

	if !catalog.IsTestFile("handler_test.go") {
		t.Error("IsTestFile(\"handler_test.go\") = false, want true")
	}
	if catalog.IsTestFile("handler.go") {
		t.Error("IsTestFile(\"handler.go\") = true, want false")
	}
}

// TestLanguageCatalog_ArePaired verifies code-test file pairing.
func TestLanguageCatalog_ArePaired(t *testing.T) {
	t.Parallel()

	testIndex := map[string][]data.TestPattern{
		"Go": {{Type: "suffix", Value: "_test.go"}},
	}
	byName := map[string]data.LanguageNodes{
		"Go": {},
	}
	catalog := NewLanguageCatalog(byName, testIndex, nil)

	if !catalog.ArePaired("handler.go", "handler_test.go") {
		t.Error("ArePaired(\"handler.go\", \"handler_test.go\") = false, want true")
	}
	if catalog.ArePaired("handler.go", "util_test.go") {
		t.Error("ArePaired(\"handler.go\", \"util_test.go\") = true, want false")
	}
}

// TestLanguageCatalog_ExtensionToLanguage_WithDetector verifies ExtensionToLanguage
// uses the injected extension detector.
func TestLanguageCatalog_ExtensionToLanguage_WithDetector(t *testing.T) {
	t.Parallel()

	byName := map[string]data.LanguageNodes{
		"Go": {Functions: []string{"function_declaration"}},
	}

	// Mock detector: returns "go" for .go extension
	detector := func(ext string) *string {
		if ext == "go" {
			s := "go"
			return &s
		}
		return nil
	}

	catalog := NewLanguageCatalog(byName, nil, detector)

	entry, ok := catalog.ExtensionToLanguage(".go")
	if !ok {
		t.Fatal("ExtensionToLanguage(\".go\") returned false, want true")
	}
	if entry.DomainName != "Go" {
		t.Errorf("ExtensionToLanguage(\".go\").DomainName = %q, want %q", entry.DomainName, "Go")
	}
	if !entry.HasGrammar {
		t.Error("ExtensionToLanguage(\".go\").HasGrammar = false, want true")
	}
}

// TestLanguageCatalog_ExtensionToLanguage_NoDetector verifies ExtensionToLanguage
// returns false when no detector is set.
func TestLanguageCatalog_ExtensionToLanguage_NoDetector(t *testing.T) {
	t.Parallel()

	catalog := NewLanguageCatalog(nil, nil, nil)

	_, ok := catalog.ExtensionToLanguage(".go")
	if ok {
		t.Error("ExtensionToLanguage with nil detector should return false")
	}
}

// TestLanguageCatalog_DebugInfo_Empty verifies DebugInfo for an empty catalog.
func TestLanguageCatalog_DebugInfo_Empty(t *testing.T) {
	t.Parallel()

	catalog := NewLanguageCatalog(nil, nil, nil)
	info := catalog.DebugInfo()

	byNameSize, ok := info["byNameSize"].(int)
	if !ok {
		t.Fatal("DebugInfo()[\"byNameSize\"] should be an int")
	}
	if byNameSize != 0 {
		t.Errorf("DebugInfo()[\"byNameSize\"] = %d, want 0", byNameSize)
	}

	testIndexSize, ok := info["testIndexSize"].(int)
	if !ok {
		t.Fatal("DebugInfo()[\"testIndexSize\"] should be an int")
	}
	if testIndexSize != 0 {
		t.Errorf("DebugInfo()[\"testIndexSize\"] = %d, want 0", testIndexSize)
	}

	patternCounts, ok := info["testPatternCounts"].(map[string]int)
	if !ok {
		t.Fatal("DebugInfo()[\"testPatternCounts\"] should be map[string]int")
	}
	if len(patternCounts) != 0 {
		t.Errorf("DebugInfo()[\"testPatternCounts\"] has %d entries, want 0", len(patternCounts))
	}
}

// TestLanguageCatalog_DebugInfo_WithData verifies DebugInfo with populated catalog.
func TestLanguageCatalog_DebugInfo_WithData(t *testing.T) {
	t.Parallel()

	byName := map[string]data.LanguageNodes{
		"Go": {Functions: []string{"function_declaration"}},
	}
	testIndex := map[string][]data.TestPattern{
		"Go": {{Type: "suffix", Value: "_test.go"}},
	}

	catalog := NewLanguageCatalog(byName, testIndex, nil)
	info := catalog.DebugInfo()

	byNameSize, ok := info["byNameSize"].(int)
	if !ok {
		t.Fatal("DebugInfo()[\"byNameSize\"] should be an int")
	}
	if byNameSize != 1 {
		t.Errorf("DebugInfo()[\"byNameSize\"] = %d, want 1", byNameSize)
	}

	testIndexSize, ok := info["testIndexSize"].(int)
	if !ok {
		t.Fatal("DebugInfo()[\"testIndexSize\"] should be an int")
	}
	if testIndexSize != 1 {
		t.Errorf("DebugInfo()[\"testIndexSize\"] = %d, want 1", testIndexSize)
	}

	patternCounts, ok := info["testPatternCounts"].(map[string]int)
	if !ok {
		t.Fatal("DebugInfo()[\"testPatternCounts\"] should be map[string]int")
	}
	if len(patternCounts) != 1 {
		t.Errorf("DebugInfo()[\"testPatternCounts\"] has %d entries, want 1", len(patternCounts))
	}
	if patternCounts["Go"] != 1 {
		t.Errorf("DebugInfo()[\"testPatternCounts\"][\"Go\"] = %d, want 1", patternCounts["Go"])
	}
}

// TestLanguageCatalog_HasConsistentExtension verifies hasConsistentExtension behavior.
func TestLanguageCatalog_HasConsistentExtension(t *testing.T) {
	t.Parallel()

	t.Run("with InDir set", func(t *testing.T) {
		t.Parallel()
		// hasConsistentExtension always returns true currently
		if !hasConsistentExtension("handler.go", data.TestPattern{InDir: "test"}) {
			t.Error("hasConsistentExtension with InDir set should return true")
		}
	})

	t.Run("with empty InDir", func(t *testing.T) {
		t.Parallel()
		if !hasConsistentExtension("handler.go", data.TestPattern{InDir: ""}) {
			t.Error("hasConsistentExtension with empty InDir should return true")
		}
	})
}

// TestLanguageCatalog_ExtensionToLanguage_UnknownExt verifies unknown extensions.
func TestLanguageCatalog_ExtensionToLanguage_UnknownExt(t *testing.T) {
	t.Parallel()

	detector := func(ext string) *string { return nil }
	catalog := NewLanguageCatalog(nil, nil, detector)

	_, ok := catalog.ExtensionToLanguage(".totallyunknown")
	if ok {
		t.Error("ExtensionToLanguage for unknown extension should return false")
	}
}