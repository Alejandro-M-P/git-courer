package openai_standard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// TestRegenerateChunk_AnnotatedEntriesMarshalledToJSON verifies that
// regenerateChunk (used by RegenerateMessage) marshals AnnotatedEntries to
// JSON and renders the annotated_diff block in the regenerate prompt, with
// the rejected message context.
func TestRegenerateChunk_AnnotatedEntriesMarshalledToJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		userMsg := req.Messages[1].Content
		if !strings.Contains(userMsg, "annotated_diff") {
			t.Errorf("regenerate prompt should contain annotated_diff JSON block; got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, `"symbol":"Handler"`) {
			t.Errorf("regenerate prompt should contain marshalled Handler entry; got:\n%s", userMsg)
		}
		// Rejected message context must be present.
		if !strings.Contains(userMsg, "Rejected Message") {
			t.Errorf("regenerate prompt should contain Rejected Message block; got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, "previous bad message") {
			t.Errorf("regenerate prompt should contain the rejected message text; got:\n%s", userMsg)
		}
		// Raw diff must NOT be sent when AnnotatedEntries is present.
		if strings.Contains(userMsg, "raw diff content") {
			t.Errorf("regenerate prompt should NOT contain raw diff when entries present; got:\n%s", userMsg)
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
		CommitType: "feat",
	}
	if _, err := adapter.regenerateChunk(chunk, "previous bad message"); err != nil {
		t.Fatalf("regenerateChunk failed: %v", err)
	}
}

// TestRegenerateChunk_FallsBackToRawDiffWhenNoEntries verifies the regenerate
// path falls back to the raw diff when AnnotatedEntries is empty.
func TestRegenerateChunk_FallsBackToRawDiffWhenNoEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		userMsg := req.Messages[1].Content
		if strings.Contains(userMsg, "annotated_diff") {
			t.Errorf("regenerate prompt should NOT contain annotated_diff when entries empty; got:\n%s", userMsg)
		}
		if !strings.Contains(userMsg, "raw diff here") {
			t.Errorf("regenerate prompt should contain raw diff fallback; got:\n%s", userMsg)
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
	if _, err := adapter.regenerateChunk(chunk, "feedback"); err != nil {
		t.Fatalf("regenerateChunk failed: %v", err)
	}
}