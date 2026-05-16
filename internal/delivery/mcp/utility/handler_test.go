package utility

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockGitForUtility is a testify-based mock implementing ports.Git for utility tests.
type mockGitForUtility struct {
	mock.Mock
}

func (m *mockGitForUtility) ListBackups() ([]domain.Backup, error) {
	args := m.Called()
	return args.Get(0).([]domain.Backup), args.Error(1)
}
func (m *mockGitForUtility) RestoreBackup(backup domain.Backup) error {
	args := m.Called(backup)
	return args.Error(0)
}
func (m *mockGitForUtility) ListBranches(pattern ...string) (string, error) {
	args := m.Called(pattern)
	return args.String(0), args.Error(1)
}

// --- Remaining ports.Git methods (stubs, not used in utility tests) ---

func (m *mockGitForUtility) Add(paths []string) error                       { panic("not implemented") }
func (m *mockGitForUtility) Amend(msg string, paths []string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Blame(filepath string) ([]domain.BlameLine, error) { panic("not implemented") }
func (m *mockGitForUtility) Branch(name string) (string, error)             { panic("not implemented") }
func (m *mockGitForUtility) CatFile(revision, path string) (string, error)  { panic("not implemented") }
func (m *mockGitForUtility) Checkout(ref string) error                       { panic("not implemented") }
func (m *mockGitForUtility) CherryPick(commit string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) Clean() error                                    { panic("not implemented") }
func (m *mockGitForUtility) Clone(repo, dest string) error                   { panic("not implemented") }
func (m *mockGitForUtility) Commit(message string) (string, error)          { panic("not implemented") }
func (m *mockGitForUtility) CommitsFromTag(sinceTag string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Config(args ...string) (string, error)          { panic("not implemented") }
func (m *mockGitForUtility) CreateBackup(op string, mode domain.StashMode) (domain.Backup, error) { panic("not implemented") }
func (m *mockGitForUtility) CreateRelease(tagName, changelog string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) CurrentBranch() (string, error)                  { panic("not implemented") }
func (m *mockGitForUtility) DeleteBackup(backup domain.Backup) error        { panic("not implemented") }
func (m *mockGitForUtility) DeleteBranch(name string, force bool) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) DeleteRemoteBranch(name string) error            { panic("not implemented") }
func (m *mockGitForUtility) DeleteTag(name string) (string, error)          { panic("not implemented") }
func (m *mockGitForUtility) DeleteTagRemote(name string) (string, error)    { panic("not implemented") }
func (m *mockGitForUtility) Diff(paths ...string) (string, error)           { panic("not implemented") }
func (m *mockGitForUtility) DiffAll(paths ...string) (string, error)        { panic("not implemented") }
func (m *mockGitForUtility) DiffRange(base, target, mode string, paths ...string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) DiffStat(paths ...string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) DiffStatStaged(paths ...string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) DiffStaged(paths ...string) (string, error)     { panic("not implemented") }
func (m *mockGitForUtility) Fetch() (string, error)                          { panic("not implemented") }
func (m *mockGitForUtility) IsGHAuthenticated() (bool, error)               { panic("not implemented") }
func (m *mockGitForUtility) IsRepo() bool                                    { panic("not implemented") }
func (m *mockGitForUtility) LatestTag() (string, error)                      { panic("not implemented") }
func (m *mockGitForUtility) ListTags(pattern ...string) ([]string, error)   { panic("not implemented") }
func (m *mockGitForUtility) ListTree(revision, path string, recursive bool) ([]string, error) { panic("not implemented") }
func (m *mockGitForUtility) ListUntracked() ([]string, error)                { panic("not implemented") }
func (m *mockGitForUtility) Log(limit int, pattern string, paths ...string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) LogFile() string                                 { panic("not implemented") }
func (m *mockGitForUtility) LogFull(limit int) (string, error)               { panic("not implemented") }
func (m *mockGitForUtility) Merge(branch string) (string, error)            { panic("not implemented") }
func (m *mockGitForUtility) MergeAbort() (string, error)                     { panic("not implemented") }
func (m *mockGitForUtility) MergeContinue() (string, error)                    { panic("not implemented") }
func (m *mockGitForUtility) MergeBase(a, b string) (string, error)          { panic("not implemented") }
func (m *mockGitForUtility) PruneBackups(olderThan time.Duration) error     { panic("not implemented") }
func (m *mockGitForUtility) Pull() (string, error)                           { panic("not implemented") }
func (m *mockGitForUtility) PullFrom(remoteBranch string) (string, error)   { panic("not implemented") }
func (m *mockGitForUtility) Push() (string, error)                           { panic("not implemented") }
func (m *mockGitForUtility) PushTag(name string) (string, error)            { panic("not implemented") }
func (m *mockGitForUtility) PushTo(remoteBranch string) (string, error)     { panic("not implemented") }
func (m *mockGitForUtility) Rebase(branch string) (string, error)           { panic("not implemented") }
func (m *mockGitForUtility) RebaseAbort() (string, error)                    { panic("not implemented") }
func (m *mockGitForUtility) RebaseContinue() (string, error)                 { panic("not implemented") }
func (m *mockGitForUtility) Reflog() ([]domain.ReflogEntry, error)          { panic("not implemented") }
func (m *mockGitForUtility) RemoteAdd(name, url string) (string, error)     { panic("not implemented") }
func (m *mockGitForUtility) RemoteInfo() (string, error)                     { panic("not implemented") }
func (m *mockGitForUtility) RemoteRemove(name string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) RemoteURL() (string, error)                      { panic("not implemented") }
func (m *mockGitForUtility) Remove(paths []string) error                     { panic("not implemented") }
func (m *mockGitForUtility) RenameBranch(oldName, newName string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Reset(mode, commit string) (string, error)      { panic("not implemented") }
func (m *mockGitForUtility) ResetSoft(ref string) error                      { panic("not implemented") }
func (m *mockGitForUtility) Restore(paths []string) error                    { panic("not implemented") }
func (m *mockGitForUtility) Revert(commit string) (string, error)           { panic("not implemented") }
func (m *mockGitForUtility) RevParse(ref string) (string, error)            { panic("not implemented") }
func (m *mockGitForUtility) Search(pattern string, context, before, after int, paths ...string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) SetOrigin(url string) error                       { panic("not implemented") }
func (m *mockGitForUtility) SetRemote(name, url string) error                { panic("not implemented") }
func (m *mockGitForUtility) SetUpstream(branch, remote string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Show(hash string) (domain.ShowResult, error)    { panic("not implemented") }
func (m *mockGitForUtility) ShowCommit(commit string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) Stash(message ...string) (string, error)        { panic("not implemented") }
func (m *mockGitForUtility) StashApply(index string) (string, error)        { panic("not implemented") }
func (m *mockGitForUtility) StashClear() (string, error)                     { panic("not implemented") }
func (m *mockGitForUtility) StashDiff(index string) (string, error)         { panic("not implemented") }
func (m *mockGitForUtility) StashDrop(index string) (string, error)         { panic("not implemented") }
func (m *mockGitForUtility) StashList() ([]domain.StashEntry, error)        { panic("not implemented") }
func (m *mockGitForUtility) StashPop() (string, error)                       { panic("not implemented") }
func (m *mockGitForUtility) StashShow() (string, error)                      { panic("not implemented") }
func (m *mockGitForUtility) StashWithUntracked(message string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Status() (domain.Status, error)                  { panic("not implemented") }
func (m *mockGitForUtility) Switch(branch string) error                      { panic("not implemented") }
func (m *mockGitForUtility) Tag(name, message string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) TagExists(name string) (bool, error)            { panic("not implemented") }
func (m *mockGitForUtility) UnsetUpstream(branch string) (string, error)    { panic("not implemented") }
func (m *mockGitForUtility) Version() (string, error)                        { panic("not implemented") }
func (m *mockGitForUtility) WorkDir() string                                 { panic("not implemented") }
func (m *mockGitForUtility) WithWorkDir(dir string) interface{ Git() interface{}; Err() error } { panic("not implemented") }

// --- Config tests ---

func TestHandler_HandleConfig_ReturnsAll(t *testing.T) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Model:    "llama3",
		},
	}
	h := NewHandler(nil, nil, cfg, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed, "config_path")
	assert.Contains(t, parsed, "content")
	assert.Contains(t, parsed, "provider")
	assert.Contains(t, parsed, "models")
}

// --- Backup tests ---

func TestHandler_HandleBackup_RESTORE(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{Ref: "backup-ref", Operation: "commit"}
	git.On("RestoreBackup", backup).Return(nil)

	h := NewHandler(git, &backup, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Successfully restored")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_RESTORE_NoBackup(t *testing.T) {
	h := NewHandler(nil, nil, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "no operation to restore")
}

// --- Config SET_TEST_COMMAND tests (project-local config) ---

func TestHandler_HandleConfig_SetTestCommand_WritesToProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	h := NewHandler(nil, nil, nil, tmpDir)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_TEST_COMMAND", "test_command": "make test-ci"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "SET_TEST_COMMAND")
	assert.Contains(t, text, "make test-ci")

	// Verify project-local config was written
	loaded, err := config.LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "make test-ci", loaded.TestCommand)
	assert.NotNil(t, loaded.Areas)
}

