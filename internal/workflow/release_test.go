package workflow

import (
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// mockGitForRelease implements ports.Git interface for testing.
type mockGitForRelease struct {
	latestTagResult         string
	latestTagErr            error
	commitsResult           string
	commitsErr              error
	commitsFromTagErr       error
	logFullResult           string
	logFullErr              error
	listTagsResult          []string
	listTagsErr             error
	tagExistsResult         bool
	isGHAuthenticatedResult bool
	isGHAuthenticatedErr    error
	isGHAuthenticatedCalled bool
	createReleaseTag        string
	createReleaseChangelog  string
	createReleaseResult     func(tag, changelog string) (string, error)
	listBranchesResult      string
	tagCreated              bool
	tagCalled               bool
	tagCalledName           string
	tagCalledMessage        string
	pushTagErr              error
}

func (m *mockGitForRelease) Status() (domain.Status, error)                 { return domain.Status{}, nil }
func (m *mockGitForRelease) Diff(paths ...string) (string, error)           { return "", nil }
func (m *mockGitForRelease) DiffStat(paths ...string) (string, error)       { return "", nil }
func (m *mockGitForRelease) DiffStatStaged(paths ...string) (string, error) { return "", nil }
func (m *mockGitForRelease) DiffAll(paths ...string) (string, error)         { return "", nil }
func (m *mockGitForRelease) DiffRange(base, target, mode string, paths ...string) (string, error) { return "", nil }
func (m *mockGitForRelease) DiffStaged(paths ...string) (string, error)     { return "", nil }
func (m *mockGitForRelease) ListUntracked() ([]string, error)               { return nil, nil }
func (m *mockGitForRelease) Log(limit int, pattern string, paths ...string) (string, error) { return "", nil }
func (m *mockGitForRelease) LogFull(limit int) (string, error) {
	if m.logFullErr != nil {
		return "", m.logFullErr
	}
	return m.logFullResult, nil
}
func (m *mockGitForRelease) CurrentBranch() (string, error) { return "develop", nil }
func (m *mockGitForRelease) ListBranches(pattern ...string) (string, error) {
	return m.listBranchesResult, nil
}
func (m *mockGitForRelease) ListTags(pattern ...string) ([]string, error) {
	return m.listTagsResult, m.listTagsErr
}
func (m *mockGitForRelease) IsRepo() bool               { return true }
func (m *mockGitForRelease) RemoteURL() (string, error) { return "", nil }
func (m *mockGitForRelease) RemoteInfo() (string, error) { return "", nil }
func (m *mockGitForRelease) LatestTag() (string, error) {
	if m.latestTagResult != "" || m.latestTagErr != nil {
		return m.latestTagResult, m.latestTagErr
	}
	return "v1.0.0", nil
}
func (m *mockGitForRelease) CommitsFromTag(sinceTag string) (string, error) {
	if m.commitsFromTagErr != nil {
		return "", m.commitsFromTagErr
	}
	if m.commitsResult != "" || m.commitsErr != nil {
		return m.commitsResult, m.commitsErr
	}
	return "feat: default mock commit", nil
}
func (m *mockGitForRelease) TagExists(name string) (bool, error)         { return m.tagExistsResult, nil }
func (m *mockGitForRelease) DeleteTag(name string) (string, error)       { return "", nil }
func (m *mockGitForRelease) DeleteTagRemote(name string) (string, error) { return "", nil }
func (m *mockGitForRelease) PushTag(name string) (string, error)         { return "", m.pushTagErr }
func (m *mockGitForRelease) PushTags() (string, error)                   { return "", nil }
func (m *mockGitForRelease) IsGHAuthenticated() (bool, error) {
	m.isGHAuthenticatedCalled = true
	return m.isGHAuthenticatedResult, m.isGHAuthenticatedErr
}
func (m *mockGitForRelease) CreateRelease(tagName, changelog string) (string, error) {
	m.createReleaseTag = tagName
	m.createReleaseChangelog = changelog
	if m.createReleaseResult != nil {
		return m.createReleaseResult(tagName, changelog)
	}
	return "", nil
}
func (m *mockGitForRelease) Search(pattern string, context, before, after int, paths ...string) (string, error) { return "", nil }
func (m *mockGitForRelease) CatFile(revision, path string) (string, error) { return "", nil }
func (m *mockGitForRelease) ListTree(revision, path string, recursive bool) ([]string, error) { return nil, nil }
func (m *mockGitForRelease) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	return domain.Backup{}, nil
}
func (m *mockGitForRelease) RestoreBackup(backup domain.Backup) error             { return nil }
func (m *mockGitForRelease) DeleteBackup(backup domain.Backup) error              { return nil }
func (m *mockGitForRelease) ListBackups() ([]domain.Backup, error)                { return nil, nil }
func (m *mockGitForRelease) PruneBackups(olderThan time.Duration) error          { return nil }
func (m *mockGitForRelease) Add(paths []string) error                             { return nil }
func (m *mockGitForRelease) Remove(paths []string) error                          { return nil }
func (m *mockGitForRelease) Checkout(name string) (string, error)                 { return "", nil }
func (m *mockGitForRelease) Switch(name string) error                             { return nil }
func (m *mockGitForRelease) Push() (string, error)                                { return "", nil }
func (m *mockGitForRelease) PushTo(remoteBranch string) (string, error)           { return "", nil }
func (m *mockGitForRelease) Pull() (string, error)                                { return "", nil }
func (m *mockGitForRelease) PullFrom(remoteBranch string) (string, error)         { return "", nil }
func (m *mockGitForRelease) Fetch() (string, error)                               { return "", nil }
func (m *mockGitForRelease) Stash(message ...string) (string, error)                            { return "", nil }
func (m *mockGitForRelease) StashPop() (string, error)                            { return "", nil }
func (m *mockGitForRelease) Commit(message string) (string, error)                { return "", nil }
func (m *mockGitForRelease) Branch(name string) (string, error)                   { m.tagCreated = true; return "", nil }
func (m *mockGitForRelease) RenameBranch(oldName, newName string) (string, error) { return "", nil }
func (m *mockGitForRelease) DeleteBranch(name string, force bool) (string, error) { return "", nil }
func (m *mockGitForRelease) DeleteRemoteBranch(name string) error                 { return nil }
func (m *mockGitForRelease) Tag(name, message string) (string, error) {
	m.tagCreated = true
	m.tagCalled = true
	m.tagCalledName = name
	m.tagCalledMessage = message
	return "", nil
}
func (m *mockGitForRelease) Merge(branch string) (string, error)                  { return "", nil }
func (m *mockGitForRelease) Reset(mode, commit string) (string, error)            { return "", nil }
func (m *mockGitForRelease) ResetSoft(ref string) error                           { return nil }
func (m *mockGitForRelease) Amend(message string, paths []string) (string, error) { return "", nil }
func (m *mockGitForRelease) Restore(paths []string) error                         { return nil }
func (m *mockGitForRelease) Clean() error                                         { return nil }
func (m *mockGitForRelease) ShowCommit(commit string) (string, error)             { return "", nil }
func (m *mockGitForRelease) RemoteAdd(name, url string) (string, error)           { return "", nil }
func (m *mockGitForRelease) RemoteRemove(name string) (string, error)             { return "", nil }
func (m *mockGitForRelease) StashWithUntracked(message string) (string, error)    { return "", nil }
func (m *mockGitForRelease) MergeAbort() (string, error)                          { return "", nil }
func (m *mockGitForRelease) MergeContinue() (string, error)                       { return "", nil }
func (m *mockGitForRelease) Rebase(branch string) (string, error)                 { return "", nil }
func (m *mockGitForRelease) RebaseAbort() (string, error)                         { return "", nil }
func (m *mockGitForRelease) RebaseContinue() (string, error)                      { return "", nil }
func (m *mockGitForRelease) CherryPick(commit string) (string, error)             { return "", nil }
func (m *mockGitForRelease) SetUpstream(branch, remote string) (string, error)    { return "", nil }
func (m *mockGitForRelease) UnsetUpstream(branch string) (string, error)          { return "", nil }
func (m *mockGitForRelease) Revert(commit string) (string, error)                 { return "", nil }
func (m *mockGitForRelease) Blame(filepath string) ([]domain.BlameLine, error)    { return nil, nil }
func (m *mockGitForRelease) Show(hash string) (domain.ShowResult, error)          { return domain.ShowResult{}, nil }
func (m *mockGitForRelease) Reflog() ([]domain.ReflogEntry, error)                { return nil, nil }
func (m *mockGitForRelease) StashList() ([]domain.StashEntry, error)              { return nil, nil }
func (m *mockGitForRelease) StashDiff(index string) (string, error)               { return "", nil }
func (m *mockGitForRelease) StashApply(index string) (string, error)              { return "", nil }
func (m *mockGitForRelease) StashDrop(index string) (string, error)               { return "", nil }
func (m *mockGitForRelease) StashClear() (string, error)                          { return "", nil }
func (m *mockGitForRelease) StashShow() (string, error)                            { return "", nil }
func (m *mockGitForRelease) MergeBase(a, b string) (string, error)                { return "", nil }

