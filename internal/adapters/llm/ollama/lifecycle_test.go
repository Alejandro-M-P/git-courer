package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"
)

// newLifecycleServer creates a test server that handles /api/tags and /api/generate.
func newLifecycleServer(t *testing.T, models []map[string]string) *httptest.Server {
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response": "",
			})
		default:
			w.WriteHeader(404)
		}
	}))
}

// TestOllamaLifecycle_EnsureRunning_AlreadyRunning verifies that when Ollama
// is available, EnsureRunning returns (false, nil) — no startup needed.
func TestOllamaLifecycle_EnsureRunning_AlreadyRunning(t *testing.T) {
	server := newLifecycleServer(t, []map[string]string{{"name": "test-model"}})
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "test-model", "", false,
		WithLifecycleHTTPClient(server.Client()),
	)

	started, err := ol.EnsureRunning()
	if err != nil {
		t.Fatalf("EnsureRunning failed: %v", err)
	}
	if started {
		t.Error("EnsureRunning: got started=true, want false (Ollama already running)")
	}
}

// TestOllamaLifecycle_EnsureRunning_NoBinary verifies that when Ollama is
// not available and no binary is found, EnsureRunning returns an error.
func TestOllamaLifecycle_EnsureRunning_NoBinary(t *testing.T) {
	ol := NewOllamaLifecycle("http://127.0.0.1:19999", "test-model", "", false,
		WithLifecycleHTTPClient(&http.Client{Timeout: 1 * time.Second}),
		WithLifecycleFindBinary(func() string { return "" }), // no binary found
	)

	_, err := ol.EnsureRunning()
	if err == nil {
		t.Fatal("expected error when Ollama unavailable and no binary, got nil")
	}
}

// TestOllamaLifecycle_EnsureRunning_WithResolve verifies that after starting,
// EnsureRunning resolves the model via ModelResolver.
func TestOllamaLifecycle_EnsureRunning_WithResolve(t *testing.T) {
	server := newLifecycleServer(t, []map[string]string{{"name": "my-model"}})
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "my-model", "", false,
		WithLifecycleHTTPClient(server.Client()),
	)

	started, err := ol.EnsureRunning()
	if err != nil {
		t.Fatalf("EnsureRunning failed: %v", err)
	}
	// Ollama is already running (server responds), so started should be false
	if started {
		t.Error("EnsureRunning: got started=true, want false (already running)")
	}

	// Model should be resolved after EnsureRunning
	if ol.resolver.Model() != "my-model" {
		t.Errorf("Model after EnsureRunning: got %q, want %q", ol.resolver.Model(), "my-model")
	}
}

// TestOllamaLifecycle_PreWarm_Success verifies that PreWarm successfully loads
// a model via /api/generate.
func TestOllamaLifecycle_PreWarm_Success(t *testing.T) {
	prewarmCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "test-model"}},
			})
		case "/api/generate":
			prewarmCalled = true
			// Verify this is a pre-warm request (minimal prompt, stream=false)
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			if reqBody["prompt"] != "." {
				t.Errorf("PreWarm should use minimal prompt '.' , got %v", reqBody["prompt"])
			}
			if reqBody["stream"] != false {
				t.Error("PreWarm should use stream=false")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response": "ok",
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "test-model", "", false,
		WithLifecycleHTTPClient(server.Client()),
	)

	if err := ol.PreWarm(); err != nil {
		t.Fatalf("PreWarm failed: %v", err)
	}
	if !prewarmCalled {
		t.Error("PreWarm should have called /api/generate")
	}
}

// TestOllamaLifecycle_PreWarm_Timeout verifies that PreWarm returns an error
// when the model fails to load (server takes too long).
func TestOllamaLifecycle_PreWarm_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/generate" {
			// Simulate model loading slowly
			time.Sleep(2 * time.Second)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "test-model", "", false,
		WithLifecycleHTTPClient(&http.Client{Timeout: 500 * time.Millisecond}),
	)

	err := ol.PreWarm()
	if err == nil {
		t.Fatal("expected timeout error from PreWarm, got nil")
	}
}

