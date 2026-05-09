package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockDetector is a test double for OllamaDetector.
type mockDetector struct {
	results map[string]mockResult
	calls   int // track how many times Lookup was called
}

type mockResult struct {
	contextWindow int
	found         bool
}

func (m *mockDetector) Lookup(ctx context.Context, model string) (int, bool) {
	m.calls++
	if r, ok := m.results[model]; ok {
		return r.contextWindow, r.found
	}
	return 0, false
}

func TestGetContextWindow_ExactMatch(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	// "llama3.1:8b" exists in catalog → 8192
	ctxWindow := catalog.GetContextWindow("llama3.1:8b")
	if ctxWindow != 8192 {
		t.Errorf("GetContextWindow(\"llama3.1:8b\") = %d, want 8192", ctxWindow)
	}
}

func TestGetContextWindow_ExactMatchNoTag(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	// "deepseek-chat" exists in catalog without tag → 131072
	ctxWindow := catalog.GetContextWindow("deepseek-chat")
	if ctxWindow != 131072 {
		t.Errorf("GetContextWindow(\"deepseek-chat\") = %d, want 131072", ctxWindow)
	}
}

func TestGetContextWindow_FamilyMatch(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	// "llama3.1:13b" is NOT in catalog, but "llama3.1:8b" IS.
	// Strip tag → "llama3.1", then base name index has "llama3.1" → 8192
	ctxWindow := catalog.GetContextWindow("llama3.1:13b")
	if ctxWindow != 8192 {
		t.Errorf("GetContextWindow(\"llama3.1:13b\") = %d, want 8192 (family match)", ctxWindow)
	}
}

func TestGetContextWindow_FamilyMatchTagless(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	// "mistral" exists in catalog (no tag) → 8192
	ctxWindow := catalog.GetContextWindow("mistral")
	if ctxWindow != 8192 {
		t.Errorf("GetContextWindow(\"mistral\") = %d, want 8192", ctxWindow)
	}
}

func TestGetContextWindow_UnknownModel(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	ctxWindow := catalog.GetContextWindow("unknown-model")
	if ctxWindow != 4096 {
		t.Errorf("GetContextWindow(\"unknown-model\") = %d, want 4096 (default)", ctxWindow)
	}
}

func TestGetContextWindow_EmptyModel(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	ctxWindow := catalog.GetContextWindow("")
	if ctxWindow != 4096 {
		t.Errorf("GetContextWindow(\"\") = %d, want 4096 (default)", ctxWindow)
	}
}

func TestGetContextWindow_WithOllamaDetector(t *testing.T) {
	detector := &mockDetector{
		results: map[string]mockResult{
			"custom-model:7b": {contextWindow: 65536, found: true},
		},
	}

	catalog := NewModelCatalogWithDetector(detector)

	ctxWindow := catalog.GetContextWindow("custom-model:7b")
	if ctxWindow != 65536 {
		t.Errorf("GetContextWindow(\"custom-model:7b\") = %d, want 65536 (from detector)", ctxWindow)
	}
}

func TestGetContextWindow_OllamaDetectorOverridesCatalog(t *testing.T) {
	detector := &mockDetector{
		results: map[string]mockResult{
			"llama3.1:8b": {contextWindow: 999999, found: true},
		},
	}

	catalog := NewModelCatalogWithDetector(detector)

	// Detector takes priority over catalog
	ctxWindow := catalog.GetContextWindow("llama3.1:8b")
	if ctxWindow != 999999 {
		t.Errorf("GetContextWindow(\"llama3.1:8b\") = %d, want 999999 (detector overrides)", ctxWindow)
	}
}

func TestGetContextWindow_OllamaDetectorFails(t *testing.T) {
	detector := &mockDetector{
		results: map[string]mockResult{
			"llama3.1:8b": {contextWindow: 0, found: false},
		},
	}

	catalog := NewModelCatalogWithDetector(detector)

	// Detector fails → falls through to catalog → 8192
	ctxWindow := catalog.GetContextWindow("llama3.1:8b")
	if ctxWindow != 8192 {
		t.Errorf("GetContextWindow(\"llama3.1:8b\") = %d, want 8192 (catalog fallback)", ctxWindow)
	}
}

func TestGetContextWindow_OllamaDetectorFailsUnknownModel(t *testing.T) {
	detector := &mockDetector{
		results: map[string]mockResult{
			"unknown": {contextWindow: 0, found: false},
		},
	}

	catalog := NewModelCatalogWithDetector(detector)

	// Detector fails → catalog miss → default
	ctxWindow := catalog.GetContextWindow("unknown")
	if ctxWindow != 4096 {
		t.Errorf("GetContextWindow(\"unknown\") = %d, want 4096 (default)", ctxWindow)
	}
}

func TestGetContextWindow_Default(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	// Completely unknown model, no ollama
	ctxWindow := catalog.GetContextWindow("totally-unknown:123b")
	if ctxWindow != 4096 {
		t.Errorf("GetContextWindow(\"totally-unknown:123b\") = %d, want 4096", ctxWindow)
	}
}

