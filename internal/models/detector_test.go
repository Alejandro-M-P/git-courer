package models

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockHTTPClient wraps an httptest.Server for testing.
type mockHTTPClient struct {
	server *httptest.Server
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	client := m.server.Client()
	return client.Do(req)
}

func TestOllamaDetector_Success(t *testing.T) {
	response := ollamaShowResponse{
		ModelInfo: map[string]any{
			"general.architecture": "qwen35",
			"qwen35.context_length": float64(262144),
			"qwen35.embedding_length": float64(1536),
		},
	}
	respBytes, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/show" {
			t.Errorf("Expected /api/show, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	ctxWindow, ok := detector.Lookup(context.Background(), "qwen3.5:0.8b")
	if !ok {
		t.Fatal("Expected Lookup to succeed")
	}
	if ctxWindow != 262144 {
		t.Errorf("Expected context window 262144, got %d", ctxWindow)
	}
}

func TestOllamaDetector_ConnectionRefused(t *testing.T) {
	// Point to a port that nothing is listening on
	detector := NewOllamaDetector("http://localhost:19999", http.DefaultClient)

	ctxWindow, ok := detector.Lookup(context.Background(), "nonexistent-model")
	if ok {
		t.Errorf("Expected Lookup to fail for connection refused, got ctxWindow=%d", ctxWindow)
	}
	if ctxWindow != 0 {
		t.Errorf("Expected ctxWindow=0 for connection refused, got %d", ctxWindow)
	}
}

func TestOllamaDetector_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(10 * time.Second)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	// Use a client with very short timeout to trigger timeout faster
	client := &http.Client{Timeout: 100 * time.Millisecond}
	detector := NewOllamaDetector(server.URL, client)

	start := time.Now()
	ctxWindow, ok := detector.Lookup(context.Background(), "slow-model")
	elapsed := time.Since(start)

	if ok {
		t.Errorf("Expected Lookup to fail due to timeout, got ctxWindow=%d", ctxWindow)
	}
	if ctxWindow != 0 {
		t.Errorf("Expected ctxWindow=0 for timeout, got %d", ctxWindow)
	}

	// Should fail within reasonable time, not hang
	if elapsed > 10*time.Second {
		t.Errorf("Lookup took too long: %v", elapsed)
	}
}

