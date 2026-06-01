package installer

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveContextWindow(t *testing.T) {
	tests := []struct {
		name                string
		model               string
		configContextWindow int
		ollamaResp          string
		ollamaStatus        int
		expectedWindow      int
	}{
		{
			name:                "Ollama returns context",
			model:               "llama3",
			configContextWindow: 16384,
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
			name:                "Ollama unavailable, config fallback",
			model:               "gpt-4",
			configContextWindow: 16384,
			ollamaStatus:        http.StatusNotFound,
			ollamaResp:          `{}`,
			expectedWindow:      16384,
		},
		{
			name:                "Ollama unavailable, config zero uses default",
			model:               "unknown",
			configContextWindow: 0,
			ollamaStatus:        http.StatusNotFound,
			ollamaResp:          `{}`,
			expectedWindow:      8192,
		},
		{
			name:                "Ollama returns value AND config has value, Ollama wins",
			model:               "llama3",
			configContextWindow: 32768,
			ollamaResp: `{
				"model_info": {
					"general.architecture": "llama",
					"llama.context_length": 8192
				}
			}`,
			ollamaStatus:   http.StatusOK,
			expectedWindow: 8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/show" {
					status := tt.ollamaStatus
					if status == 0 {
						status = http.StatusOK
					}
					w.WriteHeader(status)
					w.Write([]byte(tt.ollamaResp))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			res := resolveContextWindow(tt.model, server.URL, server.Client(), tt.configContextWindow)
			if res != tt.expectedWindow {
				t.Errorf("expected %d, got %d", tt.expectedWindow, res)
			}
		})
	}
}
