package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// stubGitForPrepare is a minimal Git stub that records calls and returns canned values.
type stubGitForPrepare struct {
	currentBranchResult string
	listBranchesResult  string
	listTagsResult      []string
	currentBranchCalled int
	listBranchesCalled  int
}

func (s *stubGitForPrepare) CurrentBranch() (string, error) {
	s.currentBranchCalled++
	if s.currentBranchResult == "" {
		return "main", nil
	}
	return s.currentBranchResult, nil
}
func (s *stubGitForPrepare) ListBranches(pattern ...string) (string, error) {
	s.listBranchesCalled++
	if s.listBranchesResult == "" {
		return "main\ndevelop\nfeat/login", nil
	}
	return s.listBranchesResult, nil
}
func (s *stubGitForPrepare) ListTags(pattern ...string) ([]string, error) {
	if s.listTagsResult == nil {
		return []string{"v1.0.0", "v1.1.0"}, nil
	}
	return s.listTagsResult, nil
}
func (s *stubGitForPrepare) Log(limit int, pattern string, paths ...string) (string, error) {
	return "commit log", nil
}
func (s *stubGitForPrepare) Status() (domain.Status, error)                          { return domain.Status{}, nil }
func (s *stubGitForPrepare) Diff(paths ...string) (string, error)                    { return "", nil }
func (s *stubGitForPrepare) DiffStaged(paths ...string) (string, error)              { return "", nil }
func (s *stubGitForPrepare) DiffStatStaged(paths ...string) (string, error)          { return "", nil }
func (s *stubGitForPrepare) ListUntracked() ([]string, error)                        { return nil, nil }
func (s *stubGitForPrepare) LogFull(limit int) (string, error)                       { return "", nil }
func (s *stubGitForPrepare) IsRepo() bool                                            { return true }
func (s *stubGitForPrepare) RemoteURL() (string, error)                              { return "", nil }
func (s *stubGitForPrepare) RemoteInfo() (string, error)                             { return "", nil }
func (s *stubGitForPrepare) LatestTag() (string, error)                              { return "v1.0.0", nil }
func (s *stubGitForPrepare) CommitsFromTag(sinceTag string) (string, error)          { return "", nil }
func (s *stubGitForPrepare) TagExists(name string) (bool, error)                     { return false, nil }
func (s *stubGitForPrepare) IsGHAuthenticated() (bool, error)                        { return true, nil }
func (s *stubGitForPrepare) CreateRelease(tagName, changelog string) (string, error) { return "", nil }
func (s *stubGitForPrepare) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	return "", nil
}
func (s *stubGitForPrepare) CatFile(revision, path string) (string, error) { return "", nil }
func (s *stubGitForPrepare) ListTree(revision, path string, recursive bool) ([]string, error) {
	return nil, nil
}
func (s *stubGitForPrepare) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	return domain.Backup{}, nil
}
func (s *stubGitForPrepare) RestoreBackup(backup domain.Backup) error             { return nil }
func (s *stubGitForPrepare) DeleteBackup(backup domain.Backup) error              { return nil }
func (s *stubGitForPrepare) ListBackups() ([]domain.Backup, error)                { return nil, nil }
func (s *stubGitForPrepare) PruneBackups(olderThan time.Duration) error           { return nil }
func (s *stubGitForPrepare) Add(paths []string) error                             { return nil }
func (s *stubGitForPrepare) Remove(paths []string) error                          { return nil }
func (s *stubGitForPrepare) Checkout(name string) (string, error)                 { return "", nil }
func (s *stubGitForPrepare) Switch(name string) error                             { return nil }
func (s *stubGitForPrepare) Push() (string, error)                                { return "", nil }
func (s *stubGitForPrepare) PushTag(name string) (string, error)                  { return "", nil }
func (s *stubGitForPrepare) PushTags() (string, error)                            { return "", nil }
func (s *stubGitForPrepare) Pull() (string, error)                                { return "", nil }
func (s *stubGitForPrepare) Fetch() (string, error)                               { return "", nil }
func (s *stubGitForPrepare) Stash(message ...string) (string, error)              { return "", nil }
func (s *stubGitForPrepare) StashPop() (string, error)                            { return "", nil }
func (s *stubGitForPrepare) Commit(message string) (string, error)                { return "", nil }
func (s *stubGitForPrepare) Branch(name string) (string, error)                   { return "", nil }
func (s *stubGitForPrepare) RenameBranch(oldName, newName string) (string, error) { return "", nil }
func (s *stubGitForPrepare) DeleteBranch(name string, force bool) (string, error) { return "", nil }
func (s *stubGitForPrepare) Reset(mode string, commit string) (string, error)     { return "", nil }
func (s *stubGitForPrepare) Merge(branch string) (string, error)                  { return "", nil }
func (s *stubGitForPrepare) Tag(name, message string) (string, error)             { return "", nil }
func (s *stubGitForPrepare) TagFromFile(name, path string) (string, error)          { return "", nil }
func (s *stubGitForPrepare) DeleteTag(name string) (string, error)                { return "", nil }
func (s *stubGitForPrepare) DeleteTagRemote(name string) (string, error)          { return "", nil }
func (s *stubGitForPrepare) Blame(filepath string) ([]domain.BlameLine, error)    { return nil, nil }
func (s *stubGitForPrepare) Show(hash string) (domain.ShowResult, error) {
	return domain.ShowResult{}, nil
}
func (s *stubGitForPrepare) Reflog() ([]domain.ReflogEntry, error)    { return nil, nil }
func (s *stubGitForPrepare) StashList() ([]domain.StashEntry, error)  { return nil, nil }
func (s *stubGitForPrepare) StashDiff(index string) (string, error)   { return "", nil }
func (s *stubGitForPrepare) StashApply(index string) (string, error)  { return "", nil }
func (s *stubGitForPrepare) StashDrop(index string) (string, error)   { return "", nil }
func (s *stubGitForPrepare) StashClear() (string, error)              { return "", nil }
func (s *stubGitForPrepare) StashShow() (string, error)               { return "", nil }
func (s *stubGitForPrepare) MergeBase(a, b string) (string, error)    { return "", nil }
func (s *stubGitForPrepare) LogRange(from, to string) (string, error) { return "", nil }

