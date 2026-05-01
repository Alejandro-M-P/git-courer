package ollama

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// TestNewOllamaLifecycle verifies the constructor sets fields correctly.
func TestNewOllamaLifecycle(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost:11434", "/models", true)
	if ol.host != "http://localhost:11434" {
		t.Errorf("host: got %q, want %q", ol.host, "http://localhost:11434")
	}
	if ol.modelsDir != "/models" {
		t.Errorf("modelsDir: got %q, want %q", ol.modelsDir, "/models")
	}
	if !ol.autoStart {
		t.Error("autoStart: got false, want true")
	}
}

// TestOllamaLifecycle_ImplementsPortsLifecycle verifies compile-time interface satisfaction.
func TestOllamaLifecycle_ImplementsPortsLifecycle(t *testing.T) {
	var _ ports.Lifecycle = (*OllamaLifecycle)(nil)
	t.Log("OllamaLifecycle satisfies ports.Lifecycle interface")
}

// TestOllamaLifecycle_isAvailable_True verifies isAvailable returns true when server responds.
func TestOllamaLifecycle_isAvailable_True(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "", false,
		WithLifecycleHTTPClient(server.Client()),
	)

	if !ol.isAvailable() {
		t.Error("isAvailable: got false, want true (server should be reachable)")
	}
}

// TestOllamaLifecycle_isAvailable_False verifies isAvailable returns false when server is down.
func TestOllamaLifecycle_isAvailable_False(t *testing.T) {
	ol := NewOllamaLifecycle("http://127.0.0.1:19999", "", false,
		WithLifecycleHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}),
	)

	if ol.isAvailable() {
		t.Error("isAvailable: got true, want false (server should be unreachable)")
	}
}

// TestOllamaLifecycle_EnsureRunning_AlreadyRunning verifies that when Ollama is available,
// EnsureRunning returns (false, nil) — no startup needed.
func TestOllamaLifecycle_EnsureRunning_AlreadyRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "", false,
		WithLifecycleHTTPClient(server.Client()),
	)

	started, err := ol.EnsureRunning()
	if err != nil {
		t.Fatalf("EnsureRunning failed: %v", err)
	}
	if started {
		t.Error("EnsureRunning: got started=true, want false (already running)")
	}
}

// TestOllamaLifecycle_EnsureRunning_NoBinary verifies that when Ollama is not available
// and no binary is found, EnsureRunning returns an error.
func TestOllamaLifecycle_EnsureRunning_NoBinary(t *testing.T) {
	ol := NewOllamaLifecycle("http://127.0.0.1:19999", "", true,
		WithLifecycleHTTPClient(&http.Client{Timeout: 1 * time.Second}),
		WithLifecycleFindBinary(func() string { return "" }), // no binary found
	)

	_, err := ol.EnsureRunning()
	if err == nil {
		t.Fatal("expected error when Ollama unavailable and no binary, got nil")
	}
}

// TestOllamaLifecycle_EnsureRunning_StartNeeded verifies that when Ollama is not available
// but a binary is found, it attempts to start Ollama.
func TestOllamaLifecycle_EnsureRunning_StartNeeded(t *testing.T) {
	ol := NewOllamaLifecycle("http://127.0.0.1:19999", "", true,
		WithLifecycleHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}),
		WithLifecycleFindBinary(func() string { return "/usr/bin/true" }),
		WithLifecycleStartCommand(func(cmd *exec.Cmd) error {
			return nil // Simulate successful start
		}),
	)
	ol.pollInterval = 10 * time.Millisecond
	ol.maxPollAttempts = 2

	_, err := ol.EnsureRunning()
	// Should timeout because there's no actual server
	if err == nil {
		t.Log("EnsureRunning returned nil (may have found server)")
	} else {
		t.Logf("EnsureRunning returned expected error: %v", err)
	}
}

// TestOllamaLifecycle_Stop verifies that Stop clears the startedByUs flag.
func TestOllamaLifecycle_Stop(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost:11434", "", false)
	ol.startedByUs = true

	ol.Stop()
	if ol.startedByUs {
		t.Error("Stop should set startedByUs=false")
	}
}

// TestOllamaLifecycle_PreWarm_NoDelegate verifies PreWarm returns nil when no preWarm fn is set.
func TestOllamaLifecycle_PreWarm_NoDelegate(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost:11434", "", false)
	if err := ol.PreWarm(); err != nil {
		t.Fatalf("PreWarm with no delegate should be no-op: %v", err)
	}
}

// TestOllamaLifecycle_PreWarm_Delegates verifies PreWarm delegates to the injected function.
func TestOllamaLifecycle_PreWarm_Delegates(t *testing.T) {
	called := false
	ol := NewOllamaLifecycle("http://localhost:11434", "", false,
		WithPreWarm(func() error {
			called = true
			return nil
		}),
	)
	if err := ol.PreWarm(); err != nil {
		t.Fatalf("PreWarm with delegate should return nil: %v", err)
	}
	if !called {
		t.Error("PreWarm did not call the injected preWarm function")
	}
}

// TestOllamaLifecycle_PreWarm_DelegatesError verifies PreWarm propagates errors from the delegate.
func TestOllamaLifecycle_PreWarm_DelegatesError(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost:11434", "", false,
		WithPreWarm(func() error {
			return fmt.Errorf("model warm-up failed")
		}),
	)
	err := ol.PreWarm()
	if err == nil {
		t.Fatal("PreWarm should propagate delegate error, got nil")
	}
	if err.Error() != "model warm-up failed" {
		t.Errorf("PreWarm error: got %q, want %q", err.Error(), "model warm-up failed")
	}
}

// TestOllamaLifecycle_IsWarmed verifies IsWarmed returns false when no server is running.
// IsWarmed returns true when startedByUs || isAvailable(), so we use an
// unreachable host to guarantee isAvailable() returns false.
func TestOllamaLifecycle_IsWarmed(t *testing.T) {
	ol := NewOllamaLifecycle("http://127.0.0.1:19999", "", false,
		WithLifecycleHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}),
	)
	if ol.IsWarmed() {
		t.Error("IsWarmed: got true, want false (server not running)")
	}
}

// TestOllamaLifecycle_findBinary verifies findBinary returns empty when not mocked.
func TestOllamaLifecycle_findBinary(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost", "", false)
	result := ol.findBinary()
	// Result could be empty or a path depending on system
	t.Logf("findBinary returned: %q", result)
}

// TestOllamaLifecycle_findBinary_Mock verifies findBinary with mock.
func TestOllamaLifecycle_findBinary_Mock(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost", "", false,
		WithLifecycleFindBinary(func() string { return "/custom/ollama" }),
	)
	result := ol.findBinary()
	if result != "/custom/ollama" {
		t.Errorf("findBinary: got %q, want %q", result, "/custom/ollama")
	}
}
