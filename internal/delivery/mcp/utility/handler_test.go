package utility

import (
	"context"
	"encoding/json"
	"fmt"
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
func (m *mockGitForUtility) CreateBackup(op string, mode domain.StashMode) (domain.Backup, error) {
	args := m.Called(op, mode)
	return args.Get(0).(domain.Backup), args.Error(1)
}
func (m *mockGitForUtility) CreateRelease(tagName, changelog string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) CurrentBranch() (string, error)                  { panic("not implemented") }
func (m *mockGitForUtility) DeleteBackup(backup domain.Backup) error {
	args := m.Called(backup)
	return args.Error(0)
}
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
func (m *mockGitForUtility) MergeSkip() (string, error)                         { panic("not implemented") }
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
func (m *mockGitForUtility) RebaseSkip() (string, error)                        { panic("not implemented") }
func (m *mockGitForUtility) RebaseOnto(newBase, upstream, branch string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) PushToBranch(remote, branch string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) PullFromBranch(remote, branch string) (string, error) { panic("not implemented") }
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
func (m *mockGitForUtility) ConfigGet(key string) (string, error)           { return "", nil }
func (m *mockGitForUtility) ConfigSet(key, value string) (string, error)    { return "", nil }
func (m *mockGitForUtility) WriteTree() (string, error)                                        { panic("not implemented") }
func (m *mockGitForUtility) CommitTree(treeHash, parentHash, message string) (string, error)   { panic("not implemented") }
func (m *mockGitForUtility) UpdateRef(ref, commitHash string) (string, error)                   { panic("not implemented") }
func (m *mockGitForUtility) Head() (string, error)                                              { panic("not implemented") }
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
	h := NewHandler(new(mockGitForUtility), cfg, "", nil, nil)

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
	backup := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_commit", Operation: "commit"}
	git.On("ListBackups").Return([]domain.Backup{backup}, nil)
	git.On("RestoreBackup", backup).Return(nil)

	h := NewHandler(git, nil, "", nil, nil)

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
	git := new(mockGitForUtility)
	git.On("ListBackups").Return([]domain.Backup{}, nil)

	h := NewHandler(git, nil, "", nil, nil)

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
	assert.Contains(t, result["error"], "no backups available to restore")
}

// --- Config SET_TEST_COMMAND tests (project-local config) ---

func TestHandler_HandleConfig_SetTestCommand_WritesToProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

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

	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

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

	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

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
	backup := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_commit", Operation: "commit"}
	git.On("ListBackups").Return([]domain.Backup{backup}, nil)
	git.On("RestoreBackup", backup).Return(nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	_, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	// RESTORE no longer sets/clears a shared pointer — it uses ListBackups
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_LIST(t *testing.T) {
	git := new(mockGitForUtility)
	git.On("ListBackups").Return([]domain.Backup{
		{Ref: "ref1", Operation: "commit", CreatedAt: time.Now()},
	}, nil)

	h := NewHandler(git, nil, "", nil, nil)

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
	h := NewHandler(new(mockGitForUtility), nil, "", nil, nil)

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

// --- Release tests (Phase 2: B4 — wired to real ReleaseService) ---

// mockReleaseService is a testify-based mock implementing ReleaseSvc.
type mockReleaseService struct {
	mock.Mock
}

func (m *mockReleaseService) Prepare(instruction, userBump string) (*domain.ReleaseIntent, string, []string, error) {
	args := m.Called(instruction, userBump)
	var warnings []string
	if args.Get(2) != nil {
		warnings = args.Get(2).([]string)
	}
	return args.Get(0).(*domain.ReleaseIntent), args.String(1), warnings, args.Error(3)
}

func (m *mockReleaseService) PrepareAndGenerateAsync(instruction, userBump string) {
	m.Called(instruction, userBump)
}

func (m *mockReleaseService) Execute(intent *domain.ReleaseIntent, changelog string) (string, error) {
	args := m.Called(intent, changelog)
	return args.String(0), args.Error(1)
}

func (m *mockReleaseService) SaveIntent(intent *domain.ReleaseIntent) {
	m.Called(intent)
}

func (m *mockReleaseService) LoadIntent() (*domain.ReleaseIntent, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ReleaseIntent), args.Error(1)
}

func (m *mockReleaseService) SaveChangelog(changelog string) {
	m.Called(changelog)
}

func (m *mockReleaseService) LoadChangelog() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mockReleaseService) ClearPending() {
	m.Called()
}

