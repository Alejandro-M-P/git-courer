package domain

import (
	"os"
	"reflect"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/data"
)

// TestMain injects the fixture JSON before all tests in this package.
func TestMain(m *testing.M) {
	if err := data.LoadLanguagesFromBytes([]byte(FixtureJSON)); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestGetLanguageNodesExists verifies lookup for mapped languages.
func TestGetLanguageNodesExists(t *testing.T) {
	t.Parallel()
	cases := []struct {
		lang     string
		minFuncs int
		minTypes int
	}{
		{"Go", 2, 2},
		{"Python", 2, 1},
		{"JavaScript", 3, 1},
		{"TypeScript", 3, 3},
		{"Rust", 1, 4},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			t.Parallel()
			nodes, ok := data.GetLanguageNodes(tc.lang)
			if !ok {
				t.Fatalf("data.GetLanguageNodes(%q) expected ok=true", tc.lang)
			}
			if len(nodes.Functions) < tc.minFuncs {
				t.Errorf("Functions count = %d, want >= %d", len(nodes.Functions), tc.minFuncs)
			}
			if len(nodes.Types) < tc.minTypes {
				t.Errorf("Types count = %d, want >= %d", len(nodes.Types), tc.minTypes)
			}
		})
	}
}

// TestGetLanguageNodesUnknown verifies lookup for unmapped language returns false.
func TestGetLanguageNodesUnknown(t *testing.T) {
	t.Parallel()
	nodes, ok := data.GetLanguageNodes("UnicornLang")
	if ok {
		t.Error("expected ok=false for unknown language")
	}
	if !reflect.DeepEqual(nodes, data.LanguageNodes{}) {
		t.Errorf("expected zero LanguageNodes for unknown language, got %+v", nodes)
	}
}

// TestLabelTypeConstants verifies all label constants exist and have correct string values.
func TestLabelTypeConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want string
	}{
		{string(NEW_FUNC), "NEW_FUNC"},
		{string(MOD_BODY), "MOD_BODY"},
		{string(MOD_SIG), "MOD_SIG"},
		{string(DELETED_FUNC), "DELETED_FUNC"},
		{string(NEW_TYPE), "NEW_TYPE"},
		{string(MOD_TYPE), "MOD_TYPE"},
		{string(DELETED_TYPE), "DELETED_TYPE"},
		{string(MOD_BODY_LOGIC), "MOD_BODY_LOGIC"},
		{string(MOD_BODY_ERROR), "MOD_BODY_ERROR"},
		{string(MOD_BODY_REORDER), "MOD_BODY_REORDER"},
		{string(MOD_BODY_CALL), "MOD_BODY_CALL"},
		{string(CHANGED), "CHANGED"},
		{string(RENAMED), "RENAMED"},
		{string(RENAMED_FUNC), "RENAMED_FUNC"},
		{string(RENAMED_TYPE), "RENAMED_TYPE"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if tc.name != tc.want {
				t.Errorf("constant value mismatch: got %q, want %q", tc.name, tc.want)
			}
		})
	}
}

// TestLabelStruct verifies Label struct fields.
func TestLabelStruct(t *testing.T) {
	t.Parallel()
	label := Label{
		Type:     NEW_FUNC,
		Name:     "HandleWebhook",
		File:     "internal/server/webhook.go",
		Line:     42,
		Breaking: false,
	}
	// Reflection to verify all exported fields exist
	val := reflect.ValueOf(label)
	if val.NumField() != 5 {
		t.Fatalf("Label struct should have 5 fields, got %d", val.NumField())
	}

	if label.Type != NEW_FUNC {
		t.Errorf("Type = %q, want %q", label.Type, NEW_FUNC)
	}
	if label.Name != "HandleWebhook" {
		t.Errorf("Name = %q, want HandleWebhook", label.Name)
	}
	if label.File != "internal/server/webhook.go" {
		t.Errorf("File = %q, want internal/server/webhook.go", label.File)
	}
	if label.Line != 42 {
		t.Errorf("Line = %d, want 42", label.Line)
	}
	if label.Breaking {
		t.Error("Breaking should be false")
	}
}