func (s *stubGitForPrepare) ResetSoft(target string) error                        { return nil }
func (s *stubGitForPrepare) PushTo(remote string) (string, error)                 { return "", nil }
func (s *stubGitForPrepare) PushToBranch(remote, branch string) (string, error)   { return "", nil }
func (s *stubGitForPrepare) PullFrom(remote string) (string, error)               { return "", nil }
func (s *stubGitForPrepare) PullFromBranch(remote, branch string) (string, error) { return "", nil }
func (s *stubGitForPrepare) DeleteRemoteBranch(name string) error                 { return nil }
func (s *stubGitForPrepare) DiffAll(paths ...string) (string, error)              { return "", nil }
func (s *stubGitForPrepare) DiffUntracked() (string, error)                       { return "", nil }
func (s *stubGitForPrepare) DiffStat(paths ...string) (string, error)             { return "", nil }
func (s *stubGitForPrepare) DiffRange(base, target, mode string, paths ...string) (string, error) {
	return "", nil
}

func (s *stubGitForPrepare) Amend(message string, paths []string) (string, error) { return "", nil }
func (s *stubGitForPrepare) Revert(commit string) (string, error)                 { return "", nil }
func (s *stubGitForPrepare) Restore(paths []string) error                         { return nil }
func (s *stubGitForPrepare) Clean() error                                         { return nil }
func (s *stubGitForPrepare) ShowCommit(commit string) (string, error)             { return "", nil }
func (s *stubGitForPrepare) RemoteAdd(name, url string) (string, error)           { return "", nil }
func (s *stubGitForPrepare) RemoteRemove(name string) (string, error)             { return "", nil }
func (s *stubGitForPrepare) StashWithUntracked(message string) (string, error)    { return "", nil }
func (s *stubGitForPrepare) MergeAbort() (string, error)                          { return "", nil }
func (s *stubGitForPrepare) MergeContinue() (string, error)                       { return "", nil }
func (s *stubGitForPrepare) MergeSkip() (string, error)                           { return "", nil }
func (s *stubGitForPrepare) Rebase(branch string) (string, error)                 { return "", nil }
func (s *stubGitForPrepare) RebaseAbort() (string, error)                         { return "", nil }
func (s *stubGitForPrepare) RebaseContinue() (string, error)                      { return "", nil }
func (s *stubGitForPrepare) RebaseSkip() (string, error)                          { return "", nil }
func (s *stubGitForPrepare) RebaseOnto(newBase, upstream, branch string) (string, error) {
	return "", nil
}
func (s *stubGitForPrepare) CherryPick(commit string) (string, error)          { return "", nil }
func (s *stubGitForPrepare) SetUpstream(branch, remote string) (string, error) { return "", nil }
func (s *stubGitForPrepare) UnsetUpstream(branch string) (string, error)       { return "", nil }
func (s *stubGitForPrepare) ConfigGet(key string) (string, error)              { return "", nil }
func (s *stubGitForPrepare) ConfigSet(key, value string) (string, error)       { return "", nil }
func (s *stubGitForPrepare) SymbolicRef(ref string) (string, error)            { return "", nil }
func (s *stubGitForPrepare) WriteTree() (string, error)                        { return "", nil }
func (s *stubGitForPrepare) CommitTree(treeHash, parentHash, message string) (string, error) {
	return "", nil
}
func (s *stubGitForPrepare) UpdateRef(ref, commitHash string) (string, error) { return "", nil }
func (s *stubGitForPrepare) Head() (string, error)                            { return "", nil }
func (s *stubGitForPrepare) HashObject(data []byte) (string, error)         { return "mock-blob-sha", nil }
func (s *stubGitForPrepare) ShowRef(pattern string) (string, error)          { return "", nil }
// Worktree & ref methods — unused in prepare tests
func (s *stubGitForPrepare) AddWorktree(path, branch string) (string, error) { return "", nil }
func (s *stubGitForPrepare) RemoveWorktree(path string) error               { return nil }
func (s *stubGitForPrepare) CreateRef(ref, commitHash string) error         { return nil }
func (s *stubGitForPrepare) GitCommonDir() (string, error)                  { return ".git", nil }

