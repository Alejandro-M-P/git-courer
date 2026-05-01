package openai_standard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Do_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		resp := ChatResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message ChatMessage `json:"message"`
		}{Message: ChatMessage{Role: "assistant", Content: "hello"}})
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	body, err := client.Do(context.Background(), "POST", "/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("response unmarshal failed: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices: got %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "hello" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "hello")
	}
}

func TestClient_Do_RetryOn429(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(429)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, WithMaxRetries(2), WithRetryWait([]time.Duration{10 * time.Millisecond}))
	body, err := client.Do(context.Background(), "POST", "/v1/test", nil)
	if err != nil {
		t.Fatalf("Do with retry on 429 failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("call count: got %d, want 2 (1 retry)", callCount)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body: got %q, want %q", string(body), `{"ok":true}`)
	}
}

func TestClient_Do_RetryOn500(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(500)
			w.Write([]byte(`{"error":"internal"}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, WithMaxRetries(2), WithRetryWait([]time.Duration{10 * time.Millisecond}))
	body, err := client.Do(context.Background(), "POST", "/v1/test", nil)
	if err != nil {
		t.Fatalf("Do with retry on 500 failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("call count: got %d, want 2", callCount)
	}
	if string(body) != `{"result":"ok"}` {
		t.Errorf("body: got %q, want %q", string(body), `{"result":"ok"}`)
	}
}

func TestClient_Do_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response by sleeping slightly longer than our context deadline
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Use a context with a short deadline to trigger timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := NewClient(server.URL, WithMaxRetries(1), WithRetryWait([]time.Duration{1 * time.Millisecond}))
	_, err := client.Do(ctx, "POST", "/v1/test", nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestClient_Do_RetryExhausted(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"always failing"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, WithMaxRetries(3), WithRetryWait([]time.Duration{10 * time.Millisecond}))
	_, err := client.Do(context.Background(), "POST", "/v1/test", nil)
	if err == nil {
		t.Fatal("expected error after retries exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention 500, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("call count: got %d, want 3 (all retries)", callCount)
	}
}

func TestClient_Do_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	client := NewClient("http://invalid-host-that-does-not-exist.local:99999", WithMaxRetries(1), WithRetryWait([]time.Duration{10 * time.Millisecond}))
	_, err := client.Do(ctx, "POST", "/v1/test", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req ChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("failed to parse request body: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("model: got %q, want %q", req.Model, "test-model")
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"response"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	body, err := client.Post(context.Background(), "/v1/chat/completions", ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "test"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	if len(body) == 0 {
		t.Error("Post returned empty body")
	}
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	body, err := client.Get(context.Background(), "/v1/models")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(body) != `{"models":[]}` {
		t.Errorf("Get body: got %q, want %q", string(body), `{"models":[]}`)
	}
}

func TestClient_Do_WithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	client := NewClient(server.URL)
	_, err := client.Do(ctx, "POST", "/v1/test", nil)
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") {
		t.Logf("error type: %v (may be wrapped)", err)
	}
}

func TestClient_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key-123" {
			t.Errorf("Authorization header: got %q, want %q", auth, "Bearer test-key-123")
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-key-123"))
	_, err := client.Do(context.Background(), "GET", "/v1/models", nil)
	if err != nil {
		t.Fatalf("Do with API key failed: %v", err)
	}
}
