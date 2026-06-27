package session

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newHandlerWithStoreAndActive builds a Handler with a MockGit, MockSessionStore,
// and an activeSession atomic.Value pre-seeded with a typed nil. Returns the
// activeSession so tests can read it back.
func newHandlerWithStoreAndActive(t *testing.T) (*Handler, *MockGit, *MockSessionStore, *atomic.Value) {
	t.Helper()
	mockGit := new(MockGit)
	store := new(MockSessionStore)
	active := &atomic.Value{}
	active.Store((*domain.Session)(nil))
	h := NewHandlerWithStore(mockGit, store, t.TempDir(), active)
	h.metaDir = t.TempDir()
	return h, mockGit, store, active
}

// ─── select ──────────────────────────────────────────────────────────────

func TestHandleSelect_ValidSession_PublishesToActiveSession(t *testing.T) {
	h, mockGit, store, active := newHandlerWithStoreAndActive(t)
	_ = mockGit
	sess := fixtureSession() // Status = active

	store.On("Get", "fix-bug").Return(sess, nil)

	args := map[string]any{"command": "select", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	require.NotNil(t, res)

	// activeSession should now hold our session.
	got := active.Load()
	s, ok := got.(*domain.Session)
	require.True(t, ok)
	assert.Equal(t, "fix-bug", s.ID)
	assert.Equal(t, "../git-courer-worktrees/fix-bug", s.Worktree)

	// Result body should be the session JSON, including the worktree.
	text := resultText(t, res)
	assert.Contains(t, text, `"id":"fix-bug"`)
	assert.Contains(t, text, `"worktree":"../git-courer-worktrees/fix-bug"`)
	store.AssertExpectations(t)
}

func TestHandleSelect_MissingSessionID_Errors(t *testing.T) {
	h, _, _, _ := newHandlerWithStoreAndActive(t)
	args := map[string]any{"command": "select"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err) // errors come back as JSON results, not Go errors
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"error"`)
	assert.Contains(t, text, "session_id is required")
}

func TestHandleSelect_StoreError_Propagates(t *testing.T) {
	h, _, store, active := newHandlerWithStoreAndActive(t)
	store.On("Get", "missing").Return((*domain.Session)(nil), errors.New("not found"))

	args := map[string]any{"command": "select", "session_id": "missing"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"error"`)
	assert.Contains(t, text, "not found")

	// activeSession should remain the seeded nil.
	v := active.Load()
	if s, ok := v.(*domain.Session); !ok || s != nil {
		t.Errorf("activeSession should still be nil after failed select, got %T %v", v, v)
	}
	store.AssertExpectations(t)
}