func TestHandler_HandleConfig_SetTestCommand_PreservesExistingFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create .git-courer/config.json with description and areas
	existing := &config.ProjectConfig{
		Description: "existing project",
		Areas: map[string][]string{
			"core": {"internal/"},
			"docs": {"docs/"},
		},
		TestCommand: "",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, existing))

	h := NewHandler(nil, nil, nil, tmpDir)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_TEST_COMMAND", "test_command": "pytest"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "pytest")

	// Verify all fields preserved
	loaded, err := config.LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "existing project", loaded.Description)
	assert.Equal(t, "pytest", loaded.TestCommand)
	assert.Equal(t, []string{"internal/"}, loaded.Areas["core"])
	assert.Equal(t, []string{"docs/"}, loaded.Areas["docs"])
}

func TestHandler_HandleConfig_SetTestCommand_EmptyString(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create config with existing test_command
	existing := &config.ProjectConfig{
		Description: "test project",
		Areas:       map[string][]string{},
		TestCommand: "old-command",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, existing))

	h := NewHandler(nil, nil, nil, tmpDir)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_TEST_COMMAND", "test_command": ""},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "SET_TEST_COMMAND")

	// Verify test_command is cleared
	loaded, err := config.LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "", loaded.TestCommand)
}

func TestHandler_HandleBackup_RESTORE_ClearsBackup(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{Ref: "backup-ref", Operation: "commit"}
	git.On("RestoreBackup", backup).Return(nil)

	h := NewHandler(git, &backup, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	_, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, domain.Backup{}, backup, "lastBackup should be cleared after RESTORE")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_LIST(t *testing.T) {
	git := new(mockGitForUtility)
	git.On("ListBackups").Return([]domain.Backup{
		{Ref: "ref1", Operation: "commit", CreatedAt: time.Now()},
	}, nil)

	h := NewHandler(git, nil, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "LIST"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "backups")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_UnknownCommand(t *testing.T) {
	h := NewHandler(nil, nil, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "UND"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown command")
}

// --- Release tests ---

func TestHandler_HandleRelease_START(t *testing.T) {
	h := NewHandler(nil, nil, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "START"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Release plan initiated")
}

func TestHandler_HandleRelease_APPLY(t *testing.T) {
	h := NewHandler(nil, nil, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "APPLY"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Release applied")
}

func TestHandler_HandleRelease_ABORT(t *testing.T) {
	h := NewHandler(nil, nil, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "ABORT"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Release aborted")
}

func TestHandler_HandleRelease_UnknownCommand(t *testing.T) {
	h := NewHandler(nil, nil, nil, "")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "PUBLISH"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown command")
}
