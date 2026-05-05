package domain

import (
	"reflect"
	"testing"
)

// TestGetLanguageNodesExists verifies lookup for mapped languages.
func TestGetLanguageNodesExists(t *testing.T) {
	t.Parallel()
	cases := []struct {
		lang     string
		wantName string
		fnCount  int
		tyCount  int
	}{
		{"Go", "Go", 2, 2},
		{"Python", "Python", 2, 1},
		{"JavaScript", "JavaScript", 3, 1},
		{"TypeScript", "TypeScript", 3, 3},
		{"Rust", "Rust", 1, 4},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			t.Parallel()
			nodes, ok := GetLanguageNodes(tc.lang)
			if !ok {
				t.Fatalf("GetLanguageNodes(%q) expected ok=true", tc.lang)
			}
			if len(nodes.Functions) != tc.fnCount {
				t.Errorf("Functions count = %d, want %d", len(nodes.Functions), tc.fnCount)
			}
			if len(nodes.Types) != tc.tyCount {
				t.Errorf("Types count = %d, want %d", len(nodes.Types), tc.tyCount)
			}
		})
	}
}

// TestGetLanguageNodesUnknown verifies lookup for unmapped language returns false.
func TestGetLanguageNodesUnknown(t *testing.T) {
	t.Parallel()
	nodes, ok := GetLanguageNodes("UnicornLang")
	if ok {
		t.Error("expected ok=false for unknown language")
	}
	if !reflect.DeepEqual(nodes, LanguageNodes{}) {
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
