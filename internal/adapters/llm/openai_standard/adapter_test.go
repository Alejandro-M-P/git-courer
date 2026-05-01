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

func TestAdapter_GenerateChunkMessage_UsesChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// GenerateChunkMessage MUST use /chat/completions (not /completions)
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		// Parse request as ChatRequest
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("model: got %q, want %q", req.Model, "test-model")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("feat: add new feature"))
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

func TestAdapter_GenerateChunkMessage_ReasoningEffortNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.ReasoningEffort != "none" {
			t.Errorf("reasoning_effort: got %q, want %q", req.ReasoningEffort, "none")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("feat: add feature"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_SystemPromptIncluded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Must have at least 2 messages: system + user
		if len(req.Messages) < 2 {
			t.Fatalf("messages: got %d, want at least 2 (system + user)", len(req.Messages))
		}
		// First message MUST be system with the anti-reasoning prompt
		if req.Messages[0].Role != "system" {
			t.Errorf("first message role: got %q, want %q", req.Messages[0].Role, "system")
		}
		if req.Messages[0].Content == "" {
			t.Error("system message content is empty, must contain anti-reasoning prompt")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("feat: add feature"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_UserMessageContainsDiff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Second message is user and contains diff content
		if len(req.Messages) < 2 {
			t.Fatalf("messages: got %d, want at least 2", len(req.Messages))
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("second message role: got %q, want %q", req.Messages[1].Role, "user")
		}
		if !strings.Contains(req.Messages[1].Content, "main.go") {
			t.Errorf("user message should contain file name 'main.go', got: %s", req.Messages[1].Content[:min(200, len(req.Messages[1].Content))])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("feat: new thing"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "+ added line"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_Temperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Temperature != 0.3 {
			t.Errorf("temperature: got %f, want 0.3", req.Temperature)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("feat: temp test"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_WithRetryContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the user message includes retry context
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		// Find the user message
		var userContent string
		for _, m := range req.Messages {
			if m.Role == "user" {
				userContent = m.Content
				break
			}
		}
		if !strings.Contains(userContent, "previously rejected") {
			t.Errorf("user message should contain retry context, got: %q", userContent[:min(200, len(userContent))])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("fix: different message"))
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

func TestChatCompletionWithMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify custom messages are passed through
		if len(req.Messages) != 2 {
			t.Errorf("messages: got %d, want 2", len(req.Messages))
		} else {
			if req.Messages[0].Role != "system" {
				t.Errorf("first message role: got %q, want %q", req.Messages[0].Role, "system")
			}
			if req.Messages[1].Role != "user" {
				t.Errorf("second message role: got %q, want %q", req.Messages[1].Role, "user")
			}
		}
		if req.ReasoningEffort != "none" {
			t.Errorf("reasoning_effort: got %q, want %q", req.ReasoningEffort, "none")
		}
		if req.MaxTokens != 5 {
			t.Errorf("max_tokens: got %d, want 5", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("test response"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	messages := []ChatMessage{
		{Role: "system", Content: "custom system prompt"},
		{Role: "user", Content: "user message"},
	}
	result, err := adapter.chatCompletionWithMessages(messages, chatCompletionOpts{
		reasoningEffort: "none",
		maxTokens:       5,
	})
	if err != nil {
		t.Fatalf("chatCompletionWithMessages failed: %v", err)
	}
	if result != "test response" {
		t.Errorf("result: got %q, want %q", result, "test response")
	}
}

func TestAdapter_RegenerateMessage_UsesChatCompletions(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("RegenerateMessage expected /v1/chat/completions, got %s", r.URL.Path)
		}
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(fmt.Sprintf("regenerated msg %d", callCount)))
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

func TestAdapter_RegenerateMessage_SystemPromptAndReasoningEffort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.ReasoningEffort != "none" {
			t.Errorf("reasoning_effort: got %q, want %q", req.ReasoningEffort, "none")
		}
		if len(req.Messages) < 2 || req.Messages[0].Role != "system" {
			t.Errorf("expected system message as first message, got %d messages", len(req.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("fix: regenerated"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunks := []domain.DiffChunk{{Files: []string{"a.go"}, Diff: "diff"}}
	previousMessages := []string{"old msg"}

	msgs, err := adapter.RegenerateMessage(previousMessages, "shorter", chunks)
	if err != nil {
		t.Fatalf("RegenerateMessage failed: %v", err)
	}
	if msgs[0] != "fix: regenerated" {
		t.Errorf("msg: got %q, want %q", msgs[0], "fix: regenerated")
	}
}

func TestAdapter_RegenerateMessage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(fmt.Sprintf("regenerated msg %d", callCount)))
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

func TestOpenAIStandardAdapter_PreWarm_UsesChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("PreWarm should POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("PreWarm should request /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify the request is a ChatRequest with max_tokens=1
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.MaxTokens != 1 {
			t.Errorf("PreWarm should set max_tokens=1, got %d", req.MaxTokens)
		}
		// Verify system message is present
		if len(req.Messages) < 2 || req.Messages[0].Role != "system" {
			t.Errorf("PreWarm should include system message, got %d messages", len(req.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(""))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	err := adapter.PreWarm()
	if err != nil {
		t.Fatalf("PreWarm chat completions: unexpected error %v", err)
	}
}

func TestOpenAIStandardAdapter_PreWarm_Success(t *testing.T) {
	// When /v1/chat/completions responds 200, PreWarm should return nil.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("PreWarm should POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("PreWarm should request /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify the request is a ChatRequest with max_tokens=1
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.MaxTokens != 1 {
			t.Errorf("PreWarm should set max_tokens=1, got %d", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(""))
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