func TestGetContextWindow_Table(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	tests := []struct {
		model    string
		expected int
		reason   string
	}{
		{"llama3:8b", 8192, "exact match"},
		{"llama3:70b", 8192, "exact match"},
		{"deepseek-chat", 131072, "exact match"},
		{"llama2:7b", 4096, "exact match"},
		{"mistral", 8192, "exact match"},
		{"llama3.1:8b", 8192, "family match"},
		{"llama3.1:70b", 8192, "family match"},
		{"deepseek-v3", 4096, "default (key is not prefix of query)"},
		{"unknown-model", 4096, "default"},
		{"", 4096, "empty string default"},
	}

	for _, tt := range tests {
		t.Run(tt.model+"_("+tt.reason+")", func(t *testing.T) {
			got := catalog.GetContextWindow(tt.model)
			if got != tt.expected {
				t.Errorf("GetContextWindow(%q) = %d, want %d (%s)", tt.model, got, tt.expected, tt.reason)
			}
		})
	}
}

func TestGetContextWindowWithCtx_PassesContext(t *testing.T) {
	response := `{"model_info":{"general.architecture":"llama","llama.context_length":32768}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})
	catalog := NewModelCatalogWithDetector(detector)

	ctx := context.Background()
	ctxWindow := catalog.GetContextWindowWithCtx(ctx, "any-model")
	if ctxWindow != 32768 {
		t.Errorf("GetContextWindowWithCtx = %d, want 32768", ctxWindow)
	}
}

func TestNewModelCatalog_LoadsEmbeddedData(t *testing.T) {
	catalog := NewModelCatalogNoOllama()

	// Verify that embedded data loaded correctly — check a known model
	if len(catalog.data) == 0 {
		t.Error("Expected catalog data to be non-empty from embedded JSON")
	}

	// Check that the base name index was built
	if len(catalog.baseName) == 0 {
		t.Error("Expected base name index to be non-empty")
	}
}

func TestNewModelCatalogWithDetector_NilDetector(t *testing.T) {
	// Passing nil detector should work (no Ollama detection)
	catalog := NewModelCatalogWithDetector(nil)
	if catalog.detector != nil {
		t.Error("Expected nil detector")
	}

	// Should fall through to catalog or default
	// llama3:8b exists in catalog → 8192
	ctxWindow := catalog.GetContextWindow("llama3:8b")
	if ctxWindow != 8192 {
		t.Errorf("Expected 8192 from catalog, got %d", ctxWindow)
	}
}

func TestStripTag(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"llama3.1:8b", "llama3.1"},
		{"mistral:7b", "mistral"},
		{"deepseek-v3", "deepseek-v3"},
		{"", ""},
		{"model:with:colons:7b", "model:with:colons"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := stripTag(tt.model)
			if got != tt.expected {
				t.Errorf("stripTag(%q) = %q, want %q", tt.model, got, tt.expected)
			}
		})
	}
}

func TestBuildBaseNameIndex(t *testing.T) {
	modelData := map[string]int{
		"llama3.1:8b":  8192,
		"llama3.1:70b": 128000,
		"mistral:7b":   32768,  // Tag variant in test data
		"deepseek-v3":  131072, // No-tag variant in test data
	}

	index := buildBaseNameIndex(modelData)

	// Both llama3.1 variants map to same base; smallest wins (most conservative)
	if index["llama3.1"] != 8192 {
		t.Errorf("Expected llama3.1 base → 8192 (smallest), got %d", index["llama3.1"])
	}

	// "mistral" (base of mistral:7b) → 32768
	if index["mistral"] != 32768 {
		t.Errorf("Expected mistral base → 32768, got %d", index["mistral"])
	}

	// "deepseek-v3" has no tag, so base is same as original
	if index["deepseek-v3"] != 131072 {
		t.Errorf("Expected deepseek-v3 base → 131072, got %d", index["deepseek-v3"])
	}
}

func TestLongestPrefixMatch(t *testing.T) {
	index := map[string]int{
		"llama3.1": 8192,
		"llama3":   8192,
		"llama2":   4096,
		"mistral":  32768,
	}

	tests := []struct {
		name     string
		query    string
		expected int
		found    bool
	}{
		{"exact match", "llama3.1", 8192, true},
		{"prefix match longer", "llama3.1:13b", 8192, true}, // "llama3.1" is prefix of "llama3.1:13b"
		{"no match for short query", "llama", 0, false},     // "llama" is NOT a prefix of "llama3.1" - query must be longer
		{"no match", "unknown", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := longestPrefixMatch(tt.query, index)
			if found != tt.found {
				t.Errorf("longestPrefixMatch(%q) found=%v, want %v", tt.query, found, tt.found)
			}
			if got != tt.expected {
				t.Errorf("longestPrefixMatch(%q) = %d, want %d", tt.query, got, tt.expected)
			}
		})
	}
}