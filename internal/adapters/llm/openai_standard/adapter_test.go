package openai_standard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Compile-time interface checks: OpenAIStandardAdapter must implement both ports.LLM and ports.Lifecycle.
var _ ports.LLM = (*OpenAIStandardAdapter)(nil)
var _ ports.Lifecycle = (*OpenAIStandardAdapter)(nil)

// newTestAdapter creates an adapter pointing at the given test server.
func newTestAdapter(server *httptest.Server) *OpenAIStandardAdapter {
	return NewOpenAIStandardAdapter(server.URL+"/v1", "test-model",
		WithMaxRetries(1),
		WithRetryWait([]time.Duration{1 * time.Millisecond}),
	)
}

// chatCompletionResponse builds a mock /v1/chat/completions JSON response.
func chatCompletionResponse(content string) []byte {
	resp := ChatResponse{}
	resp.Choices = append(resp.Choices, struct {
		Message ChatMessage `json:"message"`
	}{Message: ChatMessage{Role: "assistant", Content: content}})
	data, _ := json.Marshal(resp)
	return data
}

// completionResponse builds a mock /v1/completions JSON response.
func completionResponse(text string) []byte {
	resp := CompletionResponse{}
	resp.Choices = append(resp.Choices, struct {
		Text string `json:"text"`
	}{Text: text})
	data, _ := json.Marshal(resp)
	return data
}