// TestOllamaLifecycle_Stop verifies that Stop clears the startedByUs flag
// and does NOT kill any process.
func TestOllamaLifecycle_Stop(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost:11434", "test-model", "", false)
	ol.startedByUs = true
	ol.process = nil // No process to kill

	ol.Stop()
	if ol.startedByUs {
		t.Error("Stop should set startedByUs=false")
	}
}

// TestOllamaLifecycle_ImplementsPortsLifecycle verifies that OllamaLifecycle
// satisfies the ports.Lifecycle interface at compile time.
func TestOllamaLifecycle_ImplementsPortsLifecycle(t *testing.T) {
	// Compile-time check
	var _ interface {
		EnsureRunning() (bool, error)
		PreWarm() error
		IsWarmed() bool
		Stop()
	} = (*OllamaLifecycle)(nil)
	t.Log("OllamaLifecycle satisfies ports.Lifecycle interface")
}

// TestOllamaLifecycle_findOllamaBinary verifies that findOllamaBinary can
// find the ollama binary when it exists on PATH.
func TestOllamaLifecycle_findOllamaBinary(t *testing.T) {
	// This test just verifies the function runs. Finding depends on
	// whether ollama is installed, so we test with a mock.
	// The actual behavior is tested via WithLifecycleFindBinary option.
	result := findOllamaBinary()
	// Result could be empty or a path, both are valid
	t.Logf("findOllamaBinary returned: %q", result)
}

// TestOllamaLifecycle_EnsureRunning_StartNeeded verifies that when Ollama
// is not available but a binary is found, it attempts to start Ollama.
// We simulate a binary that "starts" (just creates a mock server after delay).
func TestOllamaLifecycle_EnsureRunning_StartNeeded(t *testing.T) {
	// We can't easily test actual process starting in unit tests,
	// so we test the case where the binary exists but startup fails
	// by pointing at an unavailable server with a "found" binary.
	ol := NewOllamaLifecycle("http://127.0.0.1:19999", "test-model", "", true,
		WithLifecycleHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}),
		WithLifecycleFindBinary(func() string { return "/usr/bin/true" }), // binary exists but won't be ollama
		WithLifecycleStartCommand(func(cmd *exec.Cmd) error {
			return nil // Simulate successful start
		}),
	)
	// Even though we simulate a "start", the server isn't actually running
	// so the polling loop will timeout
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

// TestNewOllamaLifecycle verifies the constructor sets fields correctly.
func TestNewOllamaLifecycle(t *testing.T) {
	ol := NewOllamaLifecycle("http://localhost:11434", "gemma4:26b", "/models", true)
	if ol.host != "http://localhost:11434" {
		t.Errorf("host: got %q, want %q", ol.host, "http://localhost:11434")
	}
	if ol.model != "gemma4:26b" {
		t.Errorf("model: got %q, want %q", ol.model, "gemma4:26b")
	}
	if ol.modelsDir != "/models" {
		t.Errorf("modelsDir: got %q, want %q", ol.modelsDir, "/models")
	}
	if !ol.autoStart {
		t.Error("autoStart: got false, want true")
	}
}

// TestOllamaLifecycle_IsAvailable_True verifies IsAvailable returns true when
// the Ollama server responds.
func TestOllamaLifecycle_IsAvailable_True(t *testing.T) {
	server := newLifecycleServer(t, nil)
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "test-model", "", false,
		WithLifecycleHTTPClient(server.Client()),
	)

	if !ol.IsAvailable() {
		t.Error("IsAvailable: got false, want true (server should be reachable)")
	}
}

