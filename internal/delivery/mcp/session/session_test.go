package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
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
	t.Run("same inputs yield same ID", func(t *testing.T) {
		a := computeSessionID("fix bug", fixedBaseCommit)
		b := computeSessionID("fix bug", fixedBaseCommit)
		assert.Equal(t, a, b, "deterministic: same inputs must produce same ID")
		assert.Len(t, a, 8, "ID must be 8 hex chars")
	})

	t.Run("different goals yield different IDs", func(t *testing.T) {
		a := computeSessionID("fix bug", fixedBaseCommit)
		b := computeSessionID("different goal", fixedBaseCommit)
		assert.NotEqual(t, a, b, "distinct goals must produce distinct IDs")
		assert.Len(t, a, 8)
		assert.Len(t, b, 8)
	})

	t.Run("different base commits yield different IDs", func(t *testing.T) {
		other := "fedcba9876543210fedcba9876543210fedcba98"
		a := computeSessionID("fix bug", fixedBaseCommit)
		b := computeSessionID("fix bug", other)
		assert.NotEqual(t, a, b, "distinct base commits must produce distinct IDs")
	})

	t.Run("ID is lowercase hex", func(t *testing.T) {
		id := computeSessionID("anything", fixedBaseCommit)
		for _, r := range id {
			assert.True(t, (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f'),
				"ID char %q is not lowercase hex", r)
		}
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
				id := computeSessionID("fix bug", fixedBaseCommit)
				m.On("Head").Return(fixedBaseCommit, nil)
				m.On("CreateRef", "refs/heads/courer/session-"+id, fixedBaseCommit).Return(nil)
				m.On("AddWorktree", "../git-courer-worktrees/"+id, "courer/session-"+id).
					Return("../git-courer-worktrees/"+id, nil)
			},
			wantInJSON: computeSessionID("fix bug", fixedBaseCommit),
		},
		{
			name:    "start collision retries with -2 suffix and succeeds",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug"},
			setup: func(m *MockGit) {
				id := computeSessionID("fix bug", fixedBaseCommit)
				m.On("Head").Return(fixedBaseCommit, nil)
				// First attempt fails (ref exists), second with -2 succeeds.
				m.On("CreateRef", "refs/heads/courer/session-"+id, fixedBaseCommit).
					Return(assertError("ref exists")).Once()
				m.On("CreateRef", "refs/heads/courer/session-"+id+"-2", fixedBaseCommit).
					Return(nil).Once()
				m.On("AddWorktree", "../git-courer-worktrees/"+id+"-2", "courer/session-"+id+"-2").
					Return("../git-courer-worktrees/"+id+"-2", nil)
			},
			wantInJSON: computeSessionID("fix bug", fixedBaseCommit) + "-2",
		},
		{
			name:    "start worktree failure rolls back branch ref",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug"},
			setup: func(m *MockGit) {
				id := computeSessionID("fix bug", fixedBaseCommit)
				m.On("Head").Return(fixedBaseCommit, nil)
				m.On("CreateRef", "refs/heads/courer/session-"+id, fixedBaseCommit).Return(nil)
				m.On("AddWorktree", "../git-courer-worktrees/"+id, "courer/session-"+id).
					Return("", assertError("worktree add failed"))
				// Rollback: UpdateRef with empty commit hash deletes the ref.
				m.On("UpdateRef", "refs/heads/courer/session-"+id, "").Return("", nil)
			},
			wantErr:    true,
			errContain: "worktree",
		},
		{
			name:    "start collision exhausted returns error",
			command: "start",
			args:    map[string]any{"agent": "claude", "goal": "fix bug"},
			setup: func(m *MockGit) {
				id := computeSessionID("fix bug", fixedBaseCommit)
				m.On("Head").Return(fixedBaseCommit, nil)
				// Attempt 0: bare id. Attempts 1..9: -2..-10. All fail.
				m.On("CreateRef", "refs/heads/courer/session-"+id, fixedBaseCommit).
					Return(assertError("ref exists")).Once()
				for i := 1; i <= 9; i++ {
					m.On("CreateRef", "refs/heads/courer/session-"+id+"-"+intToString(i+1), fixedBaseCommit).
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
			name:       "finish command returns not implemented",
			command:    "finish",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "not implemented",
		},
		{
			name:       "status command returns not implemented",
			command:    "status",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "not implemented",
		},
		{
			name:       "discard command returns not implemented",
			command:    "discard",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "not implemented",
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

	id := computeSessionID("fix auth", fixedBaseCommit)
	mockGit.On("Head").Return(fixedBaseCommit, nil)
	mockGit.On("CreateRef", "refs/heads/courer/session-"+id, fixedBaseCommit).Return(nil)
	mockGit.On("AddWorktree", "../git-courer-worktrees/"+id, "courer/session-"+id).
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
	assert.Equal(t, "courer/session-"+id, meta.Branch)
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

	id := computeSessionID("preserve check", fixedBaseCommit)
	mockGit.On("Head").Return(fixedBaseCommit, nil)
	mockGit.On("CreateRef", "refs/heads/courer/session-"+id, fixedBaseCommit).Return(nil)
	mockGit.On("AddWorktree", "../git-courer-worktrees/"+id, "courer/session-"+id).
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

		id := computeSessionID("fix auth", fixedBaseCommit)
		mockGit.On("Head").Return(fixedBaseCommit, nil)
		mockGit.On("CreateRef", "refs/heads/courer/session-"+id, fixedBaseCommit).Return(nil)
		mockGit.On("AddWorktree", "../git-courer-worktrees/"+id, "courer/session-"+id).
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
		assert.Equal(t, "courer/session-"+id, parsed["branch"])
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

	// The command parameter must be an enum with all four commands.
	props := tool.InputSchema.Properties
	require.NotNil(t, props)
	cmdProp, ok := props["command"].(map[string]any)
	require.True(t, ok, "command property must exist in schema")
	enumRaw, ok := cmdProp["enum"].([]string)
	require.True(t, ok, "command must have a string enum")
	assert.Equal(t, []string{"start", "finish", "status", "discard"}, enumRaw)

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

// intToString avoids strconv import in the test fixture setup closures.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// parseRFC3339 validates an RFC3339 timestamp without importing time twice.
func parseRFC3339(s string) (any, error) {
	return parseTime(s)
}

// Ensure the unused import guard doesn't trip when strings is used later.
var _ = strings.Contains