func TestAdapter_GenerateChunkMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// GenerateChunkMessage uses /completions
		if r.URL.Path != "/v1/completions" {
			t.Errorf("expected /v1/completions, got %s", r.URL.Path)
		}

		// Parse request to verify model
		var req CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("model: got %q, want %q", req.Model, "test-model")
		}
		if req.Stream != false {
			t.Errorf("stream must be false, got %v", req.Stream)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(completionResponse("feat: add new feature"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files: []string{"main.go", "util.go"},
		Diff:  "diff content here",
	}
	msg, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
	if msg != "feat: add new feature" {
		t.Errorf("message: got %q, want %q", msg, "feat: add new feature")
	}
}

func TestAdapter_GenerateChunkMessage_WithRetryContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the prompt includes retry context
		var req CompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !strings.Contains(req.Prompt, "previously rejected") {
			t.Errorf("prompt should contain retry context, got: %q", req.Prompt[:min(200, len(req.Prompt))])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(completionResponse("fix: different message"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetRetryContext("previously rejected message")

	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	msg, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage with retry failed: %v", err)
	}
	if msg != "fix: different message" {
		t.Errorf("message: got %q, want %q", msg, "fix: different message")
	}
}

func TestAdapter_DecideCommit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify request model and format
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "test-model" {
			t.Errorf("model: got %q, want %q", req.Model, "test-model")
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(`{"include_untracked": true, "file_filter": ["src/"]}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	intent, err := adapter.DecideCommit("commit everything", "M file.go", "new.go", "", "")
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

func TestAdapter_DecideCommit_PlainTextResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Some models might not return JSON — fallback extracts filter from "YES, src/"
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("YES, src/"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	intent, err := adapter.DecideCommit("commit all", "M file.go", "new.go", "", "")
	if err != nil {
		t.Fatalf("DecideCommit plain text failed: %v", err)
	}
	if intent.IncludeUntracked != true {
		t.Errorf("IncludeUntracked: got %v, want true (from YES)", intent.IncludeUntracked)
	}
	// Plain text fallback extracts filter pattern using SplitPaths logic
	if len(intent.Filter) != 1 || intent.Filter[0] != "src/" {
		t.Errorf("Filter: got %v, want [\"src/\"]", intent.Filter)
	}
}

func TestAdapter_InterpretGitOp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(`{"branch": "feat/login", "name": "login-feature"}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	args, err := adapter.InterpretGitOp("branch_create", "create login branch", map[string]string{
		"current_branch": "main",
	})
	if err != nil {
		t.Fatalf("InterpretGitOp failed: %v", err)
	}
	if args["branch"] != "feat/login" {
		t.Errorf("branch: got %q, want %q", args["branch"], "feat/login")
	}
	if args["name"] != "login-feature" {
		t.Errorf("name: got %q, want %q", args["name"], "login-feature")
	}
}

func TestAdapter_InterpretGitOp_NonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("not a json object"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	args, err := adapter.InterpretGitOp("branch_create", "create branch", nil)
	if err != nil {
		t.Fatalf("InterpretGitOp non-JSON failed: %v", err)
	}
	// Non-JSON object should return empty map
	if len(args) != 0 {
		t.Errorf("args: got %d entries, want 0 for non-JSON response", len(args))
	}
}

func TestAdapter_SetRetryContext(t *testing.T) {
	adapter := NewOpenAIStandardAdapter("http://localhost:8080/v1", "test-model")

	adapter.SetRetryContext("previous message")
	if adapter.retryContext != "previous message" {
		t.Errorf("retryContext: got %q, want %q", adapter.retryContext, "previous message")
	}
}

func TestAdapter_ClearRetryContext(t *testing.T) {
	adapter := NewOpenAIStandardAdapter("http://localhost:8080/v1", "test-model")
	adapter.SetRetryContext("some message")
	adapter.ClearRetryContext()
	if adapter.retryContext != "" {
		t.Errorf("retryContext after clear: got %q, want empty", adapter.retryContext)
	}
}

func TestAdapter_IsAvailable_True(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("EnsureRunning should request /v1/models, got %s", r.URL.Path)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	adapter := NewOpenAIStandardAdapter(server.URL+"/v1", "test-model",
		WithMaxRetries(1),
		WithRetryWait([]time.Duration{1 * time.Millisecond}),
	)
	if !adapter.IsAvailable() {
		t.Error("IsAvailable should return true when server responds 200")
	}
}

func TestAdapter_IsAvailable_False(t *testing.T) {
	// Use a port that nothing listens on with short context
	adapter := NewOpenAIStandardAdapter("http://127.0.0.1:19999", "test-model",
		WithMaxRetries(1),
		WithRetryWait([]time.Duration{1 * time.Millisecond}),
	)
	if adapter.IsAvailable() {
		t.Error("IsAvailable should return false when server is unreachable")
	}
}

func TestAdapter_VerifySecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("YES"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	findings := []domain.SecretDetection{
		{File: "config.go", Line: 10, Type: "api_key", Content: "AKIA..."},
	}
	yes, err := adapter.VerifySecrets("diff content", findings)
	if err != nil {
		t.Fatalf("VerifySecrets failed: %v", err)
	}
	if !yes {
		t.Error("VerifySecrets: got false, want true for YES response")
	}
}

func TestAdapter_VerifySecrets_NoFindings(t *testing.T) {
	// No server needed — VerifySecrets with empty findings should return false immediately
	adapter := NewOpenAIStandardAdapter("http://localhost:19999", "test-model",
		WithMaxRetries(1),
		WithRetryWait([]time.Duration{1*time.Millisecond}),
	)

	yes, err := adapter.VerifySecrets("diff", []domain.SecretDetection{})
	if err != nil {
		t.Fatalf("VerifySecrets no findings failed: %v", err)
	}
	if yes {
		t.Error("VerifySecrets: got true, want false when no findings")
	}
}

func TestAdapter_AuditBinaryContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("BINARY"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	isBinary, err := adapter.AuditBinaryContent("image.png", "binary gibberish content")
	if err != nil {
		t.Fatalf("AuditBinaryContent failed: %v", err)
	}
	if !isBinary {
		t.Error("AuditBinaryContent: got false, want true for BINARY response")
	}
}

func TestAdapter_AuditBinaryContent_Text(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("TEXT"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	isBinary, err := adapter.AuditBinaryContent("readme.md", "This is text content")
	if err != nil {
		t.Fatalf("AuditBinaryContent text failed: %v", err)
	}
	if isBinary {
		t.Error("AuditBinaryContent: got true, want false for TEXT response")
	}
}

func TestAdapter_GenerateChangelog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("## Changelog\n\n- feat: new feature\n- fix: bug fix"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	changelog, err := adapter.GenerateChangelog("abc123 def456", "", "")
	if err != nil {
		t.Fatalf("GenerateChangelog failed: %v", err)
	}
	if !strings.Contains(changelog, "Changelog") {
		t.Errorf("changelog: got %q, want to contain 'Changelog'", changelog)
	}
}

func TestAdapter_RegenerateMessage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(completionResponse(fmt.Sprintf("regenerated msg %d", callCount)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
	}
	previousMessages := []string{"old a", "old b"}

	msgs, err := adapter.RegenerateMessage(previousMessages, "make it shorter", chunks)
	if err != nil {
		t.Fatalf("RegenerateMessage failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages: got %d, want 2", len(msgs))
	}
	if msgs[0] != "regenerated msg 1" {
		t.Errorf("msg 0: got %q, want %q", msgs[0], "regenerated msg 1")
	}
	if msgs[1] != "regenerated msg 2" {
		t.Errorf("msg 1: got %q, want %q", msgs[1], "regenerated msg 2")
	}
}

func TestAdapter_RegenerateMessage_MismatchCount(t *testing.T) {
	adapter := NewOpenAIStandardAdapter("http://localhost:8080/v1", "test-model")
	chunks := []domain.DiffChunk{{Files: []string{"a.go"}, Diff: "a"}}
	previousMessages := []string{"msg1", "msg2"} // 2 msgs, 1 chunk

	_, err := adapter.RegenerateMessage(previousMessages, "feedback", chunks)
	if err == nil {
		t.Fatal("expected error for mismatched message/chunk counts, got nil")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error: got %q, want to contain 'does not match'", err.Error())
	}
}

// --- Lifecycle Tests ---

func TestOpenAIStandardAdapter_EnsureRunning_Available(t *testing.T) {
	// When the backend responds 200 to GET /v1/models, EnsureRunning should
	// return (false, nil) — false because we didn't start it ourselves.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("EnsureRunning should request /v1/models, got %s", r.URL.Path)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	started, err := adapter.EnsureRunning()
	if err != nil {
		t.Fatalf("EnsureRunning available: unexpected error %v", err)
	}
	if started {
		t.Error("EnsureRunning available: started = true, want false (we didn't start it)")
	}
}

func TestOpenAIStandardAdapter_EnsureRunning_Unavailable(t *testing.T) {
	// When the backend is unreachable, EnsureRunning should return an error.
	// Use an unreachable port with no retry delays for speed.
	adapter := NewOpenAIStandardAdapter("http://127.0.0.1:19999", "test-model",
		WithMaxRetries(1),
		WithRetryWait([]time.Duration{1*time.Millisecond}),
	)
	_, err := adapter.EnsureRunning()
	if err == nil {
		t.Error("EnsureRunning unavailable: expected error, got nil")
	}
}

func TestOpenAIStandardAdapter_PreWarm_Success(t *testing.T) {
	// When /v1/completions responds 200, PreWarm should return nil.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("PreWarm should POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/completions" {
			t.Errorf("PreWarm should request /v1/completions, got %s", r.URL.Path)
		}

		// Verify the request uses minimal tokens
		var req CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		// max_tokens should be 1 for warmup
		if req.MaxTokens != 1 {
			t.Errorf("PreWarm should set max_tokens=1, got %d", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(completionResponse("."))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	err := adapter.PreWarm()
	if err != nil {
		t.Fatalf("PreWarm success: unexpected error %v", err)
	}
}

func TestOpenAIStandardAdapter_PreWarm_Timeout(t *testing.T) {
	// When the backend is unreachable, PreWarm should return an error.
	adapter := NewOpenAIStandardAdapter("http://127.0.0.1:19999", "test-model",
		WithMaxRetries(1),
		WithRetryWait([]time.Duration{1*time.Millisecond}),
	)
	err := adapter.PreWarm()
	if err == nil {
		t.Error("PreWarm timeout: expected error, got nil")
	}
}

func TestOpenAIStandardAdapter_Stop(t *testing.T) {
	// Stop is a no-op for OpenAI-compatible backends — it should simply
	// return without panicking or erroring.
	adapter := NewOpenAIStandardAdapter("http://localhost:8080/v1", "test-model")
	adapter.Stop() // should not panic
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}