func (m *mockReleaseService) SetProgressCallback(fn func(done, total int)) {
	m.Called(fn)
}

func TestHandler_HandleRelease_START(t *testing.T) {
	svc := new(mockReleaseService)
	intent := &domain.ReleaseIntent{
		TagName:     "v1.2.0",
		VersionBump: "minor",
		IsRelease:   true,
	}
	svc.On("Prepare", "bump minor", "").Return(intent, "feat: add new feature\nfix: bug", nil, nil)
	svc.On("SaveIntent", intent).Return()
	svc.On("PrepareAndGenerateAsync", "bump minor", "").Return()
	svc.On("SetProgressCallback", mock.Anything).Return().Maybe()

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "START", "instruction": "bump minor"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var result map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "v1.2.0", result["tag_name"])
	assert.Equal(t, "pending_approval", result["status"])
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_START_DryRun(t *testing.T) {
	svc := new(mockReleaseService)
	intent := &domain.ReleaseIntent{
		TagName:     "v1.2.0",
		VersionBump: "minor",
		IsRelease:   true,
	}
	// Prepare is called even for dry_run to get the preview
	svc.On("Prepare", "", "").Return(intent, "", nil, nil)
	svc.On("SetProgressCallback", mock.Anything).Return().Maybe()

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "START", "dry_run": true},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var result map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	// Dry run returns impact preview, not pending_approval
	assert.Contains(t, result, "tag_name")
	// SaveIntent should NOT be called for dry_run
	svc.AssertNotCalled(t, "SaveIntent")
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_APPLY(t *testing.T) {
	svc := new(mockReleaseService)
	intent := &domain.ReleaseIntent{
		TagName:     "v1.2.0",
		VersionBump: "minor",
		IsRelease:   true,
	}
	svc.On("LoadIntent").Return(intent, nil)
	svc.On("LoadChangelog").Return("## Features\n- new feature", nil)
	svc.On("Execute", intent, "## Features\n- new feature").Return("Tag v1.2.0 created", nil)
	svc.On("ClearPending").Return()

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

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
	assert.Contains(t, text, "RELEASE_APPLY")
	assert.Contains(t, text, "v1.2.0")
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_APPLY_NoIntent(t *testing.T) {
	svc := new(mockReleaseService)
	svc.On("LoadIntent").Return(nil, fmt.Errorf("no release intent"))

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "APPLY"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "no release plan found")
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_ABORT(t *testing.T) {
	svc := new(mockReleaseService)
	svc.On("ClearPending").Return()

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

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
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_REGENERATE(t *testing.T) {
	svc := new(mockReleaseService)
	intent := &domain.ReleaseIntent{
		TagName:     "v1.3.0",
		VersionBump: "minor",
		IsRelease:   true,
	}
	svc.On("Prepare", "bump minor", "").Return(intent, "feat: more", nil, nil)
	svc.On("SaveIntent", intent).Return()
	svc.On("PrepareAndGenerateAsync", "bump minor", "").Return()
	svc.On("SetProgressCallback", mock.Anything).Return().Maybe()

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "REGENERATE", "instruction": "bump minor", "feedback": "add more detail"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var result map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "RELEASE_REGENERATE", result["operation"])
	assert.Contains(t, result["message"], "feedback")
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_START_PrepareError(t *testing.T) {
	svc := new(mockReleaseService)
	svc.On("Prepare", "", "").Return((*domain.ReleaseIntent)(nil), "", nil, fmt.Errorf("no tags found"))
	svc.On("SetProgressCallback", mock.Anything).Return().Maybe()

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "START"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "no tags found")
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_APPLY_ExecuteError(t *testing.T) {
	svc := new(mockReleaseService)
	intent := &domain.ReleaseIntent{
		TagName:     "v1.2.0",
		VersionBump: "minor",
		IsRelease:   true,
	}
	svc.On("LoadIntent").Return(intent, nil)
	svc.On("LoadChangelog").Return("", nil)
	svc.On("Execute", intent, "").Return("", fmt.Errorf("tag already exists"))

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "APPLY"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "tag already exists")
	svc.AssertExpectations(t)
}

func TestHandler_HandleRelease_UnknownCommand(t *testing.T) {
	svc := new(mockReleaseService)

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

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

func TestHandler_HandleRelease_APPLY_DryRun(t *testing.T) {
	svc := new(mockReleaseService)
	// APPLY with dry_run=true should return impact preview, NOT call Execute

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "APPLY", "dry_run": true},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var result map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Contains(t, result, "operation")
	// Should NOT call LoadIntent or Execute for dry_run
	svc.AssertNotCalled(t, "LoadIntent")
	svc.AssertNotCalled(t, "Execute")
}

func TestHandler_HandleRelease_MissingCommand(t *testing.T) {
	svc := new(mockReleaseService)

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "command is required")
}