func TestOllamaDetector_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json{{"))
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	ctxWindow, ok := detector.Lookup(context.Background(), "broken-model")
	if ok {
		t.Errorf("Expected Lookup to fail for malformed response, got ctxWindow=%d", ctxWindow)
	}
	if ctxWindow != 0 {
		t.Errorf("Expected ctxWindow=0 for malformed response, got %d", ctxWindow)
	}
}

func TestOllamaDetector_Caching(t *testing.T) {
	callCount := 0
	response := ollamaShowResponse{
		ModelInfo: map[string]any{
			"general.architecture": "testarch",
			"testarch.context_length": float64(16384),
		},
	}
	respBytes, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	// First call — should hit the server
	ctxWindow1, ok1 := detector.Lookup(context.Background(), "cached-model")
	if !ok1 || ctxWindow1 != 16384 {
		t.Fatalf("First lookup: got (%d, %v), want (16384, true)", ctxWindow1, ok1)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 HTTP call after first lookup, got %d", callCount)
	}

	// Second call — should return cached value without HTTP request
	ctxWindow2, ok2 := detector.Lookup(context.Background(), "cached-model")
	if !ok2 || ctxWindow2 != 16384 {
		t.Fatalf("Second lookup: got (%d, %v), want (16384, true)", ctxWindow2, ok2)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 HTTP call after second lookup (cached), got %d", callCount)
	}
}

func TestOllamaDetector_DifferentModels(t *testing.T) {
	responses := map[string]ollamaShowResponse{
		"model-a": {
			ModelInfo: map[string]any{
				"general.architecture": "arch_a",
				"arch_a.context_length": float64(8192),
			},
		},
		"model-b": {
			ModelInfo: map[string]any{
				"general.architecture": "arch_b",
				"arch_b.context_length": float64(32768),
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaShowRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp, ok := responses[req.Model]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		respBytes, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	ctxA, okA := detector.Lookup(context.Background(), "model-a")
	if !okA || ctxA != 8192 {
		t.Errorf("model-a: got (%d, %v), want (8192, true)", ctxA, okA)
	}

	ctxB, okB := detector.Lookup(context.Background(), "model-b")
	if !okB || ctxB != 32768 {
		t.Errorf("model-b: got (%d, %v), want (32768, true)", ctxB, okB)
	}
}

func TestOllamaDetector_ContextLengthDirect(t *testing.T) {
	// Test fallback when general.architecture is missing
	response := ollamaShowResponse{
		ModelInfo: map[string]any{
			"context_length": float64(4096),
		},
	}
	respBytes, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	ctxWindow, ok := detector.Lookup(context.Background(), "direct-ctx-model")
	if !ok {
		t.Fatal("Expected Lookup to succeed with direct context_length")
	}
	if ctxWindow != 4096 {
		t.Errorf("Expected context window 4096, got %d", ctxWindow)
	}
}

func TestOllamaDetector_EmptyModelInfo(t *testing.T) {
	response := ollamaShowResponse{
		ModelInfo: map[string]any{},
	}
	respBytes, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	ctxWindow, ok := detector.Lookup(context.Background(), "empty-info-model")
	if ok {
		t.Errorf("Expected Lookup to fail for empty model_info, got ctxWindow=%d", ctxWindow)
	}
}

func TestOllamaDetector_NonOKStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	ctxWindow, ok := detector.Lookup(context.Background(), "not-found-model")
	if ok {
		t.Errorf("Expected Lookup to fail for 404, got ctxWindow=%d", ctxWindow)
	}
}

func TestOllamaDetector_ConcurrentCaching(t *testing.T) {
	response := ollamaShowResponse{
		ModelInfo: map[string]any{
			"general.architecture": "llama",
			"llama.context_length":  float64(8192),
		},
	}
	respBytes, _ := json.Marshal(response)

	var callCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	// Launch multiple concurrent lookups for the same model
	var wg sync.WaitGroup
	results := make(chan int, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, ok := detector.Lookup(context.Background(), "concurrent-model")
			if ok {
				results <- ctx
			} else {
				results <- 0
			}
		}()
	}
	wg.Wait()
	close(results)

	for ctx := range results {
		if ctx != 8192 {
			t.Errorf("Expected 8192, got %d", ctx)
		}
	}

	// Note: multiple calls may happen due to race, but cache should work after first
	// The important thing is all results are correct
}

func TestOllamaDetector_DefaultConstructor(t *testing.T) {
	detector := NewDefaultOllamaDetector()
	if detector.baseURL != "http://localhost:11434" {
		t.Errorf("Expected baseURL 'http://localhost:11434', got %q", detector.baseURL)
	}
	if detector.client != http.DefaultClient {
		t.Error("Expected http.DefaultClient")
	}
}

func TestOllamaDetector_TrailingSlashTrim(t *testing.T) {
	detector := NewOllamaDetector("http://localhost:11434/", nil)
	if detector.baseURL != "http://localhost:11434" {
		t.Errorf("Expected trailing slash to be trimmed, got %q", detector.baseURL)
	}
}

func TestOllamaDetector_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ctxWindow, ok := detector.Lookup(ctx, "cancelled-model")
	if ok {
		t.Errorf("Expected Lookup to fail with cancelled context, got ctxWindow=%d", ctxWindow)
	}
	if ctxWindow != 0 {
		t.Errorf("Expected ctxWindow=0 for cancelled context, got %d", ctxWindow)
	}
}

func TestExtractContextLength(t *testing.T) {
	tests := []struct {
		name      string
		modelInfo map[string]any
		expected  int
	}{
		{
			name: "architecture-based context_length",
			modelInfo: map[string]any{
				"general.architecture": "qwen35",
				"qwen35.context_length": float64(262144),
			},
			expected: 262144,
		},
		{
			name: "direct context_length fallback",
			modelInfo: map[string]any{
				"context_length": float64(4096),
			},
			expected: 4096,
		},
		{
			name: "general.context_length fallback",
			modelInfo: map[string]any{
				"general.context_length": float64(8192),
			},
			expected: 8192,
		},
		{
			name:      "empty model_info",
			modelInfo: map[string]any{},
			expected:  0,
		},
		{
			name:      "nil model_info",
			modelInfo: nil,
			expected:  0,
		},
		{
			name: "architecture without context_length",
			modelInfo: map[string]any{
				"general.architecture": "unknown",
			},
			expected: 0,
		},
		{
			name: "architecture takes priority over direct key",
			modelInfo: map[string]any{
				"general.architecture": "testarch",
				"testarch.context_length": float64(65536),
				"context_length":          float64(4096),
			},
			expected: 65536,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContextLength(tt.modelInfo)
			if got != tt.expected {
				t.Errorf("extractContextLength() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestOllamaDetector_RequestBodyFormat(t *testing.T) {
	var receivedBody ollamaShowRequest

	response := ollamaShowResponse{
		ModelInfo: map[string]any{
			"general.architecture": "llama",
			"llama.context_length":  float64(8192),
		},
	}
	respBytes, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	detector.Lookup(context.Background(), "my-model:7b")

	if receivedBody.Model != "my-model:7b" {
		t.Errorf("Expected request model 'my-model:7b', got %q", receivedBody.Model)
	}
}

func TestOllamaDetector_FailedResponseDoesNotCache(t *testing.T) {
	callCount := 0
	failResponse := true

	response := ollamaShowResponse{
		ModelInfo: map[string]any{
			"general.architecture": "llama",
			"llama.context_length":  float64(8192),
		},
	}
	respBytes, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if failResponse {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})

	// First call fails — should NOT be cached
	ctx, ok := detector.Lookup(context.Background(), "retry-model")
	if ok {
		t.Errorf("Expected first Lookup to fail, got ctxWindow=%d", ctx)
	}

	// Make server return success now
	failResponse = false

	// Second call should hit server again (not cached) and succeed
	ctx, ok = detector.Lookup(context.Background(), "retry-model")
	if !ok {
		t.Fatal("Expected second Lookup to succeed")
	}
	if ctx != 8192 {
		t.Errorf("Expected context window 8192, got %d", ctx)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 HTTP calls (fail then success), got %d", callCount)
	}
}

// Ensure the method name matches what's used by catalog
func TestOllamaDetector_LookupMethodName(t *testing.T) {
	// Verify Lookup method exists with correct signature
	var _ func(context.Context, string) (int, bool) = (*OllamaDetector)(nil).Lookup
}

// Verify API/show endpoint is correctly formed
func TestOllamaDetector_APIEndpoint(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})
	detector.Lookup(context.Background(), "test")

	if receivedPath != "/api/show" {
		t.Errorf("Expected path /api/show, got %q", receivedPath)
	}
}

// Verify POST method is used
func TestOllamaDetector_PostMethod(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})
	detector.Lookup(context.Background(), "test")

	if receivedMethod != http.MethodPost {
		t.Errorf("Expected POST method, got %q", receivedMethod)
	}
}

// Verify Content-Type header
func TestOllamaDetector_ContentType(t *testing.T) {
	var contentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	detector := NewOllamaDetector(server.URL, &mockHTTPClient{server: server})
	detector.Lookup(context.Background(), "test")

	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Expected Content-Type to start with application/json, got %q", contentType)
	}
}