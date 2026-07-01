package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fixedBaseCommit is a 40-char SHA-1 fixture used across session tests.
const fixedBaseCommit = "0123456789abcdef0123456789abcdef01234567"

// sessionReq builds a CallToolRequest for the session tool with the given args.
func sessionReq(args map[string]any) mcpgo.CallToolRequest {
	return mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "session",
			Arguments: args,
		},
	}
}

// resultText extracts the text content from a non-nil MCP tool result.
func resultText(t *testing.T, res *mcpgo.CallToolResult) string {
	t.Helper()
	require.NotNil(t, res, "result must not be nil")
	require.NotEmpty(t, res.Content, "result content must not be empty")
	tc, ok := res.Content[0].(mcpgo.TextContent)
	require.True(t, ok, "result content[0] must be TextContent")
	return tc.Text
}

// ─── Pure function: computeSessionID (spec: Deterministic Session ID) ───────

func TestComputeSessionID(t *testing.T) {
	t.Run("same goal yields same slug", func(t *testing.T) {
		a := computeSessionID("fix bug")
		b := computeSessionID("fix bug")
		assert.Equal(t, a, b, "deterministic: same goal must produce same slug")
	})

	t.Run("different goals yield different slugs", func(t *testing.T) {
		a := computeSessionID("fix bug")
		b := computeSessionID("add tests")
		assert.NotEqual(t, a, b, "distinct goals must produce distinct slugs")
	})

	t.Run("lowercasing", func(t *testing.T) {
		assert.Equal(t, "fix-bug", computeSessionID("Fix BUG"))
	})

	t.Run("hyphenation of non-alphanumeric", func(t *testing.T) {
		assert.Equal(t, "fix-bug-v2", computeSessionID("fix: bug! (v2)"))
	})

	t.Run("consecutive separators collapse", func(t *testing.T) {
		assert.Equal(t, "fix-bug", computeSessionID("fix   bug"))
	})

	t.Run("leading and trailing separators trimmed", func(t *testing.T) {
		assert.Equal(t, "fix-bug", computeSessionID("  fix bug  "))
	})

	t.Run("truncation at 50 characters", func(t *testing.T) {
		long := strings.Repeat("a", 80)
		id := computeSessionID(long)
		assert.LessOrEqual(t, len(id), 50, "slug must not exceed 50 chars")
		assert.False(t, strings.HasSuffix(id, "-"), "truncated slug must not end with '-'")
	})
}

// ─── Dispatch + handleStart (table-driven, mirrors branch/branch_test.go) ───

