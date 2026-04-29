package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTagsServer creates a test server that handles both /api/tags and /api/generate.
func newTagsServer(t *testing.T, models []map[string]string, jsonSupport bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": models,
			})
		case "/api/generate":
			w.Header().Set("Content-Type", "application/json")
			if jsonSupport {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"response": `{"ok": true}`,
				})
			} else {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"response": "This is not JSON at all",
				})
			}
		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

// TestModelResolver_Resolve_ModelFound verifies that when the configured model
// is found in /api/tags, Resolve succeeds and the model name is preserved.
func TestModelResolver_Resolve_ModelFound(t *testing.T) {
	server := newTagsServer(t, []map[string]string{
		{"name": "gemma4:26b"},
		{"name": "qwen3:8b"},
	}, true)
	defer server.Close()

	mr := NewModelResolver(server.URL, "gemma4:26b")
	if err := mr.Resolve(); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if mr.Model() != "gemma4:26b" {
		t.Errorf("Model: got %q, want %q", mr.Model(), "gemma4:26b")
	}
	if !mr.SupportsJSON {
		t.Error("SupportsJSON: got false, want true")
	}
}

// TestModelResolver_Resolve_ModelFound_WithLatestSuffix verifies that model
// matching works with the ":latest" suffix that Ollama sometimes returns.
func TestModelResolver_Resolve_ModelFound_WithLatestSuffix(t *testing.T) {
	server := newTagsServer(t, []map[string]string{
		{"name": "gemma4:26b:latest"},
	}, true)
	defer server.Close()

	mr := NewModelResolver(server.URL, "gemma4:26b")
	if err := mr.Resolve(); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if mr.Model() != "gemma4:26b:latest" {
		t.Errorf("Model: got %q, want %q", mr.Model(), "gemma4:26b:latest")
	}
}

// TestModelResolver_Resolve_ModelNotFound_Fallback verifies that when the
// configured model is NOT found but other models exist, Resolve falls back
// to the first available model and succeeds.
func TestModelResolver_Resolve_ModelNotFound_Fallback(t *testing.T) {
	server := newTagsServer(t, []map[string]string{
		{"name": "qwen3:8b"},
		{"name": "llama3:7b"},
	}, true)
	defer server.Close()

	mr := NewModelResolver(server.URL, "gemma4:26b")
	if err := mr.Resolve(); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	// Should fall back to first available model
	if mr.Model() != "qwen3:8b" {
		t.Errorf("Model: got %q, want %q (first available fallback)", mr.Model(), "qwen3:8b")
	}
}

// TestModelResolver_Resolve_NoModels verifies that when /api/tags returns an
// empty model list, Resolve returns an error.
func TestModelResolver_Resolve_NoModels(t *testing.T) {
	server := newTagsServer(t, []map[string]string{}, false)
	defer server.Close()

	mr := NewModelResolver(server.URL, "gemma4:26b")
	err := mr.Resolve()
	if err == nil {
		t.Fatal("expected error when no models available, got nil")
	}
	if mr.Model() != "gemma4:26b" {
		t.Errorf("Model should stay as configured: got %q, want %q", mr.Model(), "gemma4:26b")
	}
}

// TestModelResolver_Resolve_OllamaUnavailable verifies that when the Ollama
// server is unreachable, Resolve returns an error.
func TestModelResolver_Resolve_OllamaUnavailable(t *testing.T) {
	mr := NewModelResolver("http://127.0.0.1:19999", "gemma4:26b",
		WithResolverHTTPClient(&http.Client{Timeout: 1 * time.Second}),
	)
	err := mr.Resolve()
	if err == nil {
		t.Fatal("expected error when Ollama unavailable, got nil")
	}
}

// TestModelResolver_DetectJSONSupport_Supported verifies that when the model
// responds with valid JSON to a format:json request, DetectJSONSupport returns true.
func TestModelResolver_DetectJSONSupport_Supported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected /api/generate, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify the request includes format:json
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if format, ok := reqBody["format"]; !ok || format != "json" {
			t.Errorf("expected format=json in request, got %v", reqBody)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": `{"ok": true}`,
		})
	}))
	defer server.Close()

	mr := NewModelResolver(server.URL, "test-model")
	supported := mr.DetectJSONSupport()
	if !supported {
		t.Error("DetectJSONSupport: got false, want true for valid JSON response")
	}
}

// TestModelResolver_DetectJSONSupport_NotSupported verifies that when the model
// responds with invalid JSON to a format:json request, DetectJSONSupport returns false.
func TestModelResolver_DetectJSONSupport_NotSupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": "This is not JSON at all, just plain text",
		})
	}))
	defer server.Close()

	mr := NewModelResolver(server.URL, "test-model")
	supported := mr.DetectJSONSupport()
	if supported {
		t.Error("DetectJSONSupport: got true, want false for invalid JSON response")
	}
}

// TestModelResolver_DetectJSONSupport_EmptyResponse verifies that when the model
// responds with an empty string, DetectJSONSupport returns false.
func TestModelResolver_DetectJSONSupport_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": "",
		})
	}))
	defer server.Close()

	mr := NewModelResolver(server.URL, "test-model")
	supported := mr.DetectJSONSupport()
	if supported {
		t.Error("DetectJSONSupport: got true, want false for empty JSON response")
	}
}

// TestModelResolver_DetectJSONSupport_Timeout verifies that when the model takes
// too long to respond, DetectJSONSupport returns false.
func TestModelResolver_DetectJSONSupport_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(3 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": `{"ok": true}`,
		})
	}))
	defer server.Close()

	mr := NewModelResolver(server.URL, "test-model",
		WithResolverHTTPClient(&http.Client{Timeout: 500 * time.Millisecond}),
	)
	supported := mr.DetectJSONSupport()
	if supported {
		t.Error("DetectJSONSupport: got true, want false on timeout")
	}
}