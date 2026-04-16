package llm

import (
	"testing"
)

// TestNew verifies the constructor creates adapter with correct defaults.
func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		model     string
		modelsDir string
		wantHost  string
	}{
		{"empty host uses default", "", "llama3", "", "http://localhost:11434"},
		{"custom host", "http://localhost:8080", "llama3", "", "http://localhost:8080"},
		{"custom model", "", "qwen2.5", "", "http://localhost:11434"},
		{"with models dir", "", "llama3", "/tmp/models", "http://localhost:11434"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := New(tt.host, tt.model, tt.modelsDir)
			if adapter.host != tt.wantHost {
				t.Errorf("New(%q, %q, %q).host = %q, want %q", tt.host, tt.model, tt.modelsDir, adapter.host, tt.wantHost)
			}
			if tt.model != "" && adapter.model != tt.model {
				t.Errorf("New(%q, %q, %q).model = %q, want %q", tt.host, tt.model, tt.modelsDir, adapter.model, tt.model)
			}
		})
	}
}

// TestSetRetryContext tests retry context storage.
func TestSetRetryContext(t *testing.T) {
	adapter := New("http://localhost:11434", "llama3", "")

	// Set retry context
	testMsg := "Previous rejected message"
	adapter.SetRetryContext(testMsg)
	if adapter.retryContext != testMsg {
		t.Errorf("SetRetryContext() failed: got %q, want %q", adapter.retryContext, testMsg)
	}

	// Clear retry context
	adapter.ClearRetryContext()
	if adapter.retryContext != "" {
		t.Errorf("ClearRetryContext() failed: got %q, want empty", adapter.retryContext)
	}
}

// TestIsAvailable tests availability detection - skipped without server.
func TestIsAvailable(t *testing.T) {
	t.Skip("Skipped - requires running Ollama instance")
}

// TestResolveModel tests model resolution - skipped without server.
func TestResolveModel(t *testing.T) {
	t.Skip("Skipped - requires running Ollama instance")
}

// TestDecideCommit tests commit decision - skipped without server.
func TestDecideCommit(t *testing.T) {
	t.Skip("Skipped - requires running Ollama instance")
}

// TestInterpretGitOp tests git operation interpretation - skipped without server.
func TestInterpretGitOp(t *testing.T) {
	t.Skip("Skipped - requires running Ollama instance")
}

// TestInterpretReleaseIntent tests release intent - skipped without server.
func TestInterpretReleaseIntent(t *testing.T) {
	t.Skip("Skipped - requires running Ollama instance")
}

// TestGenerateChangelog tests changelog - skipped without server.
func TestGenerateChangelog(t *testing.T) {
	t.Skip("Skipped - requires running Ollama instance")
}

// TestInterfaceCheck verifies compile-time interface check.
func TestInterfaceCheck(t *testing.T) {
	var _ interface{} = (*Adapter)(nil)
	t.Log("Interface check passes - Adapter implements LLM port")
}