func TestHandleSession(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        map[string]any
		setup       func(*MockGit)
		wantInJSON  string // substring expected in successful JSON result
		wantErr     bool   // true if expecting an error message in the result
		errContain  string // substring expected in error JSON result
		wantIsError bool   // when true, expect result to be an error result
	}{
		{
			name:    "start success creates branch worktree and returns id",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug"},
			setup: func(m *MockGit) {
				id := computeSessionID("fix bug")
				m.On("Head").Return(fixedBaseCommit, nil)
				m.On("CreateRef", "refs/heads/"+id, fixedBaseCommit).Return(nil)
				m.On("AddWorktree", "../git-courer-worktrees/"+id, ""+id).
					Return("../git-courer-worktrees/"+id, nil)
			},
			wantInJSON: computeSessionID("fix bug"),
		},
		{
			name:    "start with custom branch creates branch with that name",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug", "branch": "feature/fix-auth"},
			setup: func(m *MockGit) {
				m.On("Head").Return(fixedBaseCommit, nil)
				m.On("CreateRef", "refs/heads/feature/fix-auth", fixedBaseCommit).Return(nil)
				m.On("AddWorktree", "../git-courer-worktrees/fix-bug", "feature/fix-auth").
					Return("../git-courer-worktrees/fix-bug", nil)
			},
			wantInJSON: "fix-bug",
		},
		{
			name:    "start with custom branch that already exists returns error",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug", "branch": "feature/fix-auth"},
			setup: func(m *MockGit) {
				m.On("Head").Return(fixedBaseCommit, nil)
				m.On("CreateRef", "refs/heads/feature/fix-auth", fixedBaseCommit).Return(assertError("ref exists"))
			},
			wantErr:    true,
			errContain: "branch already exists",
		},
		{
			name:    "start collision retries with -2 suffix and succeeds",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug"},
			setup: func(m *MockGit) {
				id := computeSessionID("fix bug")
				m.On("Head").Return(fixedBaseCommit, nil)
				// First attempt fails (ref exists), second with -2 succeeds.
				m.On("CreateRef", "refs/heads/"+id, fixedBaseCommit).
					Return(assertError("ref exists")).Once()
				m.On("CreateRef", "refs/heads/"+id+"-2", fixedBaseCommit).
					Return(nil).Once()
				m.On("AddWorktree", "../git-courer-worktrees/"+id+"-2", ""+id+"-2").
					Return("../git-courer-worktrees/"+id+"-2", nil)
			},
			wantInJSON: computeSessionID("fix bug") + "-2",
		},
		{
			name:    "start worktree failure rolls back branch ref",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug"},
			setup: func(m *MockGit) {
				id := computeSessionID("fix bug")
				m.On("Head").Return(fixedBaseCommit, nil)
				m.On("CreateRef", "refs/heads/"+id, fixedBaseCommit).Return(nil)
				m.On("AddWorktree", "../git-courer-worktrees/"+id, ""+id).
					Return("", assertError("worktree add failed"))
				// Rollback: UpdateRef with empty commit hash deletes the ref.
				m.On("UpdateRef", "refs/heads/"+id, "").Return("", nil)
			},
			wantErr:    true,
			errContain: "worktree",
		},
		{
			name:    "start collision exhausted returns error",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug"},
			setup: func(m *MockGit) {
				id := computeSessionID("fix bug")
				m.On("Head").Return(fixedBaseCommit, nil)
				// Attempt 0: bare id. Attempts 1..9: -2..-10. All fail.
				m.On("CreateRef", "refs/heads/"+id, fixedBaseCommit).
					Return(assertError("ref exists")).Once()
				for i := 1; i <= 9; i++ {
					m.On("CreateRef", "refs/heads/"+id+"-"+strconv.Itoa(i+1), fixedBaseCommit).
						Return(assertError("ref exists")).Once()
				}
			},
			wantErr:    true,
			errContain: "exhausted",
		},
		{
			name:       "start missing agent returns error",
			command:    "start",
			args:       map[string]any{"goal": "fix bug"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "agent",
		},
		{
			name:       "start missing goal returns error",
			command:    "start",
			args:       map[string]any{"agent": "claude"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "goal",
		},
		{
			name:       "finish command missing session_id returns error",
			command:    "finish",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "session_id is required for finish",
		},
		{
			name:       "status command without store returns error",
			command:    "status",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "session store not configured",
		},
		{
			name:       "discard command missing session_id returns error",
			command:    "discard",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "session_id is required for discard",
		},
		{
			name:       "discard without confirmed returns error",
			command:    "discard",
			args:       map[string]any{"session_id": "fix-bug"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "confirmed",
		},
		{
			name:       "unknown command returns error",
			command:    "bogus",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "empty command returns error",
			command:    "",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "command is required",
		},
		{
			name:       "unknown param returns error",
			command:    "start",
			args:       map[string]any{"agent": "claude", "goal": "fix bug", "bogus": "x"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "unknown parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGit)
			h := NewHandler(mockGit)
			// Redirect metadata writes to a temp dir so tests never touch the
			// real .git/git-courer/sessions path.
			h.metaDir = t.TempDir()
			tt.setup(mockGit)

			args := map[string]any{}
			if tt.command != "" {
				args["command"] = tt.command
			}
			for k, v := range tt.args {
				args[k] = v
			}

			res, err := h.HandleSession(context.Background(), sessionReq(args))

			assert.NoError(t, err, "errors must be returned in the result, not as Go error")
			if tt.wantErr {
				assert.NotNil(t, res, "error result must not be nil")
				if res != nil && len(res.Content) > 0 {
					text := resultText(t, res)
					assert.Contains(t, text, tt.errContain)
				}
			} else {
				assert.NotNil(t, res)
				if res != nil && len(res.Content) > 0 {
					text := resultText(t, res)
					assert.Contains(t, text, tt.wantInJSON)
				}
			}
			mockGit.AssertExpectations(t)
		})
	}
}

// ─── Metadata persistence (spec: Metadata Persistence) ─────────────────────

func TestHandleStart_MetadataWritten(t *testing.T) {
	mockGit := new(MockGit)
	h := NewHandler(mockGit)
	metaDir := t.TempDir()
	h.metaDir = metaDir

	id := computeSessionID("fix auth")
	mockGit.On("Head").Return(fixedBaseCommit, nil)
	mockGit.On("CreateRef", "refs/heads/"+id, fixedBaseCommit).Return(nil)
	mockGit.On("AddWorktree", "../git-courer-worktrees/"+id, ""+id).
		Return("../git-courer-worktrees/"+id, nil)

	args := map[string]any{"command": "start", "agent": "claude", "goal": "fix auth"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	require.NotNil(t, res)

	// The metadata file MUST exist at {metaDir}/{id}.json and parse as JSON
	// with all six required fields.
	metaPath := filepath.Join(metaDir, id+".json")
	data, err := os.ReadFile(metaPath)
	require.NoError(t, err, "metadata file must be written at %s", metaPath)

	var meta SessionMeta
	require.NoError(t, json.Unmarshal(data, &meta), "metadata must be valid JSON")
	assert.Equal(t, id, meta.ID, "metadata id must match session id")
	assert.Equal(t, "claude", meta.Agent)
	assert.Equal(t, "fix auth", meta.Goal)
	assert.Equal(t, ""+id, meta.Branch)
	assert.Equal(t, "../git-courer-worktrees/"+id, meta.Worktree)
	assert.Equal(t, fixedBaseCommit, meta.BaseCommit)
	assert.NotEmpty(t, meta.CreatedAt, "created_at must be set")
	// created_at must be RFC3339-parseable.
	_, perr := parseRFC3339(meta.CreatedAt)
	assert.NoError(t, perr, "created_at must be RFC3339: %s", meta.CreatedAt)

	mockGit.AssertExpectations(t)
}

func TestHandleStart_MetadataDoesNotTouchCommitstore(t *testing.T) {
	mockGit := new(MockGit)
	h := NewHandler(mockGit)
	metaDir := t.TempDir()
	h.metaDir = metaDir
	// Simulate pre-existing commitstore data inside the metadata root.
	commitstoreDir := filepath.Join(metaDir, "..", "commitstore")
	require.NoError(t, os.MkdirAll(commitstoreDir, 0o755))
	probe := filepath.Join(commitstoreDir, "probe.json")
	require.NoError(t, os.WriteFile(probe, []byte("preserve-me"), 0o644))

	id := computeSessionID("preserve check")
	mockGit.On("Head").Return(fixedBaseCommit, nil)
	mockGit.On("CreateRef", "refs/heads/"+id, fixedBaseCommit).Return(nil)
	mockGit.On("AddWorktree", "../git-courer-worktrees/"+id, ""+id).
		Return("../git-courer-worktrees/"+id, nil)

	args := map[string]any{"command": "start", "agent": "claude", "goal": "preserve check"}
	_, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)

	got, err := os.ReadFile(probe)
	require.NoError(t, err, "commitstore probe file must be untouched")
	assert.Equal(t, "preserve-me", string(got))
}

// ─── Return shape (spec: Return Shape) ──────────────────────────────────────

func TestHandleSession_ReturnShape(t *testing.T) {
	t.Run("start returns exactly id branch worktree base_commit", func(t *testing.T) {
		mockGit := new(MockGit)
		h := NewHandler(mockGit)
		h.metaDir = t.TempDir()

		id := computeSessionID("fix auth")
		mockGit.On("Head").Return(fixedBaseCommit, nil)
		mockGit.On("CreateRef", "refs/heads/"+id, fixedBaseCommit).Return(nil)
		mockGit.On("AddWorktree", "../git-courer-worktrees/"+id, ""+id).
			Return("../git-courer-worktrees/"+id, nil)

		args := map[string]any{"command": "start", "agent": "claude", "goal": "fix auth"}
		res, err := h.HandleSession(context.Background(), sessionReq(args))
		require.NoError(t, err)

		text := resultText(t, res)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(text), &parsed), "result must be valid JSON")

		// Exactly these four keys (no success/operation/message wrapper — spec
		// says the tool result has exactly {id, branch, worktree, base_commit}).
		expectedKeys := map[string]bool{"id": true, "branch": true, "worktree": true, "base_commit": true}
		for k := range parsed {
			assert.True(t, expectedKeys[k], "unexpected key in result: %s", k)
		}
		for k := range expectedKeys {
			_, ok := parsed[k]
			assert.True(t, ok, "missing required key in result: %s", k)
		}
		assert.Equal(t, id, parsed["id"])
		assert.Equal(t, ""+id, parsed["branch"])
		assert.Equal(t, fixedBaseCommit, parsed["base_commit"])
	})
}