func TestHandler_HandleRelease_START_WithWarnings(t *testing.T) {
	svc := new(mockReleaseService)
	intent := &domain.ReleaseIntent{
		TagName:     "v2.0.0",
		VersionBump: "major",
		IsRelease:   true,
	}
	warnings := []string{"bump type: user chose major"}
	svc.On("Prepare", "major release", "").Return(intent, "feat: big change", warnings, nil)
	svc.On("SaveIntent", intent).Return()
	svc.On("PrepareAndGenerateAsync", "major release", "").Return()
	svc.On("SetProgressCallback", mock.Anything).Return().Maybe()

	h := NewHandler(new(mockGitForUtility), nil, "", svc, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "release",
			Arguments: map[string]any{"command": "START", "instruction": "major release"},
		},
	}

	res, err := h.HandleRelease(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var result map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, true, result["success"])
	warnList, ok := result["warnings"].([]any)
	assert.True(t, ok)
	assert.Equal(t, 1, len(warnList))
	svc.AssertExpectations(t)
}

// --- Backup CREATE tests (Phase 1: B5a) ---

func TestHandler_HandleBackup_CREATE(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{
		Ref:       "refs/git-courer/backup/20260517123000_AMEND",
		Operation: "AMEND",
	}
	git.On("CreateBackup", "AMEND", domain.StashNone).Return(backup, nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "CREATE", "ref": "AMEND"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Backup created")
	assert.Contains(t, text, "AMEND")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_CREATE_DefaultOperation(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{
		Ref:       "refs/git-courer/backup/20260517123000_MANUAL",
		Operation: "MANUAL",
	}
	git.On("CreateBackup", "MANUAL", domain.StashNone).Return(backup, nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "CREATE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Backup created")
	assert.Contains(t, text, "MANUAL")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_CreateError(t *testing.T) {
	git := new(mockGitForUtility)
	git.On("CreateBackup", "MANUAL", domain.StashNone).Return(domain.Backup{}, fmt.Errorf("git error"))

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "CREATE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "git error")
	git.AssertExpectations(t)
}

// --- Backup DELETE tests (Phase 1: B5a) ---

func TestHandler_HandleBackup_DELETE(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{
		Ref:       "refs/git-courer/backup/20260517123000_MERGE",
		Operation: "MERGE",
	}
	git.On("ListBackups").Return([]domain.Backup{backup}, nil)
	git.On("DeleteBackup", backup).Return(nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "DELETE", "ref": "refs/git-courer/backup/20260517123000_MERGE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "deleted")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_DELETE_NoRef(t *testing.T) {
	h := NewHandler(new(mockGitForUtility), nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "DELETE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "ref is required")
}

func TestHandler_HandleBackup_DELETE_UnknownRef(t *testing.T) {
	git := new(mockGitForUtility)
	git.On("ListBackups").Return([]domain.Backup{}, nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "DELETE", "ref": "refs/nonexistent"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown backup ref")
	git.AssertExpectations(t)
}

// --- Backup RESTORE with ref test (Phase 1: B5b) ---

func TestHandler_HandleBackup_RESTORE_WithRef(t *testing.T) {
	git := new(mockGitForUtility)
	backup1 := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_MERGE", Operation: "MERGE"}
	backup2 := domain.Backup{Ref: "refs/git-courer/backup/20260517124000_COMMIT", Operation: "COMMIT"}
	git.On("ListBackups").Return([]domain.Backup{backup2, backup1}, nil)
	git.On("RestoreBackup", backup1).Return(nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE", "ref": "refs/git-courer/backup/20260517123000_MERGE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Successfully restored")
	assert.Contains(t, text, "MERGE")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_RESTORE_DefaultsToMostRecent(t *testing.T) {
	git := new(mockGitForUtility)
	backup1 := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_MERGE", Operation: "MERGE"}
	backup2 := domain.Backup{Ref: "refs/git-courer/backup/20260517124000_COMMIT", Operation: "COMMIT"}
	// ListBackups returns newest first
	git.On("ListBackups").Return([]domain.Backup{backup2, backup1}, nil)
	git.On("RestoreBackup", backup2).Return(nil)

	h := NewHandler(git, nil, "", nil, nil)

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
	assert.Contains(t, text, "COMMIT")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_RESTORE_UnknownRef(t *testing.T) {
	git := new(mockGitForUtility)
	backup1 := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_MERGE", Operation: "MERGE"}
	git.On("ListBackups").Return([]domain.Backup{backup1}, nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE", "ref": "nonexistent"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown backup ref")
	git.AssertExpectations(t)
}

// --- Undo alias tests (Phase 1: B9f) ---

func TestHandler_HandleUndo(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_AMEND", Operation: "AMEND"}
	git.On("ListBackups").Return([]domain.Backup{backup}, nil)
	git.On("RestoreBackup", backup).Return(nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "undo",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandleUndo(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Successfully restored")
	git.AssertExpectations(t)
}

func TestHandler_HandleUndo_NoBackups(t *testing.T) {
	git := new(mockGitForUtility)
	git.On("ListBackups").Return([]domain.Backup{}, nil)

	h := NewHandler(git, nil, "", nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "undo",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandleUndo(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "no backups available to restore")
	git.AssertExpectations(t)
}

// --- Config GET tests (Phase 3: B9d) ---

func TestHandler_HandleConfig_GET_ReturnsProject(t *testing.T) {
	tmpDir := t.TempDir()
	// Pre-create a project config
	existing := &config.ProjectConfig{
		Description: "test project",
		Areas:       map[string][]string{"core": {"internal/"}},
		TestCommand: "make test",
		UserName:    "Ada Lovelace",
		UserEmail:   "ada@example.com",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, existing))

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Model:    "llama3",
		},
	}
	h := NewHandler(new(mockGitForUtility), cfg, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "GET"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed, "config_path")
	assert.Contains(t, parsed, "project")

	proj := parsed["project"].(map[string]any)
	assert.Equal(t, "Ada Lovelace", proj["user_name"])
	assert.Equal(t, "ada@example.com", proj["user_email"])
}

func TestHandler_HandleConfig_GET_NoProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Model:    "llama3",
		},
	}
	h := NewHandler(new(mockGitForUtility), cfg, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "GET"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed, "config_path")
	assert.Nil(t, parsed["project"])
}

// --- Config SET_USER_NAME tests (Phase 3: B9d) ---

func TestHandler_HandleConfig_SetUserName(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_USER_NAME", "value": "Ada Lovelace"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "SET_USER_NAME")
	assert.Contains(t, text, "Ada Lovelace")

	loaded, err := config.LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", loaded.UserName)
}

func TestHandler_HandleConfig_SetUserName_EmptyValue(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_USER_NAME"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "value is required")
}

// --- Config SET_USER_EMAIL tests (Phase 3: B9d) ---

func TestHandler_HandleConfig_SetUserEmail(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_USER_EMAIL", "value": "ada@example.com"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "SET_USER_EMAIL")
	assert.Contains(t, text, "ada@example.com")

	loaded, err := config.LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", loaded.UserEmail)
}

func TestHandler_HandleConfig_SetUserEmail_EmptyValue(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_USER_EMAIL"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "value is required")
}

// --- Config SET_SIGNING_KEY tests (Phase 3: B9d) ---

func TestHandler_HandleConfig_SetSigningKey(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_SIGNING_KEY", "value": "ABC123DEF"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "SET_SIGNING_KEY")
	assert.Contains(t, text, "ABC123DEF")

	loaded, err := config.LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "ABC123DEF", loaded.SigningKey)
}

func TestHandler_HandleConfig_SetSigningKey_EmptyValue(t *testing.T) {
	tmpDir := t.TempDir()
	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_SIGNING_KEY"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "value is required")
}

func TestHandler_HandleConfig_SetSigningKey_PreservesExistingFields(t *testing.T) {
	tmpDir := t.TempDir()
	existing := &config.ProjectConfig{
		Description: "my project",
		Areas:       map[string][]string{"core": {"internal/"}},
		TestCommand: "make test",
		UserName:    "Ada",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, existing))

	h := NewHandler(new(mockGitForUtility), nil, tmpDir, nil, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "config",
			Arguments: map[string]any{"command": "SET_SIGNING_KEY", "value": "KEY456"},
		},
	}

	res, err := h.HandleConfig(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	loaded, err := config.LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "my project", loaded.Description)
	assert.Equal(t, "make test", loaded.TestCommand)
	assert.Equal(t, "Ada", loaded.UserName)
	assert.Equal(t, "KEY456", loaded.SigningKey)
}
