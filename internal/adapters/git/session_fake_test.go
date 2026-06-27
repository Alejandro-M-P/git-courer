package git

import (
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// fakeGit is a minimal ports.Git that records which adapter received each
// call. We only instrument the methods the wrapper tests exercise; the rest
// are no-op stubs so the interface is satisfied.
type fakeGit struct {
	calls       []string
	worktreeErr error // for AddWorktree/RemoveWorktree/CreateRef
}

func (f *fakeGit) mark(name string) { f.calls = append(f.calls, name) }

func (f *fakeGit) Status() (domain.Status, error) { f.mark("base:Status"); return domain.Status{}, nil }
func (f *fakeGit) AddWorktree(path, branch string) (string, error) {
	f.mark("AddWorktree")
	return "/wt/" + branch, f.worktreeErr
}
func (f *fakeGit) RemoveWorktree(path string) error {
	f.mark("RemoveWorktree")
	return f.worktreeErr
}
func (f *fakeGit) CreateRef(ref, commitHash string) error {
	f.mark("CreateRef")
	return f.worktreeErr
}

// Remaining methods are stubs that simply mark delegation through base.
func (f *fakeGit) Diff(paths ...string) (string, error)     { f.mark("Diff"); return "", nil }
func (f *fakeGit) DiffStat(paths ...string) (string, error) { f.mark("DiffStat"); return "", nil }
func (f *fakeGit) DiffStatStaged(paths ...string) (string, error) {
	f.mark("DiffStatStaged")
	return "", nil
}
func (f *fakeGit) DiffAll(paths ...string) (string, error)    { f.mark("DiffAll"); return "", nil }
func (f *fakeGit) DiffUntracked() (string, error)             { f.mark("DiffUntracked"); return "", nil }
func (f *fakeGit) DiffStaged(paths ...string) (string, error) { f.mark("DiffStaged"); return "", nil }
func (f *fakeGit) DiffRange(b, t, m string, paths ...string) (string, error) {
	f.mark("DiffRange")
	return "", nil
}
func (f *fakeGit) ListUntracked() ([]string, error) { f.mark("ListUntracked"); return nil, nil }
func (f *fakeGit) Log(l int, p string, paths ...string) (string, error) {
	f.mark("Log")
	return "", nil
}
func (f *fakeGit) LogRange(from, to string) (string, error) { f.mark("LogRange"); return "", nil }
func (f *fakeGit) LogFull(limit int) (string, error)        { f.mark("LogFull"); return "", nil }
func (f *fakeGit) CurrentBranch() (string, error)           { f.mark("CurrentBranch"); return "", nil }
func (f *fakeGit) ListBranches(pattern ...string) (string, error) {
	f.mark("ListBranches")
	return "", nil
}
func (f *fakeGit) ListTags(pattern ...string) ([]string, error) { f.mark("ListTags"); return nil, nil }
func (f *fakeGit) IsRepo() bool                                 { f.mark("IsRepo"); return true }
func (f *fakeGit) RemoteURL() (string, error)                   { f.mark("RemoteURL"); return "", nil }
func (f *fakeGit) RemoteInfo() (string, error)                  { f.mark("RemoteInfo"); return "", nil }
func (f *fakeGit) Search(p string, c, b, a int, paths ...string) (string, error) {
	f.mark("Search")
	return "", nil
}
func (f *fakeGit) CatFile(rev, p string) (string, error) { f.mark("CatFile"); return "", nil }
func (f *fakeGit) ListTree(rev, p string, r bool) ([]string, error) {
	f.mark("ListTree")
	return nil, nil
}
func (f *fakeGit) LatestTag() (string, error)              { f.mark("LatestTag"); return "", nil }
func (f *fakeGit) CommitsFromTag(s string) (string, error) { f.mark("CommitsFromTag"); return "", nil }
func (f *fakeGit) TagExists(name string) (bool, error)     { f.mark("TagExists"); return false, nil }
func (f *fakeGit) IsGHAuthenticated() (bool, error)        { f.mark("IsGHAuthenticated"); return false, nil }
func (f *fakeGit) CreateRelease(t, c string) (string, error) {
	f.mark("CreateRelease")
	return "", nil
}
func (f *fakeGit) Blame(fp string) ([]domain.BlameLine, error) {
	f.mark("Blame")
	return nil, nil
}
func (f *fakeGit) Show(h string) (domain.ShowResult, error) {
	f.mark("Show")
	return domain.ShowResult{}, nil
}
func (f *fakeGit) Reflog() ([]domain.ReflogEntry, error) { f.mark("Reflog"); return nil, nil }
func (f *fakeGit) StashList() ([]domain.StashEntry, error) {
	f.mark("StashList")
	return nil, nil
}
func (f *fakeGit) StashDiff(i string) (string, error) { f.mark("StashDiff"); return "", nil }
func (f *fakeGit) StashShow() (string, error)         { f.mark("StashShow"); return "", nil }
func (f *fakeGit) MergeBase(a, b string) (string, error) {
	f.mark("MergeBase")
	return "", nil
}
func (f *fakeGit) CreateBackup(op string, m domain.StashMode) (domain.Backup, error) {
	f.mark("CreateBackup")
	return domain.Backup{}, nil
}
func (f *fakeGit) RestoreBackup(b domain.Backup) error { f.mark("RestoreBackup"); return nil }
func (f *fakeGit) DeleteBackup(b domain.Backup) error  { f.mark("DeleteBackup"); return nil }
func (f *fakeGit) ListBackups() ([]domain.Backup, error) {
	f.mark("ListBackups")
	return nil, nil
}
func (f *fakeGit) PruneBackups(olderThan time.Duration) error {
	f.mark("PruneBackups")
	return nil
}

func (f *fakeGit) Add(paths []string) error            { f.mark("Add"); return nil }
func (f *fakeGit) Remove(paths []string) error         { f.mark("Remove"); return nil }
func (f *fakeGit) Commit(m string) (string, error)     { f.mark("Commit"); return "", nil }
func (f *fakeGit) Push() (string, error)               { f.mark("Push"); return "", nil }
func (f *fakeGit) PushTo(rb string) (string, error)    { f.mark("PushTo"); return "", nil }
func (f *fakeGit) Pull() (string, error)               { f.mark("Pull"); return "", nil }
func (f *fakeGit) PullFrom(rb string) (string, error)  { f.mark("PullFrom"); return "", nil }
func (f *fakeGit) Fetch() (string, error)              { f.mark("Fetch"); return "", nil }
func (f *fakeGit) Stash(m ...string) (string, error)   { f.mark("Stash"); return "", nil }
func (f *fakeGit) StashPop() (string, error)           { f.mark("StashPop"); return "", nil }
func (f *fakeGit) StashApply(i string) (string, error) { f.mark("StashApply"); return "", nil }
func (f *fakeGit) StashDrop(i string) (string, error)  { f.mark("StashDrop"); return "", nil }
func (f *fakeGit) StashClear() (string, error)         { f.mark("StashClear"); return "", nil }
func (f *fakeGit) Switch(b string) error               { f.mark("Switch"); return nil }
func (f *fakeGit) Branch(n string) (string, error)     { f.mark("Branch"); return "", nil }
func (f *fakeGit) DeleteBranch(n string, force bool) (string, error) {
	f.mark("DeleteBranch")
	return "", nil
}
func (f *fakeGit) RenameBranch(o, n string) (string, error) { f.mark("RenameBranch"); return "", nil }
func (f *fakeGit) DeleteRemoteBranch(n string) error        { f.mark("DeleteRemoteBranch"); return nil }
func (f *fakeGit) Tag(n, m string) (string, error)          { f.mark("Tag"); return "", nil }
func (f *fakeGit) TagFromFile(n, p string) (string, error)  { f.mark("TagFromFile"); return "", nil }
func (f *fakeGit) PushTag(n string) (string, error)         { f.mark("PushTag"); return "", nil }
func (f *fakeGit) DeleteTag(n string) (string, error)       { f.mark("DeleteTag"); return "", nil }
func (f *fakeGit) DeleteTagRemote(n string) (string, error) {
	f.mark("DeleteTagRemote")
	return "", nil
}
func (f *fakeGit) Merge(b string) (string, error)    { f.mark("Merge"); return "", nil }
func (f *fakeGit) MergeAbort() (string, error)       { f.mark("MergeAbort"); return "", nil }
func (f *fakeGit) MergeContinue() (string, error)    { f.mark("MergeContinue"); return "", nil }
func (f *fakeGit) MergeSkip() (string, error)        { f.mark("MergeSkip"); return "", nil }
func (f *fakeGit) Reset(m, c string) (string, error) { f.mark("Reset"); return "", nil }
func (f *fakeGit) ResetSoft(r string) error          { f.mark("ResetSoft"); return nil }
func (f *fakeGit) Restore(p []string) error          { f.mark("Restore"); return nil }
func (f *fakeGit) Clean() error                      { f.mark("Clean"); return nil }
func (f *fakeGit) Rebase(b string) (string, error)   { f.mark("Rebase"); return "", nil }
func (f *fakeGit) RebaseAbort() (string, error)      { f.mark("RebaseAbort"); return "", nil }
func (f *fakeGit) RebaseContinue() (string, error)   { f.mark("RebaseContinue"); return "", nil }
func (f *fakeGit) RebaseSkip() (string, error)       { f.mark("RebaseSkip"); return "", nil }
func (f *fakeGit) RebaseOnto(n, u, b string) (string, error) {
	f.mark("RebaseOnto")
	return "", nil
}
func (f *fakeGit) PushToBranch(r, b string) (string, error) { f.mark("PushToBranch"); return "", nil }
func (f *fakeGit) PullFromBranch(r, b string) (string, error) {
	f.mark("PullFromBranch")
	return "", nil
}
func (f *fakeGit) CherryPick(c string) (string, error) { f.mark("CherryPick"); return "", nil }
func (f *fakeGit) Revert(c string) (string, error)     { f.mark("Revert"); return "", nil }
func (f *fakeGit) Amend(m string, p []string) (string, error) {
	f.mark("Amend")
	return "", nil
}
func (f *fakeGit) ShowCommit(c string) (string, error) { f.mark("ShowCommit"); return "", nil }
func (f *fakeGit) RemoteAdd(n, u string) (string, error) {
	f.mark("RemoteAdd")
	return "", nil
}
func (f *fakeGit) RemoteRemove(n string) (string, error) { f.mark("RemoteRemove"); return "", nil }
func (f *fakeGit) SetUpstream(b, r string) (string, error) {
	f.mark("SetUpstream")
	return "", nil
}
func (f *fakeGit) UnsetUpstream(b string) (string, error) {
	f.mark("UnsetUpstream")
	return "", nil
}
func (f *fakeGit) StashWithUntracked(m string) (string, error) {
	f.mark("StashWithUntracked")
	return "", nil
}
func (f *fakeGit) ConfigGet(k string) (string, error) { f.mark("ConfigGet"); return "", nil }
func (f *fakeGit) ConfigSet(k, v string) (string, error) {
	f.mark("ConfigSet")
	return "", nil
}
func (f *fakeGit) SymbolicRef(r string) (string, error) { f.mark("SymbolicRef"); return "", nil }
func (f *fakeGit) WriteTree() (string, error)           { f.mark("WriteTree"); return "", nil }
func (f *fakeGit) CommitTree(t, p, m string) (string, error) {
	f.mark("CommitTree")
	return "", nil
}
func (f *fakeGit) UpdateRef(r, c string) (string, error) { f.mark("UpdateRef"); return "", nil }
func (f *fakeGit) Head() (string, error)                 { f.mark("Head"); return "deadbeef", nil }
func (f *fakeGit) HashObject(d []byte) (string, error)   { f.mark("HashObject"); return "", nil }
func (f *fakeGit) ShowRef(p string) (string, error)      { f.mark("ShowRef"); return "", nil }
func (f *fakeGit) GitCommonDir() (string, error)         { f.mark("GitCommonDir"); return "", nil }

var _ ports.Git = (*fakeGit)(nil)