// ─── Registration (spec: Tool registered on initialize) ────────────────────
// Verifies the `session` tool is registered with the correct name and the
// command enum declares start/finish/status/discard. The full wantTools list
// is checked at the mcp package level (registration_v2_test.go).
func TestRegister_ToolNameAndCommandEnum(t *testing.T) {
	mockGit := new(MockGit)
	h := NewHandler(mockGit)

	srv := server.NewMCPServer("test", "1.0")
	assert.NotPanics(t, func() { Register(srv, h) })

	tools := srv.ListTools()
	st, ok := tools["session"]
	require.True(t, ok, "session tool must be registered")
	require.NotNil(t, st)
	tool := &st.Tool
	assert.Equal(t, "session", tool.Name)

	// The command parameter must be an enum with all five commands.
	props := tool.InputSchema.Properties
	require.NotNil(t, props)
	cmdProp, ok := props["command"].(map[string]any)
	require.True(t, ok, "command property must exist in schema")
	enumRaw, ok := cmdProp["enum"].([]string)
	require.True(t, ok, "command must have a string enum")
	assert.Equal(t, []string{"start", "finish", "status", "select", "discard"}, enumRaw)

	// agent + goal params must be declared.
	_, hasAgent := props["agent"]
	assert.True(t, hasAgent, "agent param must be declared")
	_, hasGoal := props["goal"]
	assert.True(t, hasGoal, "goal param must be declared")
}

