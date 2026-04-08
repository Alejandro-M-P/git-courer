package security

import "testing"

func TestParseModelSize(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		// Large models (14B+)
		{"qwen 14b", "qwen3.5-14b", "large"},
		{"llama 70b", "llama3.1-70b", "large"},
		{"codellama 34b", "codellama-34b", "large"},
		{"mistral 8x22b", "mistral-8x22b", "large"}, // 22B
		{"qwen 32b", "qwen2.5-32b", "large"},

		// Medium models (7B-13B)
		{"qwen 7b", "qwen2.5-7b", "medium"},
		{"mistral 7b", "mistral-7b", "medium"},
		{"llama 8b", "llama3.2-8b", "medium"},
		{"codellama 13b", "codellama-13b", "medium"},

		// Small models (< 7B)
		{"phi 3b", "phi-3b", "small"},
		{"gemma 2b", "gemma-2b", "small"},

		// Edge cases
		{"exact 14b should be large", "model-14b", "large"},
		{"13b should be medium", "model-13b", "medium"},
		{"6b should be small", "model-6b", "small"},
		{"unknown model", "unknown-model", "small"},
		{"empty string", "", "small"},

		// Case insensitive
		{"uppercase QWEN", "QWEN3.5-14B", "large"},
		{"mixed case Llama", "Llama3.1-70B", "large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseModelSize(tt.model)
			if string(got) != tt.expected {
				t.Errorf("ParseModelSize(%q) = %q, want %q", tt.model, got, tt.expected)
			}
		})
	}
}