type mockLLMForRelease struct {
	changelogResult string
	changelogErr    error
	contextSet      string
	intentResult    map[string]string
}

func (m *mockLLMForRelease) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) { return "", nil }
func (m *mockLLMForRelease) DecideCommit(instruction, status, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return domain.CommitIntent{}, nil
}
func (m *mockLLMForRelease) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return m.intentResult, nil
}
func (m *mockLLMForRelease) SetRetryContext(msg string) {}
func (m *mockLLMForRelease) ClearRetryContext()         {}
func (m *mockLLMForRelease) IsAvailable() bool          { return true }
func (m *mockLLMForRelease) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockLLMForRelease) AuditBinaryContent(filename, content string) (bool, error) {
	return false, nil
}
func (m *mockLLMForRelease) GenerateChangelog(commits, prev, out string) (*domain.Changelog, error) {
	if m.changelogErr != nil {
		return nil, m.changelogErr
	}
	return &domain.Changelog{Features: []string{m.changelogResult}}, nil
}
func (m *mockLLMForRelease) GenerateChangelogByArea(formattedGroups string) (domain.ChangelogByArea, error) {
	if m.changelogErr != nil {
		return nil, m.changelogErr
	}
	return domain.ChangelogByArea{"general": []string{m.changelogResult}}, nil
}
func (m *mockLLMForRelease) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}
func (m *mockLLMForRelease) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) {
	return nil, nil
}
func (m *mockLLMForRelease) SetContext(ctx string) {
	m.contextSet = ctx
}

func (m *mockLLMForRelease) ClassifyBinary(prompt string) (string, error) {
	return "fix", nil
}

type mockLogChunker struct {
	chunks       []string
	chunksResult []string
	err          error
}

func (m *mockLogChunker) Chunk(commits string, maxPerChunk int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.chunksResult) > 0 {
		return m.chunksResult, nil
	}
	if len(m.chunks) > 0 {
		return m.chunks, nil
	}
	return []string{commits}, nil
}