// assertError is a tiny helper returning a non-nil error for mock Return args.
func assertError(msg string) error { return &simpleError{msg: msg} }

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

// parseRFC3339 validates an RFC3339 timestamp without importing time twice.
func parseRFC3339(s string) (any, error) {
	return parseTime(s)
}

// ─── finish / status / discard (spec: session finish lifecycle) ────────────

// newHandlerWithStore builds a Handler with a MockGit and a MockSessionStore,
// redirecting metadata writes to a temp dir. Used by finish/status/discard
// tests so the canonical domain.Session path is exercised. The session Handler
// no longer carries a separate main-repo git adapter since finish no longer
// merges — there is one git port for the worktree/preview/cleanup path.
func newHandlerWithStore(t *testing.T) (*Handler, *MockGit, *MockSessionStore) {
	t.Helper()
	mockGit := new(MockGit)
	store := new(MockSessionStore)
	active := &atomic.Value{}
	active.Store((*domain.Session)(nil))
	h := NewHandlerWithStore(mockGit, store, t.TempDir(), active)
	h.metaDir = t.TempDir()
	return h, mockGit, store
}

// fixtureSession returns a canonical domain.Session for finish/status/discard
// tests. BaseBranch defaults to "main" when empty in the workflow.
func fixtureSession() *domain.Session {
	return &domain.Session{
		ID:         "fix-bug",
		Agent:      "claude",
		Goal:       "fix bug",
		Branch:     "fix-bug",
		Worktree:   "../git-courer-worktrees/fix-bug",
		BaseCommit: fixedBaseCommit,
		BaseBranch: "main",
		CreatedAt:  "2025-01-01T00:00:00Z",
		Status:     domain.SessionActive,
	}
}