func TestHandleSelect_NonActiveSession_Rejected(t *testing.T) {
	h, _, store, active := newHandlerWithStoreAndActive(t)
	finished := fixtureSession()
	finished.Status = domain.SessionFinished
	store.On("Get", "fix-bug").Return(finished, nil)

	args := map[string]any{"command": "select", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"error"`)
	assert.Contains(t, text, "not active")

	v := active.Load()
	if s, ok := v.(*domain.Session); ok && s != nil {
		t.Errorf("activeSession must not be set when session is not active, got %v", s)
	}
	store.AssertExpectations(t)
}

// ─── status list mode ────────────────────────────────────────────────────

func TestHandleStatus_NoSessionID_ListsAllSessions(t *testing.T) {
	h, _, store, _ := newHandlerWithStoreAndActive(t)
	list := []*domain.Session{
		{ID: "s1", Status: domain.SessionActive, Worktree: "/wt/s1"},
		{ID: "s2", Status: domain.SessionFinished, Worktree: "/wt/s2"},
	}
	store.On("List").Return(list, nil)

	args := map[string]any{"command": "status"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)

	var got []*domain.Session
	require.NoError(t, json.Unmarshal([]byte(resultText(t, res)), &got))
	assert.Len(t, got, 2)
	assert.Equal(t, "s1", got[0].ID)
	assert.Equal(t, "s2", got[1].ID)
	store.AssertExpectations(t)
}

func TestHandleStatus_WithSessionID_ReturnsSingle(t *testing.T) {
	h, _, store, _ := newHandlerWithStoreAndActive(t)
	sess := fixtureSession()
	store.On("Get", "fix-bug").Return(sess, nil)

	args := map[string]any{"command": "status", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	text := resultText(t, res)
	assert.Contains(t, text, `"id":"fix-bug"`)
	store.AssertExpectations(t)
}

// ─── finish clears active session ─────────────────────────────────────────

func TestHandleFinish_ClearsActiveSession_WhenIDMatches(t *testing.T) {
	h, mockGit, store, active := newHandlerWithStoreAndActive(t)
	sess := fixtureSession()

	// Pre-select the session so we can verify finish clears it.
	active.Store(sess)

	mockGit.On("Status").Return(cleanStatus(), nil).Times(2)
	mockGit.On("Head").Return(fixedBaseCommit, nil)
	mockGit.On("Merge", "main").Return("", nil)
	mockGit.On("MergeAbort").Return("", nil)
	mockGit.On("Reset", "--hard", fixedBaseCommit).Return("", nil)
	mockGit.On("MergeBase", "main", "fix-bug").Return("", assertError("none"))
	mockGit.On("GitCommonDir").Return(".git", nil)
	mockGit.On("Switch", "main").Return(nil)
	mockGit.On("Merge", "fix-bug").Return("", nil)
	mockGit.On("RemoveWorktree", "../git-courer-worktrees/fix-bug").Return(nil)
	mockGit.On("DeleteBranch", "fix-bug", true).Return("", nil)

	store.On("Get", "fix-bug").Return(sess, nil)
	store.On("Delete", "fix-bug").Return(nil)

	args := map[string]any{"command": "finish", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"success"`)

	// activeSession should now be nil.
	v := active.Load()
	if s, ok := v.(*domain.Session); ok && s != nil {
		t.Errorf("activeSession should be cleared after finish, got %v", s)
	}
	mockGit.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestHandleFinish_DoesNotClearActiveSession_WhenIDMismatches(t *testing.T) {
	h, mockGit, store, active := newHandlerWithStoreAndActive(t)
	// A *different* session is selected.
	other := fixtureSession()
	other.ID = "other-session"
	other.Branch = "other-session"
	other.Worktree = "../git-courer-worktrees/other-session"
	active.Store(other)

	// Finish a different session (fix-bug).
	sess := fixtureSession()
	mockGit.On("Status").Return(cleanStatus(), nil).Times(2)
	mockGit.On("Head").Return(fixedBaseCommit, nil)
	mockGit.On("Merge", "main").Return("", nil)
	mockGit.On("MergeAbort").Return("", nil)
	mockGit.On("Reset", "--hard", fixedBaseCommit).Return("", nil)
	mockGit.On("MergeBase", "main", "fix-bug").Return("", assertError("none"))
	mockGit.On("GitCommonDir").Return(".git", nil)
	mockGit.On("Switch", "main").Return(nil)
	mockGit.On("Merge", "fix-bug").Return("", nil)
	mockGit.On("RemoveWorktree", "../git-courer-worktrees/fix-bug").Return(nil)
	mockGit.On("DeleteBranch", "fix-bug", true).Return("", nil)

	store.On("Get", "fix-bug").Return(sess, nil)
	store.On("Delete", "fix-bug").Return(nil)

	args := map[string]any{"command": "finish", "session_id": "fix-bug"}
	_, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)

	// activeSession should still hold the other session.
	v := active.Load()
	s, ok := v.(*domain.Session)
	require.True(t, ok)
	require.NotNil(t, s)
	assert.Equal(t, "other-session", s.ID)
	mockGit.AssertExpectations(t)
	store.AssertExpectations(t)
}

// ─── unknown command suggestion includes select ──────────────────────────

func TestHandleSession_UnknownCommandSuggestsSelect(t *testing.T) {
	h, _, _, _ := newHandlerWithStoreAndActive(t)
	// "selct" is close to "select" — suggestion should fire.
	args := map[string]any{"command": "selct"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"error"`)
	// Suggestion should mention one of the known commands.
	assert.Contains(t, text, "Did you mean")
}

// ensure mock package import is used (Some tests rely on mock.MatchedBy etc.)
var _ = mock.MatchedBy