// TestLabelZeroValue verifies zero-value behavior.
func TestLabelZeroValue(t *testing.T) {
	t.Parallel()
	var label Label
	if label.Type != "" {
		t.Errorf("zero-value Type = %q, want empty", label.Type)
	}
	if label.Name != "" {
		t.Errorf("zero-value Name = %q, want empty", label.Name)
	}
	if label.File != "" {
		t.Errorf("zero-value File = %q, want empty", label.File)
	}
	if label.Line != 0 {
		t.Errorf("zero-value Line = %d, want 0", label.Line)
	}
	if label.Breaking {
		t.Error("zero-value Breaking should be false")
	}
}

// TestLabelWithBreakingChange verifies Breaking flag on a public signature change.
func TestLabelWithBreakingChange(t *testing.T) {
	t.Parallel()
	label := Label{
		Type:     MOD_SIG,
		Name:     "HandleRequest",
		File:     "handlers.go",
		Line:     10,
		Breaking: true,
	}
	if !label.Breaking {
		t.Error("Breaking should be true for a public signature change")
	}
}

// TestLoadLanguagesFromBytes_Valid verifies injection with valid fixture data.
func TestLoadLanguagesFromBytes_Valid(t *testing.T) {
	err := data.LoadLanguagesFromBytes([]byte(FixtureJSON))
	if err != nil {
		t.Fatalf("LoadLanguagesFromBytes with fixture should succeed: %v", err)
	}
	nodes, ok := data.GetLanguageNodes("Python")
	if !ok {
		t.Fatal("Python should exist after injection")
	}
	if len(nodes.Functions) < 2 {
		t.Errorf("Python functions should be >= 2, got %d", len(nodes.Functions))
	}
}

// TestLoadLanguagesFromBytes_Invalid verifies injection with invalid JSON returns error.
func TestLoadLanguagesFromBytes_Invalid(t *testing.T) {
	// Save current state
	saved, _ := data.GetLanguageNodes("Go")
	err := data.LoadLanguagesFromBytes([]byte("{invalid"))
	if err == nil {
		t.Fatal("LoadLanguagesFromBytes with invalid JSON should return error")
	}
	// Previous state should be preserved
	current, ok := data.GetLanguageNodes("Go")
	if !ok {
		t.Fatal("Go should still exist after failed injection")
	}
	if len(current.Functions) != len(saved.Functions) {
		t.Error("failed injection should not modify state")
	}
}

// TestLoadLanguagesFromBytes_Overwrite verifies injection overwrites previous state.
func TestLoadLanguagesFromBytes_Overwrite(t *testing.T) {
	overwriteJSON := `{"languages": {"NewLang": {"functions": ["fn1"], "types": ["t1"]}}}`
	err := data.LoadLanguagesFromBytes([]byte(overwriteJSON))
	if err != nil {
		t.Fatalf("LoadLanguagesFromBytes with overwrite JSON: %v", err)
	}
	// NewLang should now exist
	nodes, ok := data.GetLanguageNodes("NewLang")
	if !ok {
		t.Fatal("NewLang should exist after overwrite")
	}
	if len(nodes.Functions) != 1 || nodes.Functions[0] != "fn1" {
		t.Errorf("NewLang functions mismatch: %v", nodes.Functions)
	}
	// Old languages should NOT be present (overwritten)
	if _, ok := data.GetLanguageNodes("Go"); ok {
		t.Error("Go should NOT exist after full overwrite")
	}
	// Restore fixture so other tests aren't affected
	if err := data.LoadLanguagesFromBytes([]byte(FixtureJSON)); err != nil {
		t.Fatalf("failed to restore fixture: %v", err)
	}
}
