package installer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveContextWindow(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		liteLLMResp    string
		liteLLMStatus  int
		ollamaResp     string
		ollamaStatus   int
		expectedWindow int
	}{
		{
			name:  "LiteLLM success",
			model: "gpt-4",
			liteLLMResp: `{
				"gpt-4": {
					"max_input_tokens": 16384
				}
			}`,
			liteLLMStatus:  http.StatusOK,
			expectedWindow: 8192, // 16384 / 2
		},
		{
			name:          "LiteLLM fails, Ollama success",
			model:         "llama3",
			liteLLMStatus: http.StatusNotFound,
			ollamaResp: `{
				"model_info": {
					"general.architecture": "llama",
					"llama.context_length": 4096
				}
			}`,
			ollamaStatus:   http.StatusOK,
			expectedWindow: 4096,
		},
		{
			name:           "All fail, default fallback",
			model:          "unknown",
			liteLLMStatus:  http.StatusNotFound,
			ollamaStatus:   http.StatusNotFound,
			expectedWindow: 8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// We don't easily know which URL it's hitting if they are both server.URL
				// but queryLiteLLM uses GET and Ollama Lookup uses POST.
				if r.Method == http.MethodGet {
					w.WriteHeader(tt.liteLLMStatus)
					fmt.Fprint(w, tt.liteLLMResp)
				} else if r.Method == http.MethodPost {
					w.WriteHeader(tt.ollamaStatus)
					fmt.Fprint(w, tt.ollamaResp)
				}
			}))
			defer server.Close()

			res := resolveContextWindow(tt.model, server.URL, server.Client(), server.URL)
			if res != tt.expectedWindow {
				t.Errorf("expected %d, got %d", tt.expectedWindow, res)
			}
		})
	}
}
