package config

import "testing"

func TestDeriveContextWindow_KnownModels(t *testing.T) {
	cases := []struct {
		model  string
		expect int
	}{
		{"qwen3.5:0.8b", 32768},
		{"llama3.1:8b", 8192},
		{"llama3.1:70b", 128000},
		{"gemma2:9b", 8192},
		{"mistral:7b", 32768},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			got := DeriveContextWindow(tc.model)
			if got != tc.expect {
				t.Errorf("DeriveContextWindow(%q) = %d, want %d", tc.model, got, tc.expect)
			}
		})
	}
}

func TestDeriveContextWindow_UnknownModel(t *testing.T) {
	// Unknown models should return fallback (4096)
	fallback := DeriveContextWindow("unknown-model:99b")
	if fallback != 4096 {
		t.Errorf("DeriveContextWindow(unknown) = %d, want 4096 (fallback)", fallback)
	}
}

func TestDeriveContextWindow_EmptyModel(t *testing.T) {
	// Empty model should also return fallback
	fallback := DeriveContextWindow("")
	if fallback != 4096 {
		t.Errorf("DeriveContextWindow(\"\") = %d, want 4096 (fallback)", fallback)
	}
}

func TestModelRegistry_ContainsExpectedModels(t *testing.T) {
	expected := []string{
		"qwen3.5:0.8b",
		"llama3.1:8b",
		"llama3.1:70b",
		"gemma2:9b",
		"mistral:7b",
	}
	for _, model := range expected {
		if _, ok := modelWindows[model]; !ok {
			t.Errorf("modelWindows missing expected model: %s", model)
		}
	}
}

func TestModelRegistry_MapNotEmpty(t *testing.T) {
	if len(modelWindows) == 0 {
		t.Error("modelWindows should not be empty")
	}
}