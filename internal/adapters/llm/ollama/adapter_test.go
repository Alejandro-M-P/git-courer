package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// newAdapterServer creates a mock server that handles both Ollama /api/ endpoints
// and OpenAI /v1/ endpoints for testing the OllamaAdapter.
func newAdapterServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "test-model"}},
			})
		case "/api/generate":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response": "{\"ok\": true}",
			})
		case "/v1/models":
			w.WriteHeader(200)
			w.Write([]byte(`{"data":[]}`))
		case "/v1/chat/completions":
			var req struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"role": "assistant", "content": "delegated response"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case "/v1/completions":
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"text": "delegated completion"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		default:
			t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

// newTestOllamaAdapter creates an OllamaAdapter wired to the given test server.
func newTestOllamaAdapter(server *httptest.Server) *OllamaAdapter {
	return NewOllamaAdapter(server.URL, "test-model", "", false,
		WithAdapterHTTPClient(server.Client()),
	)
}

// TestOllamaAdapter_DelegatesGenerateChunkMessage verifies that
// GenerateChunkMessage delegates to the standard adapter.
func TestOllamaAdapter_DelegatesGenerateChunkMessage(t *testing.T) {
	server := newAdapterServer(t)
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	msg, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
	if msg != "delegated completion" {
		t.Errorf("message: got %q, want %q", msg, "delegated completion")
	}
}

// TestOllamaAdapter_DelegatesDecideCommit verifies that DecideCommit
// delegates to the standard adapter.
func TestOllamaAdapter_DelegatesDecideCommit(t *testing.T) {
	// Need a server that returns JSON for DecideCommit
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"role": "assistant", "content": `{"include_untracked": true, "file_filter": ["src/"]}`}},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	intent, err := adapter.DecideCommit("commit all", "M file.go", "new.go", "", "")
	if err != nil {
		t.Fatalf("DecideCommit failed: %v", err)
	}
	if intent.IncludeUntracked != true {
		t.Errorf("IncludeUntracked: got %v, want true", intent.IncludeUntracked)
	}
	if len(intent.Filter) != 1 || intent.Filter[0] != "src/" {
		t.Errorf("Filter: got %v, want [\"src/\"]", intent.Filter)
	}
}

// TestOllamaAdapter_DelegatesInterpretGitOp verifies that InterpretGitOp
// delegates to the standard adapter.
func TestOllamaAdapter_DelegatesInterpretGitOp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"role": "assistant", "content": `{"branch": "feat/login"}`}},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	args, err := adapter.InterpretGitOp("branch_create", "create login branch", nil)
	if err != nil {
		t.Fatalf("InterpretGitOp failed: %v", err)
	}
	if args["branch"] != "feat/login" {
		t.Errorf("branch: got %q, want %q", args["branch"], "feat/login")
	}
}

// TestOllamaAdapter_DelegatesVerifySecrets verifies that VerifySecrets
// delegates to the standard adapter.
func TestOllamaAdapter_DelegatesVerifySecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"role": "assistant", "content": "YES"}},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	findings := []domain.SecretDetection{{File: "config.go", Line: 10, Type: "api_key", Content: "AKIA..."}}
	yes, err := adapter.VerifySecrets("diff", findings)
	if err != nil {
		t.Fatalf("VerifySecrets failed: %v", err)
	}
	if !yes {
		t.Error("VerifySecrets: got false, want true")
	}
}

// TestOllamaAdapter_DelegatesAuditBinaryContent verifies that AuditBinaryContent
// delegates to the standard adapter.
func TestOllamaAdapter_DelegatesAuditBinaryContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"role": "assistant", "content": "BINARY"}},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	isBinary, err := adapter.AuditBinaryContent("image.png", "binary content")
	if err != nil {
		t.Fatalf("AuditBinaryContent failed: %v", err)
	}
	if !isBinary {
		t.Error("AuditBinaryContent: got false, want true")
	}
}

// TestOllamaAdapter_DelegatesGenerateChangelog verifies that GenerateChangelog
// delegates to the standard adapter.
func TestOllamaAdapter_DelegatesGenerateChangelog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"role": "assistant", "content": "## Changelog\n\n- feat: new feature"}},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	changelog, err := adapter.GenerateChangelog("abc123", "", "")
	if err != nil {
		t.Fatalf("GenerateChangelog failed: %v", err)
	}
	if !strings.Contains(changelog, "Changelog") {
		t.Errorf("changelog: got %q, want to contain 'Changelog'", changelog)
	}
}