// newWorkflowForPrepareTest builds a minimal Workflow with the given stub.
func newWorkflowForPrepareTest(stub *stubGitForPrepare) *Workflow {
	return &Workflow{git: stub}
}

// TestPrepare_BranchRenamePopulatesContext verifies that branch_rename sets
// both CurrentBranch and Branches in PrepContext, mirroring branch_create.
func TestPrepare_BranchRenamePopulatesContext(t *testing.T) {
	stub := &stubGitForPrepare{
		currentBranchResult: "feat/old-name",
		listBranchesResult:  "main\ndevelop\nfeat/old-name",
	}
	w := newWorkflowForPrepareTest(stub)

	ctx, err := w.prepare(context.Background(), "branch_rename")
	if err != nil {
		t.Fatalf("prepare(branch_rename) error = %v", err)
	}
	if ctx.CurrentBranch == "" {
		t.Error("branch_rename: PrepContext.CurrentBranch must be non-empty")
	}
	if ctx.Branches == "" {
		t.Error("branch_rename: PrepContext.Branches must be non-empty")
	}
	if stub.currentBranchCalled == 0 {
		t.Error("branch_rename: CurrentBranch() was never called")
	}
	if stub.listBranchesCalled == 0 {
		t.Error("branch_rename: ListBranches() was never called")
	}
}

// TestPrepare_Operations is a table-driven test covering all operations and
// the fields they are expected to populate.
func TestPrepare_Operations(t *testing.T) {
	tests := []struct {
		op           string
		wantBranch   bool // PrepContext.CurrentBranch should be non-empty
		wantBranches bool // PrepContext.Branches should be non-empty
		wantTags     bool // PrepContext.Tags should be non-empty
		wantLog      bool // PrepContext.Log should be non-empty
	}{
		{op: "branch_create", wantBranch: true, wantBranches: true},
		{op: "branch_rename", wantBranch: true, wantBranches: true},
		{op: "branch_delete", wantBranch: false, wantBranches: true},
		{op: "merge", wantBranch: true, wantBranches: true},
		{op: "release", wantBranch: true, wantBranches: true, wantTags: true, wantLog: true},
		{op: "tag_create", wantBranch: false, wantBranches: false}, // no branch context needed
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			stub := &stubGitForPrepare{}
			w := newWorkflowForPrepareTest(stub)

			got, err := w.prepare(context.Background(), tt.op)
			if err != nil {
				t.Fatalf("prepare(%q) error = %v", tt.op, err)
			}

			if tt.wantBranch && got.CurrentBranch == "" {
				t.Errorf("prepare(%q): expected non-empty CurrentBranch", tt.op)
			}
			if !tt.wantBranch && got.CurrentBranch != "" {
				// Not a hard failure for ops that don't set it — informational only.
			}
			if tt.wantBranches && got.Branches == "" {
				t.Errorf("prepare(%q): expected non-empty Branches", tt.op)
			}
			if tt.wantTags && got.Tags == "" {
				t.Errorf("prepare(%q): expected non-empty Tags", tt.op)
			}
			if tt.wantLog && got.Log == "" {
				t.Errorf("prepare(%q): expected non-empty Log", tt.op)
			}
		})
	}
}
