package openai_standard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
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

func TestAdapter_GenerateChunkMessage_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("not valid json at all"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("error = %v, want ErrInvalidJSON", err)
	}
}

func TestAdapter_GenerateChunkMessage_ValidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commit := CommitMessageJSON{Type: "feat", Scope: "api", Description: "add endpoint", Breaking: true, Body: "BREAKING CHANGE: old API removed"}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, commit)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	msg, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "feat(api)!: add endpoint\n\nBREAKING CHANGE: old API removed"
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
}

func TestAdapter_GenerateChunkMessage_ValidJSONNoScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commit := CommitMessageJSON{Type: "fix", Description: "fix crash", Body: "details here"}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, commit)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	msg, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "fix: fix crash\n\ndetails here"
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
}

func TestAdapter_GenerateChunkMessage_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(""))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
	if !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("error = %v, want ErrEmptyResponse", err)
	}
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add new feature"})))
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_UserMessageOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Dual suppression: system /no_think + user prompt
		if len(req.Messages) != 2 {
			t.Fatalf("messages: got %d, want 2 (system /no_think + user)", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("first message role: got %q, want %q", req.Messages[0].Role, "system")
		}
		if req.Messages[0].Content != "/no_think" {
			t.Errorf("first message content: got %q, want %q", req.Messages[0].Content, "/no_think")
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("second message role: got %q, want %q", req.Messages[1].Role, "user")
		}

		// Verify exact prompt match on user message
		wantPrompt, _ := prompts.RenderOp("commit_message", prompts.MessageParams{
			Files: "main.go",
			Diff:  "diff",
		})
		if req.Messages[1].Content != wantPrompt {
			t.Errorf("prompt mismatch:\ngot: %q\nwant: %q", req.Messages[1].Content, wantPrompt)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add feature"})))
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

		// Dual suppression: system /no_think + user prompt
		if len(req.Messages) != 2 {
			t.Fatalf("messages: got %d, want 2 (system /no_think + user)", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("first message role: got %q, want %q", req.Messages[0].Role, "system")
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("second message role: got %q, want %q", req.Messages[1].Role, "user")
		}
		if !strings.Contains(req.Messages[1].Content, "main.go") {
			t.Errorf("user message should contain file name 'main.go', got: %s", req.Messages[1].Content[:min(200, len(req.Messages[1].Content))])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "new thing"})))
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
		if req.Temperature == nil {
			t.Fatal("temperature: got nil, want non-nil pointer to 0.3")
		}
		if *req.Temperature != 0.3 {
			t.Errorf("temperature: got %f, want 0.3", *req.Temperature)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "temp test"})))
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "fix", Description: "different message"})))
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