// TestOllamaAdapter_DelegatesRegenerateMessage verifies that RegenerateMessage
// delegates to the standard adapter.
func TestOllamaAdapter_DelegatesRegenerateMessage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/v1/completions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"text": strings.TrimSpace(string(rune('a' + callCount - 1)))},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
	}
	msgs, err := adapter.RegenerateMessage([]string{"old1", "old2"}, "feedback", chunks)
	if err != nil {
		t.Fatalf("RegenerateMessage failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages: got %d, want 2", len(msgs))
	}
}

// TestOllamaAdapter_DelegatesIsAvailable verifies that IsAvailable
// delegates to the standard adapter.
func TestOllamaAdapter_DelegatesIsAvailable(t *testing.T) {
	server := newAdapterServer(t)
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	if !adapter.IsAvailable() {
		t.Error("IsAvailable: got false, want true (delegates to standard adapter)")
	}
}

// TestOllamaAdapter_SetClearRetryContext verifies that retry context is stored and cleared.
func TestOllamaAdapter_SetClearRetryContext(t *testing.T) {
	adapter := NewOllamaAdapter("http://localhost:11434", "test-model", "", false)

	adapter.SetRetryContext("previous message")
	if adapter.standard.RetryContext() != "previous message" {
		t.Errorf("retryContext after Set: got %q, want %q", adapter.standard.RetryContext(), "previous message")
	}

	adapter.ClearRetryContext()
	if adapter.standard.RetryContext() != "" {
		t.Errorf("retryContext after Clear: got %q, want empty", adapter.standard.RetryContext())
	}
}

// TestOllamaAdapter_ResolveModel verifies that ResolveModel calls the resolver.
func TestOllamaAdapter_ResolveModel(t *testing.T) {
	server := newAdapterServer(t)
	defer server.Close()

	adapter := newTestOllamaAdapter(server)
	err := adapter.ResolveModel()
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}
	if adapter.resolver.Model() != "test-model" {
		t.Errorf("Model after ResolveModel: got %q, want %q", adapter.resolver.Model(), "test-model")
	}
}

// TestOllamaAdapter_StandardBaseURL verifies that StandardBaseURL always
// returns host+"/v1" for OpenAI-compatible endpoints.
func TestOllamaAdapter_StandardBaseURL(t *testing.T) {
	adapter := NewOllamaAdapter("http://localhost:11434", "test-model", "", false)

	baseURL := adapter.StandardBaseURL()
	expected := "http://localhost:11434/v1"
	if baseURL != expected {
		t.Errorf("StandardBaseURL() = %q, want %q (always /v1)", baseURL, expected)
	}
}

// TestOllamaAdapter_StandardBaseURL_CustomHost verifies that StandardBaseURL
// uses /v1 regardless of the host.
func TestOllamaAdapter_StandardBaseURL_CustomHost(t *testing.T) {
	adapter := NewOllamaAdapter("http://my-server:11434", "test-model", "", false)

	baseURL := adapter.StandardBaseURL()
	expected := "http://my-server:11434/v1"
	if baseURL != expected {
		t.Errorf("StandardBaseURL() = %q, want %q (always /v1)", baseURL, expected)
	}
}

// TestOllamaAdapter_ImplementsPortsLLM verifies at compile time that
// OllamaAdapter implements the ports.LLM interface.
func TestOllamaAdapter_ImplementsPortsLLM(t *testing.T) {
	var _ ports.LLM = (*OllamaAdapter)(nil)
	t.Log("OllamaAdapter satisfies ports.LLM interface")
}

// TestOllamaAdapter_ImplementsLifecycle verifies at compile time that
// OllamaAdapter satisfies the ports.Lifecycle interface.
func TestOllamaAdapter_ImplementsLifecycle(t *testing.T) {
	var _ ports.Lifecycle = (*OllamaAdapter)(nil)
	t.Log("OllamaAdapter satisfies ports.Lifecycle interface")
}

// TestNewOllamaAdapter verifies that NewOllamaAdapter creates a properly
// configured adapter with all sub-components.
func TestNewOllamaAdapter(t *testing.T) {
	adapter := NewOllamaAdapter("http://localhost:11434", "gemma4:26b", "/models", true)
	if adapter.standard == nil {
		t.Error("standard adapter should not be nil")
	}
	if adapter.lifecycle == nil {
		t.Error("lifecycle should not be nil")
	}
	if adapter.resolver == nil {
		t.Error("resolver should not be nil")
	}
}