// cleanStatus returns a domain.Status with IsClean=true (no uncommitted work).
func cleanStatus() domain.Status {
	return domain.Status{IsClean: true, Branch: "fix-bug", Files: []domain.FileStatus{}}
}

func TestHandleFinish_Success(t *testing.T) {
	h, mockGit, store := newHandlerWithStore(t)
	sess := fixtureSession()

	// PreviewLight: clean status + MergeBase (returns no merge base → empty
	// diff stats). No Head/Merge/MergeAbort/Reset expectations because
	// PreviewLight does not run them.
	mockGit.On("Status").Return(cleanStatus(), nil)
	mockGit.On("MergeBase", "main", "fix-bug").Return("", assertError("none"))
	mockGit.On("RemoveWorktree", "../git-courer-worktrees/fix-bug").Return(nil)
	// NOTE: DeleteBranch is intentionally NOT expected — the branch stays alive.

	store.On("Get", "fix-bug").Return(sess, nil)
	store.On("Delete", "fix-bug").Return(nil)

	args := map[string]any{"command": "finish", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	assert.NoError(t, err)
	require.NotNil(t, res)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"success"`)
	assert.Contains(t, text, `"branch_alive":true`)
	mockGit.AssertExpectations(t)
	store.AssertExpectations(t)
}

// TestHandleFinish_UncommittedAborts verifies that uncommitted changes are the
// ONLY abort gate: when the worktree is dirty, finish returns preview_failed,
// preserves the worktree and metadata, and never reaches cleanup.
func TestHandleFinish_UncommittedAborts(t *testing.T) {
	h, mockGit, store := newHandlerWithStore(t)
	sess := fixtureSession()

	dirtyStatus := domain.Status{
		IsClean: false,
		Branch:  "fix-bug",
		Files:   []domain.FileStatus{{Path: "x.go", Status: "M "}},
	}
	mockGit.On("Status").Return(dirtyStatus, nil)
	mockGit.On("MergeBase", "main", "fix-bug").Return("", assertError("none"))
	// No RemoveWorktree / DeleteBranch / store.Delete expectations: abort keeps
	// everything in place.

	store.On("Get", "fix-bug").Return(sess, nil)

	args := map[string]any{"command": "finish", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	assert.NoError(t, err)
	require.NotNil(t, res)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"preview_failed"`)
	assert.Contains(t, text, "uncommitted")
	mockGit.AssertExpectations(t)
	store.AssertExpectations(t)
}

// TestHandleFinish_CleanupFailure verifies that a worktree-removal failure is
// reported as cleanup_failed, but the branch is STILL alive (no DeleteBranch is
// called during cleanup) and the merge step was never attempted.
func TestHandleFinish_CleanupFailure(t *testing.T) {
	h, mockGit, store := newHandlerWithStore(t)
	sess := fixtureSession()

	mockGit.On("Status").Return(cleanStatus(), nil)
	mockGit.On("MergeBase", "main", "fix-bug").Return("", assertError("none"))
	mockGit.On("RemoveWorktree", "../git-courer-worktrees/fix-bug").Return(assertError("worktree busy"))
	// DeleteBranch is NOT expected — cleanup only removes the worktree now.

	store.On("Get", "fix-bug").Return(sess, nil)
	store.On("Save", mock.MatchedBy(func(s *domain.Session) bool {
		return s.Status == domain.SessionCleanupFailed
	})).Return(nil)

	args := map[string]any{"command": "finish", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	assert.NoError(t, err)
	require.NotNil(t, res)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"cleanup_failed"`)
	// Even on cleanup failure, the branch is reported alive — the user can
	// still integrate or discard it manually.
	assert.Contains(t, text, `"branch_alive":true`)
	mockGit.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestHandleFinish_NotFound(t *testing.T) {
	h, _, store := newHandlerWithStore(t)
	store.On("Get", "nope").Return((*domain.Session)(nil), assertError("session not found"))

	args := map[string]any{"command": "finish", "session_id": "nope"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	assert.NoError(t, err)
	require.NotNil(t, res)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"not_found"`)
	store.AssertExpectations(t)
}

