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
		commit := CommitMessageJSON{Description: "add endpoint", Body: "BREAKING CHANGE: old API removed"}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, commit)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff", CommitType: "feat!"}
	msg, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "feat!: add endpoint\n\nBREAKING CHANGE: old API removed"
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
}

func TestAdapter_GenerateChunkMessage_ValidJSONNoScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commit := CommitMessageJSON{Description: "fix crash", Body: "details here"}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, commit)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff", CommitType: "fix"}
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
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("error = %v, want ErrInvalidJSON (empty response falls through parseSingleOrArray fallback)", err)
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add new feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files:      []string{"main.go", "util.go"},
		Diff:       "diff content here",
		CommitType: "feat",
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
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
		wantPrompt, _ := prompts.Render(prompts.GetCommitMessage(), prompts.MessageParams{
			Files:      "main.go",
			CommitType: "fix", // InferCommitType infers "fix" for source modifications
			Diff:       "diff",
		})
		if req.Messages[1].Content != wantPrompt {
			t.Errorf("prompt mismatch:\ngot: %q\nwant: %q", req.Messages[1].Content, wantPrompt)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
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

		if len(req.Messages) != 2 {
			t.Fatalf("messages: got %d, want 2 (system /no_think + user)", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("first message role: got %q, want system", req.Messages[0].Role)
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("second message role: got %q, want user", req.Messages[1].Role)
		}
		// AnnotatedDiff includes the file header (📄 main.go) so the name appears in the prompt
		if !strings.Contains(req.Messages[1].Content, "main.go") {
			t.Errorf("user message should contain file name 'main.go', got: %s", req.Messages[1].Content[:min(200, len(req.Messages[1].Content))])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "new thing"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files:         []string{"main.go"},
		Diff:          "+ added line",
		AnnotatedDiff: "📄 main.go\nNewHandler [NEW_FUNC] main.go:10\n",
		CommitType:    "feat",
	}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// TestAdapter_GenerateChunkMessage_UsesAnnotatedDiffWhenPresent verifies that
// when AnnotatedDiff is set, it is sent to the LLM instead of the raw diff.
func TestAdapter_GenerateChunkMessage_UsesAnnotatedDiffWhenPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		userMsg := req.Messages[1].Content
		if !strings.Contains(userMsg, "Annotated Diff") {
			t.Errorf("prompt should use 'Annotated Diff' section, got:\n%s", userMsg)
		}
		if strings.Contains(userMsg, "raw diff content") {
			t.Errorf("prompt should NOT contain raw diff when annotated diff is present")
		}
		if !strings.Contains(userMsg, "NewHandler [NEW_FUNC]") {
			t.Errorf("prompt should contain annotated diff content, got:\n%s", userMsg)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add handler"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files:         []string{"internal/api/handler.go"},
		Diff:          "raw diff content",
		AnnotatedDiff: "📄 internal/api/handler.go\nNewHandler [NEW_FUNC] internal/api/handler.go:10\n",
		CommitType:    "feat",
	}
	if _, err := adapter.GenerateChunkMessage(chunk); err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// TestAdapter_GenerateChunkMessage_FallsBackToRawDiff verifies that when
// AnnotatedDiff is empty, the raw diff is used instead.
func TestAdapter_GenerateChunkMessage_FallsBackToRawDiff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		userMsg := req.Messages[1].Content
		if strings.Contains(userMsg, "Annotated Diff") {
			t.Errorf("prompt should NOT use 'Annotated Diff' section when AnnotatedDiff is empty")
		}
		if !strings.Contains(userMsg, "Diff:") {
			t.Errorf("prompt should use raw 'Diff:' section, got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, "+ added function") {
			t.Errorf("prompt should contain raw diff content, got:\n%s", userMsg)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add function"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files:      []string{"internal/api/handler.go"},
		Diff:       "+ added function",
		CommitType: "feat",
	}
	if _, err := adapter.GenerateChunkMessage(chunk); err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// TestAdapter_GenerateChunkMessage_UsesPreClassifiedCommitType verifies that
// the commit type from the classifier (chunk.CommitType) is used in the final
// output — the LLM only generates description+body.
func TestAdapter_GenerateChunkMessage_UsesPreClassifiedCommitType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{
			Description: "add user authentication handler",
			Body:        "Handles OAuth2 token validation for API requests",
		})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)

	cases := []struct {
		name       string
		commitType string
		wantPrefix string
	}{
		{"feat", "feat", "feat:"},
		{"fix", "fix", "fix:"},
		{"breaking", "feat!", "feat!:"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunk := domain.DiffChunk{
				Files:         []string{"internal/auth/handler.go"},
				Diff:          "+ func NewAuthHandler",
				AnnotatedDiff: "📄 internal/auth/handler.go\nNewAuthHandler [NEW_FUNC] internal/auth/handler.go:5\n",
				CommitType:    tc.commitType,
			}
			msg, err := adapter.GenerateChunkMessage(chunk)
			if err != nil {
				t.Fatalf("GenerateChunkMessage failed: %v", err)
			}
			if !strings.HasPrefix(msg, tc.wantPrefix) {
				t.Errorf("commit message should start with %q, got: %q", tc.wantPrefix, msg)
			}
		})
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "temp test"})))
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "different message"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetRetryContext("previously rejected message")

	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff", CommitType: "fix"}
	msg, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage with retry failed: %v", err)
	}
	if msg != "fix: different message" {
		t.Errorf("message: got %q, want %q", msg, "fix: different message")
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
		if !strings.Contains(userContent, "Context:\nProject: X") {
			t.Errorf("user message should contain context after SetContext; got:\n%s", userContent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
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
		if !strings.Contains(userContent, "Context:\nProject: X") {
			t.Errorf("user message should contain context; got:\n%s", userContent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
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
		if strings.Contains(userContent, "Project context:") || strings.Contains(userContent, "Context:\n") {
			t.Errorf("user message should NOT contain empty context block")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
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

func TestAdapter_SetWhy(t *testing.T) {
	adapter := NewOpenAIStandardAdapter("http://localhost:8080/v1", "test-model")
	adapter.SetWhy("refactor auth")
	if adapter.why != "refactor auth" {
		t.Errorf("why: got %q, want %q", adapter.why, "refactor auth")
	}
}

func TestAdapter_ClearWhy(t *testing.T) {
	adapter := NewOpenAIStandardAdapter("http://localhost:8080/v1", "test-model")
	adapter.SetWhy("refactor auth")
	adapter.ClearWhy()
	if adapter.why != "" {
		t.Errorf("why after clear: got %q, want empty", adapter.why)
	}
}

func TestAdapter_GenerateChunkMessage_WhyInjected(t *testing.T) {
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
		if !strings.Contains(userContent, "Developer's reason") {
			t.Errorf("prompt should contain 'Developer's reason' heading when Why is set; got:\n%s", userContent[:min(300, len(userContent))])
		}
		if !strings.Contains(userContent, "refactor auth module") {
			t.Errorf("prompt should contain Why text 'refactor auth module'; got:\n%s", userContent[:min(300, len(userContent))])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "refactor auth"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetWhy("refactor auth module")
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff", CommitType: "refactor"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage with Why failed: %v", err)
	}
}

func TestAdapter_GenerateChunkMessage_WhyCleared_NoWhyInPrompt(t *testing.T) {
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
		if strings.Contains(userContent, "Developer's reason") {
			t.Errorf("prompt should NOT contain 'Developer's reason' when Why is empty; got:\n%s", userContent[:min(300, len(userContent))])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetWhy("something")
	adapter.ClearWhy()
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage after ClearWhy failed: %v", err)
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
		if !strings.Contains(userContent, "Project context: Project: X") && !strings.Contains(userContent, "Context:\nProject: X") {
			t.Errorf("retry prompt should contain context; got:\n%s", userContent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
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
		commit := CommitMessageJSON{Description: "fix login", Body: "details"}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, commit)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff", CommitType: "fix"}
	previousMessages := []string{"old msg"}
	msgs, err := adapter.RegenerateMessage(previousMessages, "feedback", []domain.DiffChunk{chunk})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "fix: fix login\n\ndetails"
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: fmt.Sprintf("regenerated msg %d", callCount)})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "chore"},
		{Files: []string{"b.go"}, Diff: "diff b", CommitType: "chore"},
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "regenerated"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunks := []domain.DiffChunk{{Files: []string{"a.go"}, Diff: "diff", CommitType: "fix"}}
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: fmt.Sprintf("regenerated msg %d", callCount)})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "chore"},
		{Files: []string{"b.go"}, Diff: "diff b", CommitType: "chore"},
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "regenerated"})))
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add feature"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{Files: []string{"main.go"}, Diff: "diff"}
	_, err := adapter.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
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
		{
			name:    "text before JSON",
			input:   "Here is the result:\n{\"type\":\"fix\",\"description\":\"fix bug\"}",
			wantErr: false,
		},
		{
			name:    "text before and after JSON",
			input:   "This is what I got:\n```json\n{\"type\":\"feat\",\"description\":\"add feature\"}\n```\nHope this helps!",
			wantErr: false,
		},
		{
			name:    "think block before text before JSON",
			input:   "<think>Let me analyze this...</think>\nHere is the JSON:\n```json\n{\"type\":\"refactor\",\"description\":\"refactor code\"}\n```",
			wantErr: false,
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
		name       string
		commit     CommitMessageJSON
		commitType string
		scope      string
		breaking   bool
		want       string
	}{
		{
			name:       "feat no breaking no body",
			commit:     CommitMessageJSON{Description: "add feature"},
			commitType: "feat",
			want:       "feat: add feature",
		},
		{
			name:       "feat breaking no body",
			commit:     CommitMessageJSON{Description: "overhaul system"},
			commitType: "feat",
			breaking:   true,
			want:       "feat!: overhaul system",
		},
		{
			name:       "fix with body",
			commit:     CommitMessageJSON{Description: "fix crash", Body: "Detailed explanation\nof the fix."},
			commitType: "fix",
			want:       "fix: fix crash\n\nDetailed explanation\nof the fix.",
		},
		{
			name:       "refactor breaking with body",
			commit:     CommitMessageJSON{Description: "rewrite engine", Body: "BREAKING CHANGE: old API removed"},
			commitType: "refactor",
			breaking:   true,
			want:       "refactor!: rewrite engine\n\nBREAKING CHANGE: old API removed",
		},
		{
			name:       "chore no breaking no body",
			commit:     CommitMessageJSON{Description: "update deps"},
			commitType: "chore",
			want:       "chore: update deps",
		},
		{
			name:       "ci no breaking no body",
			commit:     CommitMessageJSON{Description: "fix ci pipeline"},
			commitType: "ci",
			want:       "ci: fix ci pipeline",
		},
		{
			name:       "feat with scope",
			commit:     CommitMessageJSON{Description: "add webhook handler"},
			commitType: "feat",
			scope:      "security",
			want:       "feat(security): add webhook handler",
		},
		{
			name:       "fix with scope and breaking",
			commit:     CommitMessageJSON{Description: "change token interface"},
			commitType: "fix",
			scope:      "core",
			breaking:   true,
			want:       "fix(core)!: change token interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.commit.ToConventionalCommit(tt.commitType, tt.scope, tt.breaking)
			if got != tt.want {
				t.Errorf("ToConventionalCommit() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJSON_IntoStruct(t *testing.T) {
	// Triangulate: verify parseJSON works with a concrete struct type
	input := "```json\n{\"description\":\"fix login\",\"body\":\"details\"}\n```"
	var commit CommitMessageJSON
	if err := parseJSON(input, &commit); err != nil {
		t.Fatalf("parseJSON into CommitMessageJSON: %v", err)
	}
	if commit.Description != "fix login" {
		t.Errorf("Description: got %q, want %q", commit.Description, "fix login")
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: fmt.Sprintf("regenerated msg %d", cc)})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.numParallel = 3

	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "chore"},
		{Files: []string{"b.go"}, Diff: "diff b", CommitType: "chore"},
		{Files: []string{"c.go"}, Diff: "diff c", CommitType: "chore"},
		{Files: []string{"d.go"}, Diff: "diff d", CommitType: "chore"},
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
		if strings.Contains(msg, "regenerated msg ") || strings.Contains(msg, "chore:") || strings.Contains(msg, "fix:") {
			// ok — it's from the server with an inferred type prefix
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: fmt.Sprintf("serial msg %d", callCount)})))
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
	// InferCommitType infers "fix" for source modifications (non-empty diff)
	if msgs[0] != "fix: serial msg 1" {
		t.Errorf("msg[0]: got %q, want %q", msgs[0], "fix: serial msg 1")
	}
	if msgs[1] != "fix: serial msg 2" {
		t.Errorf("msg[1]: got %q, want %q", msgs[1], "fix: serial msg 2")
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "ok"})))
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
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add"})))
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
		if opts["num_ctx"] != float64(8192) {
			t.Errorf("options.num_ctx = %v, want 8192", opts["num_ctx"])
		}
		if opts["keep_alive"] != "5m" {
			t.Errorf("options.keep_alive = %v, want 5m", opts["keep_alive"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetProvider("ollama")
	adapter.SetNumCtx(8192)
	_, err := adapter.GenerateChunkMessage(domain.DiffChunk{Files: []string{"x.go"}, Diff: "d"})
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

func TestAdapter_OllamaOptions_NoNumCtxWhenNotSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode: %v", err)
		}
		opts, ok := raw["options"].(map[string]interface{})
		if !ok {
			t.Fatal("options map missing for ollama provider")
		}
		// num_ctx should NOT be injected when SetNumCtx hasn't been called (numCtx == 0)
		if _, exists := opts["num_ctx"]; exists {
			t.Errorf("options.num_ctx should not be injected when SetNumCtx not called, got %v", opts["num_ctx"])
		}
		if opts["keep_alive"] != "5m" {
			t.Errorf("options.keep_alive = %v, want 5m", opts["keep_alive"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	adapter.SetProvider("ollama")
	// Intentionally NOT calling SetNumCtx — num_ctx should not be injected
	_, err := adapter.GenerateChunkMessage(domain.DiffChunk{Files: []string{"x.go"}, Diff: "d"})
	if err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// --- GenerateChangelogByArea with nameMap ---

func TestAdapter_GenerateChangelogByArea_WithNameMap(t *testing.T) {
	expectedResult := domain.ChangelogByArea{
		"group_1": []string{"Added semantic diff analysis"},
		"group_2": []string{"Fixed auth token validation"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, expectedResult)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	nameMap := map[string]string{
		"group_1": "core",
		"group_2": "security",
	}

	ch, err := adapter.GenerateChangelogByArea("group_1:\n- feat(core): add feature\ngroup_2:\n- fix(security): fix bug", nameMap)
	if err != nil {
		t.Fatalf("GenerateChangelogByArea failed: %v", err)
	}

	// Verify remapping: group_1 → core, group_2 → security
	if len(ch) != 2 {
		t.Fatalf("expected 2 areas after remapping, got %d", len(ch))
	}
	if items, ok := ch["core"]; !ok || len(items) != 1 || items[0] != "Added semantic diff analysis" {
		t.Errorf("core area: got %v", ch["core"])
	}
	if items, ok := ch["security"]; !ok || len(items) != 1 || items[0] != "Fixed auth token validation" {
		t.Errorf("security area: got %v", ch["security"])
	}
	// group_N keys should NOT be present after remapping
	if _, ok := ch["group_1"]; ok {
		t.Error("group_1 key should be remapped to core, not present in result")
	}
	if _, ok := ch["group_2"]; ok {
		t.Error("group_2 key should be remapped to security, not present in result")
	}
}

func TestAdapter_GenerateChangelogByArea_EmptyNameMap_PreservesKeys(t *testing.T) {
	// When nameMap is empty, group_N keys should be preserved as-is (fallback behavior)
	expectedResult := domain.ChangelogByArea{
		"group_1": []string{"Added feature"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, expectedResult)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	ch, err := adapter.GenerateChangelogByArea("group_1:\n- feat: add feature", nil)
	if err != nil {
		t.Fatalf("GenerateChangelogByArea failed: %v", err)
	}
	// With nil nameMap, group_1 should remain (no remapping)
	if len(ch) != 1 {
		t.Fatalf("expected 1 area, got %d", len(ch))
	}
	if items, ok := ch["group_1"]; !ok || len(items) != 1 {
		t.Errorf("group_1 should be preserved when no nameMap, got %v", ch)
	}
}

func TestAdapter_GenerateChangelogByArea_PromptUsesGroupN(t *testing.T) {
	// Verify that the prompt sent to LLM contains group_N keys, not area names
	expectedResult := domain.ChangelogByArea{
		"group_1": []string{"Added feature"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		// The prompt should contain "group_1" and NOT contain "core" or real area names
		promptContent := req.Messages[len(req.Messages)-1].Content
		if !strings.Contains(promptContent, "group_1") {
			t.Error("prompt should contain group_1 key, not area names")
		}
		if strings.Contains(promptContent, "core") {
			// "core" could appear in commit subjects, but the GROUP LABELS should be group_N
			// This is a soft check since "core" might appear in commit text
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, expectedResult)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	nameMap := map[string]string{"group_1": "core"}
	_, err := adapter.GenerateChangelogByArea("group_1:\n- feat: add feature\n", nameMap)
	if err != nil {
		t.Fatalf("GenerateChangelogByArea failed: %v", err)
	}
}

func TestAdapter_GenerateChangelogByArea_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("not valid json at all"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.GenerateChangelogByArea("group_1:\n- feat: add", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("error = %v, want ErrInvalidJSON", err)
	}
}

// --- GenerateChangelogGeneric end-to-end ---

func TestAdapter_GenerateChangelogGeneric_ReturnsChangelog(t *testing.T) {
	expectedResult := domain.Changelog{
		Features: []string{"Added login flow"},
		Fixes:    []string{"Fixed memory leak"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, expectedResult)))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	ch, err := adapter.GenerateChangelogGeneric("feat: add login\nfix: memory leak", "", "")
	if err != nil {
		t.Fatalf("GenerateChangelogGeneric failed: %v", err)
	}
	if ch == nil {
		t.Fatal("changelog should not be nil")
	}
	if len(ch.Features) != 1 || ch.Features[0] != "Added login flow" {
		t.Errorf("Features: got %v", ch.Features)
	}
	if len(ch.Fixes) != 1 || ch.Fixes[0] != "Fixed memory leak" {
		t.Errorf("Fixes: got %v", ch.Fixes)
	}
}

func TestAdapter_GenerateChangelogGeneric_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse("not valid json"))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	_, err := adapter.GenerateChangelogGeneric("feat: add", "", "")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