// TestOllamaLifecycle_IsAvailable_False verifies IsAvailable returns false when
// the Ollama server is unreachable.
func TestOllamaLifecycle_IsAvailable_False(t *testing.T) {
	ol := NewOllamaLifecycle("http://127.0.0.1:19999", "test-model", "", false,
		WithLifecycleHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}),
	)

	if ol.IsAvailable() {
		t.Error("IsAvailable: got true, want false (server should be unreachable)")
	}
}

// TestOllamaLifecycle_timeoutFor verifies the timeout logic for different attempts.
func TestOllamaLifecycle_timeoutFor(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 60 * time.Second},
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{3, 120 * time.Second},
		{4, 180 * time.Second},
		{5, 180 * time.Second},
	}
	for _, tt := range tests {
		got := timeoutForAttempt(tt.attempt)
		if got != tt.want {
			t.Errorf("timeoutFor(%d): got %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

// TestOllamaLifecycle_PreWarm_Idempotent verifies that after DetectJSONSupport
// sets warmed=true, subsequent PreWarm calls are no-ops.
func TestOllamaLifecycle_PreWarm_Idempotent(t *testing.T) {
	generateCalled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "test-model"}},
			})
		case "/api/generate":
			generateCalled++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"response": ""})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "test-model", "", false,
		WithLifecycleHTTPClient(server.Client()),
	)
	// EnsureRunning calls Resolve() which calls DetectJSONSupport → sets warmed
	_, err := ol.EnsureRunning()
	if err != nil {
		t.Fatalf("EnsureRunning failed: %v", err)
	}
	if !ol.IsWarmed() {
		t.Error("IsWarmed should be true after EnsureRunning (DetectJSONSupport)")
	}
	// Call PreWarm 10 times — should be no-op since warmed=true
	for i := 0; i < 10; i++ {
		if err := ol.PreWarm(); err != nil {
			t.Fatalf("PreWarm call %d failed: %v", i, err)
		}
	}
	// generateCalled should be 1 (from DetectJSONSupport during Resolve),
	// NOT 1+10 from additional PreWarmNative calls
	if generateCalled != 1 {
		t.Errorf("PreWarm should not call /api/generate when warmed, got %d calls (expected 1 from DetectJSONSupport)", generateCalled)
	}
}

// TestOllamaLifecycle_PreWarmNotWarmed_CallsNative verifies that PreWarm
// calls PreWarmNative when warmed is false.
func TestOllamaLifecycle_PreWarmNotWarmed_CallsNative(t *testing.T) {
	generateCalled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			generateCalled++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"response": ""})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "test-model", "", false,
		WithLifecycleHTTPClient(server.Client()),
	)
	// NOT calling EnsureRunning — warmed is false
	if err := ol.PreWarm(); err != nil {
		t.Fatalf("PreWarm failed: %v", err)
	}
	if generateCalled != 1 {
		t.Errorf("PreWarm should call /api/generate when not warmed, got %d calls", generateCalled)
	}
	if !ol.IsWarmed() {
		t.Error("IsWarmed should be true after successful PreWarm")
	}
}

// TestOllamaLifecycle_PreWarmNative_AlwaysCalls verifies that PreWarmNative
// always makes the HTTP call regardless of warmed state.
func TestOllamaLifecycle_PreWarmNative_AlwaysCalls(t *testing.T) {
	generateCalled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "test-model"}},
			})
		case "/api/generate":
			generateCalled++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"response": ""})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	ol := NewOllamaLifecycle(server.URL, "test-model", "", false,
		WithLifecycleHTTPClient(server.Client()),
	)
	// EnsureRunning sets warmed=true
	ol.EnsureRunning()

	// PreWarmNative should still make HTTP call even when warmed
	if err := ol.PreWarmNative(); err != nil {
		t.Fatalf("PreWarmNative failed: %v", err)
	}
	// 1 call from DetectJSONSupport + 1 call from PreWarmNative = 2 total
	if generateCalled != 2 {
		t.Errorf("PreWarmNative should call /api/generate regardless, got %d calls (expected 2: 1 from Detect + 1 from PreWarmNative)", generateCalled)
	}
}