func TestHandleStatus_ReturnsSessionState(t *testing.T) {
	h, _, store := newHandlerWithStore(t)
	sess := fixtureSession()
	store.On("Get", "fix-bug").Return(sess, nil)

	args := map[string]any{"command": "status", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	assert.NoError(t, err)
	require.NotNil(t, res)
	text := resultText(t, res)
	var got domain.Session
	require.NoError(t, json.Unmarshal([]byte(text), &got))
	assert.Equal(t, "fix-bug", got.ID)
	assert.Equal(t, domain.SessionActive, got.Status)
	assert.Equal(t, "fix-bug", got.Branch)
	store.AssertExpectations(t)
}

func TestHandleDiscard_RemovesWorktreeBranchMetadata(t *testing.T) {
	h, mockGit, store, active := newHandlerWithStoreAndActive(t)
	sess := fixtureSession()

	// Pre-select the session so we can verify discard clears it, mirroring
	// the finish test in session_select_test.go.
	active.Store(sess)

	store.On("Get", "fix-bug").Return(sess, nil)
	mockGit.On("RemoveWorktree", "../git-courer-worktrees/fix-bug").Return(nil)
	mockGit.On("DeleteBranch", "fix-bug", true).Return("", nil)
	store.On("Delete", "fix-bug").Return(nil)

	args := map[string]any{"command": "discard", "session_id": "fix-bug", "confirmed": true}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	assert.NoError(t, err)
	require.NotNil(t, res)
	text := resultText(t, res)
	assert.Contains(t, text, `"status":"success"`)

	// activeSession should now be nil — discard must not leave a dangling
	// pointer to the removed worktree.
	v := active.Load()
	if s, ok := v.(*domain.Session); ok && s != nil {
		t.Errorf("activeSession should be cleared after discard, got %v", s)
	}
	mockGit.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestHandleDiscard_MissingConfirmedReturnsError(t *testing.T) {
	h, _, _ := newHandlerWithStore(t)
	args := map[string]any{"command": "discard", "session_id": "fix-bug"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	assert.NoError(t, err)
	require.NotNil(t, res)
	text := resultText(t, res)
	assert.Contains(t, text, "confirmed")
}

// TestHandleStart_UnbornRepo_UsesEmptyTreeHash verifies handleStart falls back
// to emptyTreeHash as baseCommit when Head() fails on an unborn repo (zero
// commits). The session branch ref is created at the empty tree, and the
// worktree is checked out. See spec delta "Start session on fresh repo".
func TestHandleStart_UnbornRepo_UsesEmptyTreeHash(t *testing.T) {
	mockGit := new(MockGit)
	h := NewHandler(mockGit)
	h.metaDir = t.TempDir()

	id := computeSessionID("first commit")
	// Head() fails on unborn repo.
	mockGit.On("Head").Return("", assertError("rev-parse HEAD: bad revision"))
	// baseCommit must be the empty-tree hash, not a real commit.
	emptyTree := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	mockGit.On("CreateRef", "refs/heads/"+id, emptyTree).Return(nil)
	mockGit.On("AddWorktree", "../git-courer-worktrees/"+id, ""+id).
		Return("../git-courer-worktrees/"+id, nil)

	args := map[string]any{"command": "start", "agent": "claude", "goal": "first commit"}
	res, err := h.HandleSession(context.Background(), sessionReq(args))
	require.NoError(t, err)
	require.NotNil(t, res)

	text := resultText(t, res)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, emptyTree, parsed["base_commit"], "base_commit must be emptyTreeHash on unborn repo")
	mockGit.AssertExpectations(t)
}