func TestAdapter_DecideCommit_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Non-JSON response — should return error, not fallback to YES/NO parsing
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("YES, src/"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.DecideCommit("commit all", "M file.go", "new.go", "", "")
	if err == nil {
		t.Fatal("expected error for non-JSON DecideCommit response, got nil")
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("error = %v, want ErrInvalidJSON", err)
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

		// Verify exact prompt match user message
		wantPrompt, _ := prompts.RenderOp("decide_commit", prompts.DecideParams{
			Instruction: "commit everything",
			GitStatus:   "M file.go",
			Untracked:   "new.go",
			Modified:    "",
			Deleted:     "",
		})
		if len(req.Messages) < 2 {
			t.Fatalf("messages: got %d, want at least 2", len(req.Messages))
		}
		if req.Messages[1].Content != wantPrompt {
			t.Errorf("prompt mismatch:\ngot: %q\nwant: %q", req.Messages[1].Content, wantPrompt)
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

func TestAdapter_InterpretGitOp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify exact prompt match on user message
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) < 2 {
			t.Fatalf("messages: got %d, want at least 2", len(req.Messages))
		}
		ctx := map[string]string{"current_branch": "main", "Instruction": "create login branch"}
		tmpl, _ := prompts.Get("branch_create")
		wantPrompt, _ := prompts.Render(tmpl, ctx)
		if req.Messages[1].Content != wantPrompt {
			t.Errorf("prompt mismatch:\ngot: %q\nwant: %q", req.Messages[1].Content, wantPrompt)
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

func TestAdapter_InterpretGitOp_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("not a json object"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.InterpretGitOp("branch_create", "create branch", nil)
	if err == nil {
		t.Fatal("expected error for non-JSON InterpretGitOp response, got nil")
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("error = %v, want ErrInvalidJSON", err)
	}
}

func TestAdapter_SetContext_Behavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		var userContent string
		for _, m := range req.Messages {
			if m.Role == "user" {
				userContent = m.Content
				break
			}
		}
		if !strings.Contains(userContent, "Project context: Project: X") {
			t.Errorf("user message should contain context after SetContext; got:\n%s", userContent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetContext("Project: X")
	_, err := adapter.GenerateChunkMessage(domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"})
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_ContextInjected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		var userContent string
		for _, m := range req.Messages {
			if m.Role == "user" {
				userContent = m.Content
				break
			}
		}
		if !strings.Contains(userContent, "Project context: Project: X") {
			t.Errorf("user message should contain context; got:\n%s", userContent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetContext("Project: X")
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChangelog_ContextInjected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		var userContent string
		for _, m := range req.Messages {
			if m.Role == "user" {
				userContent = m.Content
				break
			}
		}
		if !strings.Contains(userContent, "Project context: Project: X") {
			t.Errorf("user message should contain context; got:\n%s", userContent)
		}
		changelogJSON := domain.Changelog{Features: []string{"y"}}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, changelogJSON)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetContext("Project: X")
	_, err := adapter.GenerateChangelog("abc", "", "")
	if err != nil {
		t.Fatalf("GenerateChangelog failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_EmptyContextOmitsBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		var userContent string
		for _, m := range req.Messages {
			if m.Role == "user" {
				userContent = m.Content
				break
			}
		}
		if strings.Contains(userContent, "Project context:") {
			t.Errorf("user message should NOT contain empty context block")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetContext("")
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
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
		WithRetryWait([]time.Duration{1 * time.Millisecond}),
	)

	yes, err := adapter.VerifySecrets("diff", []domain.SecretDetection{})
	if err != nil {
		t.Fatalf("VerifySecrets no findings failed: %v", err)
	}
	if yes {
		t.Error("VerifySecrets: got true, want false when no findings")
	}
}

func TestAdapter_AuditBinaryContent_NoServer(t *testing.T) {
	// AuditBinaryContent uses a pure-Go heuristic (null-byte detection).
	// No HTTP server is required — the adapter never makes an LLM call for this method.
	adapter := NewOpenAIStandardAdapter("http://127.0.0.1:1/v1", "test-model")

	isBinary, err := adapter.AuditBinaryContent("image.png", "binary gibberish content")
	if err != nil {
		t.Fatalf("AuditBinaryContent failed: %v", err)
	}
	if isBinary {
		t.Error("AuditBinaryContent: got true, want false for plain text (no null bytes)")
	}

	isBinary, err = adapter.AuditBinaryContent("image.png", "binary\x00gibberish")
	if err != nil {
		t.Fatalf("AuditBinaryContent failed: %v", err)
	}
	if !isBinary {
		t.Error("AuditBinaryContent: got false, want true for content with null bytes")
	}
}

func TestAdapter_AuditBinaryContent_Text(t *testing.T) {
	adapter := NewOpenAIStandardAdapter("http://127.0.0.1:1/v1", "test-model")

	isBinary, err := adapter.AuditBinaryContent("readme.md", "This is text content")
	if err != nil {
		t.Fatalf("AuditBinaryContent text failed: %v", err)
	}
	if isBinary {
		t.Error("AuditBinaryContent: got true, want false for TEXT response")
	}
}

func TestAdapter_GenerateChangelog_ReturnsChangelog(t *testing.T) {
	changelogJSON := domain.Changelog{
		Features: []string{"add login", "add logout"},
		Fixes:    []string{"fix crash"},
		Breaking: []string{"remove old API"},
		Docs:     []string{"update readme"},
		Perf:     []string{"faster queries"},
		Internal: []string{"refactor"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, changelogJSON)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	ch, err := adapter.GenerateChangelog("abc123 def456", "", "")
	if err != nil {
		t.Fatalf("GenerateChangelog failed: %v", err)
	}
	if ch == nil {
		t.Fatal("changelog should not be nil")
	}
	if len(ch.Features) != 2 || ch.Features[0] != "add login" {
		t.Errorf("Features: got %v", ch.Features)
	}
	if len(ch.Fixes) != 1 || ch.Fixes[0] != "fix crash" {
		t.Errorf("Fixes: got %v", ch.Fixes)
	}
	if len(ch.Breaking) != 1 || ch.Breaking[0] != "remove old API" {
		t.Errorf("Breaking: got %v", ch.Breaking)
	}
	if len(ch.Docs) != 1 || ch.Docs[0] != "update readme" {
		t.Errorf("Docs: got %v", ch.Docs)
	}
	if len(ch.Perf) != 1 || ch.Perf[0] != "faster queries" {
		t.Errorf("Perf: got %v", ch.Perf)
	}
	if len(ch.Internal) != 1 || ch.Internal[0] != "refactor" {
		t.Errorf("Internal: got %v", ch.Internal)
	}
}

func TestAdapter_GenerateChangelog_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("not valid json"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.GenerateChangelog("abc", "", "")
	if err == nil {
		t.Fatal("expected error for invalid JSON changelog, got nil")
	}
}

func TestAdapter_GenerateChangelog_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(""))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.GenerateChangelog("abc", "", "")
	if err == nil {
		t.Fatal("expected error for empty changelog response, got nil")
	}
	if !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("error = %v, want ErrEmptyResponse", err)
	}
}

func TestAdapter_GenerateChangelog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		changelogJSON := domain.Changelog{Features: []string{"new feature"}, Fixes: []string{"bug fix"}}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, changelogJSON)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	ch, err := adapter.GenerateChangelog("abc123 def456", "", "")
	if err != nil {
		t.Fatalf("GenerateChangelog failed: %v", err)
	}
	if ch == nil {
		t.Fatal("changelog should not be nil")
	}
	if len(ch.Features) != 1 || ch.Features[0] != "new feature" {
		t.Errorf("Features: got %v", ch.Features)
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

func TestAdapter_RegenerateChunk_ContextInjected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		var userContent string
		for _, m := range req.Messages {
			if m.Role == "user" {
				userContent = m.Content
				break
			}
		}
		if !strings.Contains(userContent, "Project context: Project: X") {
			t.Errorf("retry prompt should contain context; got:\n%s", userContent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetContext("Project: X")
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	previousMessages := []string{"old msg"}
	_, err := adapter.RegenerateMessage(previousMessages, "feedback", []domain.DiffChunk{chunk})
	if err != nil {
		t.Fatalf("RegenerateMessage failed: %v", err)
	}
}

func TestAdapter_RegenerateChunk_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("not valid json"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	previousMessages := []string{"old msg"}
	_, err := adapter.RegenerateMessage(previousMessages, "feedback", []domain.DiffChunk{chunk})
	if err == nil {
		t.Fatal("expected error for invalid JSON in regenerate, got nil")
	}
}

func TestAdapter_RegenerateChunk_ValidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commit := CommitMessageJSON{Type: "fix", Scope: "auth", Description: "fix login", Body: "details"}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, commit)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	previousMessages := []string{"old msg"}
	msgs, err := adapter.RegenerateMessage(previousMessages, "feedback", []domain.DiffChunk{chunk})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "fix(auth): fix login\n\ndetails"
	if msgs[0] != want {
		t.Errorf("msg = %q, want %q", msgs[0], want)
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "chore", Description: fmt.Sprintf("regenerated msg %d", callCount)})))
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
	if msgs[0] != "chore: regenerated msg 1" {
		t.Errorf("msg 0: got %q, want %q", msgs[0], "chore: regenerated msg 1")
	}
	if msgs[1] != "chore: regenerated msg 2" {
		t.Errorf("msg 1: got %q, want %q", msgs[1], "chore: regenerated msg 2")
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
		if len(req.Messages) < 1 {
			t.Errorf("expected at least 1 message, got %d messages", len(req.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "fix", Description: "regenerated"})))
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "chore", Description: fmt.Sprintf("regenerated msg %d", callCount)})))
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
	if msgs[0] != "chore: regenerated msg 1" {
		t.Errorf("msg 0: got %q, want %q", msgs[0], "chore: regenerated msg 1")
	}
	if msgs[1] != "chore: regenerated msg 2" {
		t.Errorf("msg 1: got %q, want %q", msgs[1], "chore: regenerated msg 2")
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
		WithRetryWait([]time.Duration{1 * time.Millisecond}),
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
		// Verify single user message is present (no system message to save tokens)
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
			t.Errorf("PreWarm should include 1 user message, got %d messages with role %q", len(req.Messages), req.Messages[0].Role)
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
		WithRetryWait([]time.Duration{1 * time.Millisecond}),
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

func TestAdapter_DecideCommit_Params(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Fatal("DecideCommit: temperature is nil, want non-nil pointer to 0.0")
		}
		if *req.Temperature != 0.0 {
			t.Errorf("DecideCommit temperature: got %f, want 0.0", *req.Temperature)
		}
		if req.MaxTokens != 128 {
			t.Errorf("DecideCommit maxTokens: got %d, want 128", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(`{"include_untracked": true, "file_filter": []}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.DecideCommit("commit all", "M file.go", "new.go", "", "")
	if err != nil {
		t.Fatalf("DecideCommit failed: %v", err)
	}
}

func TestAdapter_InterpretGitOp_Params(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Fatal("InterpretGitOp: temperature is nil, want non-nil pointer to 0.1")
		}
		if *req.Temperature != 0.1 {
			t.Errorf("InterpretGitOp temperature: got %f, want 0.1", *req.Temperature)
		}
		if req.MaxTokens != 256 {
			t.Errorf("InterpretGitOp maxTokens: got %d, want 256", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(`{"branch": "feat/x"}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.InterpretGitOp("branch_create", "create branch", nil)
	if err != nil {
		t.Fatalf("InterpretGitOp failed: %v", err)
	}
}

func TestAdapter_VerifySecrets_Params(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Fatal("VerifySecrets: temperature is nil, want non-nil pointer to 0.0")
		}
		if *req.Temperature != 0.0 {
			t.Errorf("VerifySecrets temperature: got %f, want 0.0", *req.Temperature)
		}
		if req.MaxTokens != 64 {
			t.Errorf("VerifySecrets maxTokens: got %d, want 64", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("YES"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	findings := []domain.SecretDetection{
		{File: "config.go", Line: 10, Type: "api_key", Content: "AKIA..."},
	}
	_, err := adapter.VerifySecrets("diff", findings)
	if err != nil {
		t.Fatalf("VerifySecrets failed: %v", err)
	}
}

func TestAdapter_AuditBinaryContent_Params(t *testing.T) {
	// No server call — AuditBinaryContent uses Go heuristic, not LLM.
	adapter := NewOpenAIStandardAdapter("http://127.0.0.1:1/v1", "test-model")
	_, err := adapter.AuditBinaryContent("file.bin", "binary content")
	if err != nil {
		t.Fatalf("AuditBinaryContent failed: %v", err)
	}
}

func TestAdapter_GenerateChangelog_Params(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Fatal("GenerateChangelog: temperature is nil, want non-nil pointer to 0.3")
		}
		if *req.Temperature != 0.3 {
			t.Errorf("GenerateChangelog temperature: got %f, want 0.3", *req.Temperature)
		}
		if req.MaxTokens != 1024 {
			t.Errorf("GenerateChangelog maxTokens: got %d, want 1024", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, domain.Changelog{Features: []string{"new feature"}})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.GenerateChangelog("abc123 def456", "", "")
	if err != nil {
		t.Fatalf("GenerateChangelog failed: %v", err)
	}
}

func TestAdapter_RegenerateMessage_Params(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Fatal("RegenerateMessage: temperature is nil, want non-nil pointer to 0.5")
		}
		if *req.Temperature != 0.5 {
			t.Errorf("RegenerateMessage temperature: got %f, want 0.5", *req.Temperature)
		}
		if req.MaxTokens != 256 {
			t.Errorf("RegenerateMessage maxTokens: got %d, want 256", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "fix", Description: "regenerated"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunks := []domain.DiffChunk{{Files: []string{"a.go"}, Diff: "diff"}}
	previousMessages := []string{"old msg"}
	_, err := adapter.RegenerateMessage(previousMessages, "shorter", chunks)
	if err != nil {
		t.Fatalf("RegenerateMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_Params(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Fatal("GenerateChunkMessage: temperature is nil, want non-nil pointer to 0.3")
		}
		if *req.Temperature != 0.3 {
			t.Errorf("GenerateChunkMessage temperature: got %f, want 0.3", *req.Temperature)
		}
		if req.MaxTokens != 256 {
			t.Errorf("GenerateChunkMessage maxTokens: got %d, want 256", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_DecideCommit_ZeroTemperatureNotOmitted(t *testing.T) {
	// Verify that temperature=0 is serialized as "temperature":0 in the raw JSON,
	// not omitted by omitempty. *float64 with omitempty omits nil, not 0.
	var rawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(`{"include_untracked": true, "file_filter": []}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.DecideCommit("commit all", "M file.go", "", "", "")
	if err != nil {
		t.Fatalf("DecideCommit failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to unmarshal raw body: %v", err)
	}
	tempVal, ok := parsed["temperature"]
	if !ok {
		t.Fatal("DecideCommit: temperature key missing from raw JSON — zero must NOT be omitted")
	}
	// JSON numbers decode as float64
	if tempNum, ok := tempVal.(float64); !ok || tempNum != 0 {
		t.Errorf("DecideCommit: temperature = %v, want 0", tempVal)
	}
}

func TestAdapter_PreWarm_TemperatureOmitted(t *testing.T) {
	// Verify that PreWarm (nil temperature) does NOT include "temperature" in raw JSON.
	var rawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(""))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	err := adapter.PreWarm()
	if err != nil {
		t.Fatalf("PreWarm failed: %v", err)
	}

	if strings.Contains(string(rawBody), `"temperature"`) {
		t.Errorf("PreWarm: raw JSON should NOT contain 'temperature' key, got: %s", string(rawBody))
	}
}

// MockLLMCounter is a mock ports.LLM that counts calls with a mutex (for concurrent safety).
type mockLLM struct {
	mu      sync.Mutex
	callLog []int // logged chunk indices in call order
	results map[int]string
	errors  map[int]error
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errSentinel error
	}{
		{
			name:    "fenced valid JSON",
			input:   "```json\n{\"type\":\"feat\",\"description\":\"add feature\"}\n```",
			wantErr: false,
		},
		{
			name:    "valid bare JSON",
			input:   `{"type":"feat","description":"add feature"}`,
			wantErr: false,
		},
		{
			name:        "empty after clean",
			input:       "```\n```",
			wantErr:     true,
			errSentinel: ErrEmptyResponse,
		},
		{
			name:        "whitespace only",
			input:       "   \n\t  ",
			wantErr:     true,
			errSentinel: ErrEmptyResponse,
		},
		{
			name:        "non-object prefix",
			input:       `[1,2,3]`,
			wantErr:     true,
			errSentinel: ErrInvalidJSON,
		},
		{
			name:        "invalid JSON",
			input:       `{not valid json}`,
			wantErr:     true,
			errSentinel: ErrInvalidJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target map[string]interface{}
			err := parseJSON(tt.input, &target)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSentinel != nil && !errors.Is(err, tt.errSentinel) {
					t.Errorf("error = %v, want sentinel %v", err, tt.errSentinel)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(target) == 0 {
				t.Errorf("target should be populated, got empty map")
			}
		})
	}
}

func TestCommitMessageJSON_ToConventionalCommit(t *testing.T) {
	tests := []struct {
		name   string
		commit CommitMessageJSON
		want   string
	}{
		{
			name:   "no scope no breaking no body",
			commit: CommitMessageJSON{Type: "feat", Description: "add feature"},
			want:   "feat: add feature",
		},
		{
			name:   "with scope no breaking no body",
			commit: CommitMessageJSON{Type: "fix", Scope: "auth", Description: "fix login"},
			want:   "fix(auth): fix login",
		},
		{
			name:   "breaking with scope",
			commit: CommitMessageJSON{Type: "feat", Scope: "api", Description: "new endpoint", Breaking: true},
			want:   "feat(api)!: new endpoint",
		},
		{
			name:   "breaking no scope",
			commit: CommitMessageJSON{Type: "feat", Description: "overhaul system", Breaking: true},
			want:   "feat!: overhaul system",
		},
		{
			name:   "with body no scope",
			commit: CommitMessageJSON{Type: "fix", Description: "fix crash", Body: "Detailed explanation\nof the fix."},
			want:   "fix: fix crash\n\nDetailed explanation\nof the fix.",
		},
		{
			name:   "with scope breaking and body",
			commit: CommitMessageJSON{Type: "refactor", Scope: "core", Description: "rewrite engine", Breaking: true, Body: "BREAKING CHANGE: old API removed"},
			want:   "refactor(core)!: rewrite engine\n\nBREAKING CHANGE: old API removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.commit.ToConventionalCommit()
			if got != tt.want {
				t.Errorf("ToConventionalCommit() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJSON_IntoStruct(t *testing.T) {
	// Triangulate: verify parseJSON works with a concrete struct type
	input := "```json\n{\"type\":\"fix\",\"scope\":\"auth\",\"description\":\"fix login\",\"breaking\":false,\"body\":\"details\"}\n```"
	var commit CommitMessageJSON
	if err := parseJSON(input, &commit); err != nil {
		t.Fatalf("parseJSON into CommitMessageJSON: %v", err)
	}
	if commit.Type != "fix" {
		t.Errorf("Type: got %q, want %q", commit.Type, "fix")
	}
	if commit.Scope != "auth" {
		t.Errorf("Scope: got %q, want %q", commit.Scope, "auth")
	}
	if commit.Description != "fix login" {
		t.Errorf("Description: got %q, want %q", commit.Description, "fix login")
	}
	if commit.Breaking != false {
		t.Errorf("Breaking: got %v, want false", commit.Breaking)
	}
	if commit.Body != "details" {
		t.Errorf("Body: got %q, want %q", commit.Body, "details")
	}
}

// mockJSONResponse builds a mock LLM response string from a Go value.
func mockJSONResponse(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Phase 5: RegenerateMessage Parallelism Tests ---

func TestAdapter_RegenerateMessage_NumParallel3(t *testing.T) {
	// Build a server that returns different content per call so we can verify ordering.
	callCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		cc := callCount
		mu.Unlock()

		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "chore", Description: fmt.Sprintf("regenerated msg %d", cc)})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.numParallel = 3

	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
		{Files: []string{"c.go"}, Diff: "diff c"},
		{Files: []string{"d.go"}, Diff: "diff d"},
	}
	previousMessages := []string{"old a", "old b", "old c", "old d"}

	msgs, err := adapter.RegenerateMessage(previousMessages, "make it shorter", chunks)
	if err != nil {
		t.Fatalf("RegenerateMessage failed: %v", err)
	}
	if len(msgs) != len(chunks) {
		t.Fatalf("messages: got %d, want %d", len(msgs), len(chunks))
	}

	// Verify each position has a non-empty regenerated message
	for i, msg := range msgs {
		if msg == "" {
			t.Errorf("msg[%d] is empty, want non-empty", i)
		}
		if strings.Contains(msg, "regenerated msg ") || strings.Contains(msg, "chore:") {
			// ok — it's from the server
		} else {
			t.Errorf("msg[%d] = %q, want to contain 'regenerated msg'", i, msg)
		}
	}

	// Total calls must equal chunk count
	mu.Lock()
	if callCount != len(chunks) {
		t.Errorf("callCount = %d, want %d", callCount, len(chunks))
	}
	mu.Unlock()
}

func TestAdapter_RegenerateMessage_NumParallel1(t *testing.T) {
	// NumParallel=1 should behave identically to serial loop.
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "chore", Description: fmt.Sprintf("serial msg %d", callCount)})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.numParallel = 1

	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
	}
	previousMessages := []string{"old a", "old b"}

	msgs, err := adapter.RegenerateMessage(previousMessages, "feedback", chunks)
	if err != nil {
		t.Fatalf("RegenerateMessage failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages: got %d, want 2", len(msgs))
	}
	// Serial execution means call order matches chunk order exactly
	if msgs[0] != "chore: serial msg 1" {
		t.Errorf("msg[0]: got %q, want %q", msgs[0], "chore: serial msg 1")
	}
	if msgs[1] != "chore: serial msg 2" {
		t.Errorf("msg[1]: got %q, want %q", msgs[1], "chore: serial msg 2")
	}
}

func TestAdapter_RegenerateMessage_OneChunkFails(t *testing.T) {
	// Make the server fail deterministically for the chunk whose diff contains "diff b".
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "bad req"}`))
			return
		}

		var userContent string
		for _, m := range req.Messages {
			if m.Role == "user" {
				userContent = m.Content
				break
			}
		}

		// Fail the chunk whose user prompt contains "diff b"
		if strings.Contains(userContent, "diff b") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "boom"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "fix", Description: "ok"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.numParallel = 3

	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
		{Files: []string{"c.go"}, Diff: "diff c"},
	}
	previousMessages := []string{"old a", "old b", "old c"}

	msgs, err := adapter.RegenerateMessage(previousMessages, "feedback", chunks)
	// Should return messages with partial results + error containing warnings
	if err == nil {
		t.Fatalf("expected error for failed chunk, got nil")
	}
	if msgs == nil || len(msgs) != 3 {
		t.Fatalf("expected %d messages, got %d", 3, len(msgs))
	}
	if msgs[0] == "" {
		t.Errorf("msg[0] is empty, want non-empty (ok)")
	}
	if msgs[1] != "" {
		t.Errorf("msg[1] = %q, want empty string for failed chunk", msgs[1])
	}
	if msgs[2] == "" {
		t.Errorf("msg[2] is empty, want non-empty (ok)")
	}
	if !strings.Contains(err.Error(), "warnings") {
		t.Errorf("error should contain 'warnings', got: %q", err.Error())
	}
}

func TestAdapter_RegenerateMessage_AllChunksFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "all broken"}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.numParallel = 2

	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
	}
	previousMessages := []string{"old a", "old b"}

	msgs, err := adapter.RegenerateMessage(previousMessages, "feedback", chunks)
	if err == nil {
		t.Fatalf("expected error when all chunks fail, got nil")
	}
	if msgs == nil || len(msgs) != 2 {
		t.Fatalf("expected %d messages, got %d", 2, len(msgs))
	}
	if msgs[0] != "" || msgs[1] != "" {
		t.Errorf("expected all messages empty, got %v", msgs)
	}
	if !strings.Contains(err.Error(), "warnings") {
		t.Errorf("error should contain 'warnings', got: %q", err.Error())
	}
}

// --- think:false + Ollama options tests ---

func TestAdapter_GenerateChunkMessage_ThinkIsFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode: %v", err)
		}
		think, ok := raw["think"]
		if !ok {
			t.Fatal("think key missing — must always be present")
		}
		if think != false {
			t.Errorf("think = %v, want false", think)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.GenerateChunkMessage(domain.DiffChunk{Files: []string{"x.go"}, Diff: "d"})
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_OllamaOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode: %v", err)
		}
		opts, ok := raw["options"].(map[string]interface{})
		if !ok {
			t.Fatal("options map missing for ollama provider")
		}
		if opts["num_ctx"] != float64(4096) {
			t.Errorf("options.num_ctx = %v, want 4096", opts["num_ctx"])
		}
		if opts["keep_alive"] != "5m" {
			t.Errorf("options.keep_alive = %v, want 5m", opts["keep_alive"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Type: "feat", Description: "add"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetProvider("ollama")
	_, err := adapter.GenerateChunkMessage(domain.DiffChunk{Files: []string{"x.go"}, Diff: "d"})
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}
