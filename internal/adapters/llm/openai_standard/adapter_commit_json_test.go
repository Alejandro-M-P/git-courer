package openai_standard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// TestGenerateChunkMessage_AnnotatedEntriesMarshalledToJSON verifies that when
// a chunk carries AnnotatedEntries, GenerateChunkMessage marshals them to JSON
// and the rendered prompt contains the annotated_diff JSON block (not the
// legacy emoji AnnotatedDiff and not the raw diff).
func TestGenerateChunkMessage_AnnotatedEntriesMarshalledToJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		userMsg := req.Messages[1].Content
		if !strings.Contains(userMsg, "annotated_diff") {
			t.Errorf("prompt should contain annotated_diff JSON block; got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, `"symbol":"Handler"`) {
			t.Errorf("prompt should contain the marshalled Handler entry; got:\n%s", userMsg)
		}
		// When AnnotatedEntries is non-empty, the raw diff must NOT be sent.
		if strings.Contains(userMsg, "raw diff content") {
			t.Errorf("prompt should NOT contain raw diff when AnnotatedEntries is present; got:\n%s", userMsg)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add handler"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files: []string{"internal/api/handler.go"},
		Diff:  "raw diff content",
		AnnotatedEntries: []domain.AnnotatedEntry{
			{File: "internal/api/handler.go", Symbol: "Handler", Type: "NEW_FUNC", Line: 10, After: "+ func Handler() {}"},
		},
		CallGraph: []domain.CallGraphEntry{
			{From: "internal/api/handler.go", To: "internal/api/util.go", Symbol: "Helper"},
		},
		CFGBefore: &domain.CFGCount{Branch: 0, Loop: 0, Return: 0, Error: 0},
		CFGAfter:  &domain.CFGCount{Branch: 1, Loop: 0, Return: 0, Error: 0},
		CommitType: "feat",
	}
	if _, err := adapter.GenerateChunkMessage(chunk); err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// TestGenerateChunkMessage_CallGraphJSONInPrompt verifies that the call_graph
// JSON block is rendered in the prompt when the chunk has CallGraph entries.
func TestGenerateChunkMessage_CallGraphJSONInPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		userMsg := req.Messages[1].Content
		if !strings.Contains(userMsg, "call_graph") {
			t.Errorf("prompt should contain call_graph block; got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, `"symbol":"Helper"`) {
			t.Errorf("prompt should contain Helper call edge; got:\n%s", userMsg)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "wire helper"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files: []string{"a.go"},
		Diff:  "diff",
		AnnotatedEntries: []domain.AnnotatedEntry{
			{File: "a.go", Symbol: "F", Type: "NEW_FUNC", Line: 1},
		},
		CallGraph: []domain.CallGraphEntry{
			{From: "a.go", To: "b.go", Symbol: "Helper"},
		},
		CommitType: "feat",
	}
	if _, err := adapter.GenerateChunkMessage(chunk); err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// TestGenerateChunkMessage_CFGSummaryJSONInPrompt verifies the cfg block is
// rendered when CFGBefore/CFGAfter are present, and that conditionals carry
// before/after counts.
func TestGenerateChunkMessage_CFGSummaryJSONInPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		userMsg := req.Messages[1].Content
		if !strings.Contains(userMsg, "cfg") {
			t.Errorf("prompt should contain cfg block; got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, `"conditionals"`) {
			t.Errorf("prompt should contain conditionals entry; got:\n%s", userMsg)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add branch"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files: []string{"a.go"},
		Diff:  "diff",
		AnnotatedEntries: []domain.AnnotatedEntry{
			{File: "a.go", Symbol: "F", Type: "MOD_BODY_LOGIC", Line: 1},
		},
		CFGBefore: &domain.CFGCount{Branch: 0, Loop: 1, Return: 2, Error: 0},
		CFGAfter:  &domain.CFGCount{Branch: 1, Loop: 1, Return: 2, Error: 0},
		CommitType: "fix",
	}
	if _, err := adapter.GenerateChunkMessage(chunk); err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// TestGenerateChunkMessage_CFGNullWhenNotComputed verifies the cfg block is
// omitted (null) when CFGBefore/CFGAfter are nil (not computed).
func TestGenerateChunkMessage_CFGNullWhenNotComputed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		userMsg := req.Messages[1].Content
		// annotated_diff must still be present (entries are non-empty).
		if !strings.Contains(userMsg, "annotated_diff") {
			t.Errorf("prompt should contain annotated_diff; got:\n%s", userMsg)
		}
		// cfg must NOT be present when CFGBefore/CFGAfter are nil.
		if strings.Contains(userMsg, "cfg") {
			t.Errorf("prompt should NOT contain cfg block when CFG not computed; got:\n%s", userMsg)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files: []string{"a.go"},
		Diff:  "diff",
		AnnotatedEntries: []domain.AnnotatedEntry{
			{File: "a.go", Symbol: "F", Type: "NEW_FUNC", Line: 1},
		},
		CommitType: "feat",
	}
	if _, err := adapter.GenerateChunkMessage(chunk); err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}

// TestGenerateChunkMessage_FallsBackToRawDiffWhenNoEntries verifies the
// regression: when AnnotatedEntries is empty, the raw diff is used (no
// annotated_diff block). This preserves the existing fallback contract.
func TestGenerateChunkMessage_FallsBackToRawDiffWhenNoEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		userMsg := req.Messages[1].Content
		if strings.Contains(userMsg, "annotated_diff") {
			t.Errorf("prompt should NOT contain annotated_diff when entries empty; got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, "raw diff here") {
			t.Errorf("prompt should contain raw diff fallback; got:\n%s", userMsg)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatCompletionResponse(mockJSONResponse(t, CommitMessageJSON{Description: "add"})))
	}))
	defer server.Close()

	adapter := newTestAdapter(server)
	chunk := domain.DiffChunk{
		Files:      []string{"a.go"},
		Diff:       "raw diff here",
		CommitType: "feat",
	}
	if _, err := adapter.GenerateChunkMessage(chunk); err != nil {
		t.Fatalf("GenerateChunkMessage failed: %v", err)
	}
}