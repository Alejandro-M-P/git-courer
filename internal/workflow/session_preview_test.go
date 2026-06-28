package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// fakeGit is a minimal ports.Git stub for PreviewEngine unit tests. It embeds
// ports.Git (as a nil interface) so any method NOT explicitly overridden panics
// with a nil-dereference — a loud failure proving PreviewLight stayed on the
// light path. The methods PreviewLight touches are overridden to return canned
// values; the heavy-path methods (Merge/MergeAbort/Head/Reset) set flags so we
// can assert they were never called.
type fakeGit struct {
	ports.Git // nil: unimplemented methods panic loudly

	status    domain.Status
	statusErr error
	mb        string
	mbErr     error
	diff      string
	diffErr   error

	calledMerge      bool
	calledMergeAbort bool
	calledHead       bool
	calledReset      bool
}

func (f *fakeGit) Status() (domain.Status, error) { return f.status, f.statusErr }
func (f *fakeGit) MergeBase(_, _ string) (string, error) { return f.mb, f.mbErr }
func (f *fakeGit) DiffRange(_, _, _ string, _ ...string) (string, error) {
	return f.diff, f.diffErr
}

// Mutator methods that PreviewLight MUST NOT call. Setting flags (instead of
// panicking) lets the test report WHICH heavy method was hit before failing.
func (f *fakeGit) Merge(_ string) (string, error) {
	f.calledMerge = true
	return "", errors.New("Merge must not be called by PreviewLight")
}
func (f *fakeGit) MergeAbort() (string, error) {
	f.calledMergeAbort = true
	return "", errors.New("MergeAbort must not be called by PreviewLight")
}
func (f *fakeGit) Head() (string, error) {
	f.calledHead = true
	return "", errors.New("Head must not be called by PreviewLight")
}
func (f *fakeGit) Reset(_, _ string) (string, error) {
	f.calledReset = true
	return "", errors.New("Reset must not be called by PreviewLight")
}

// TestPreviewLight_CleanStatus_NoUncommitted reports the status faithfully and
// does not invoke any merge/head/reset plumbing.
func TestPreviewLight_CleanStatus_NoUncommitted(t *testing.T) {
	g := &fakeGit{
		status: domain.Status{IsClean: true, Branch: "fix-bug", Files: []domain.FileStatus{}},
		mb:     "mergebase-sha",
		diff:   "diff --git a/x.go b/x.go\n+1\n",
	}
	eng := NewPreviewEngine(g, "")

	res, err := eng.PreviewLight(context.Background(), "main")
	if err != nil {
		t.Fatalf("PreviewLight returned error: %v", err)
	}
	if res.HasUncommitted {
		t.Errorf("HasUncommitted = true; want false for clean status")
	}
	if res.HasConflict {
		t.Errorf("HasConflict = true; PreviewLight must never report conflicts")
	}
	if res.TestResult != nil {
		t.Errorf("TestResult = %v; want nil — PreviewLight skips tests", res.TestResult)
	}
	if g.calledMerge || g.calledMergeAbort || g.calledHead || g.calledReset {
		t.Errorf("PreviewLight invoked heavy path: merge=%v abort=%v head=%v reset=%v",
			g.calledMerge, g.calledMergeAbort, g.calledHead, g.calledReset)
	}
	// DiffStats should be populated from the fake diff (one file, one addition).
	if len(res.DiffStats.Files) != 1 {
		t.Errorf("DiffStats.Files len = %d; want 1", len(res.DiffStats.Files))
	}
	if res.DiffStats.TotalAdditions != 1 {
		t.Errorf("TotalAdditions = %d; want 1", res.DiffStats.TotalAdditions)
	}
}

// TestPreviewLight_DirtyStatus_ReportsUncommitted triangulates the dirty path:
// the only abort gate is uncommitted changes, and diff stats are still computed.
func TestPreviewLight_DirtyStatus_ReportsUncommitted(t *testing.T) {
	g := &fakeGit{
		status: domain.Status{
			IsClean: false,
			Branch:  "fix-bug",
			Files:   []domain.FileStatus{{Path: "x.go", Status: "M "}},
		},
		mb:   "mergebase-sha",
		diff: "diff --git a/y.go b/y.go\n-1\n",
	}
	eng := NewPreviewEngine(g, "")

	res, err := eng.PreviewLight(context.Background(), "main")
	if err != nil {
		t.Fatalf("PreviewLight returned error: %v", err)
	}
	if !res.HasUncommitted {
		t.Errorf("HasUncommitted = false; want true for dirty status")
	}
	if res.HasConflict {
		t.Errorf("HasConflict = true; PreviewLight must never report conflicts")
	}
	if res.TestResult != nil {
		t.Errorf("TestResult = %v; want nil — PreviewLight skips tests", res.TestResult)
	}
	if g.calledMerge || g.calledMergeAbort || g.calledHead || g.calledReset {
		t.Errorf("PreviewLight invoked heavy path: merge=%v abort=%v head=%v reset=%v",
			g.calledMerge, g.calledMergeAbort, g.calledHead, g.calledReset)
	}
	// Dirty branch still gets diff stats (best-effort).
	if len(res.DiffStats.Files) != 1 {
		t.Errorf("DiffStats.Files len = %d; want 1 (diff stats still computed on dirty)", len(res.DiffStats.Files))
	}
	if res.DiffStats.TotalDeletions != 1 {
		t.Errorf("TotalDeletions = %d; want 1", res.DiffStats.TotalDeletions)
	}
}

// TestPreviewLight_StatusError_Propagates verifies that a Status() error
// short-circuits PreviewLight with a wrapped error.
func TestPreviewLight_StatusError_Propagates(t *testing.T) {
	g := &fakeGit{
		statusErr: errors.New("boom"),
	}
	eng := NewPreviewEngine(g, "")

	if _, err := eng.PreviewLight(context.Background(), "main"); err == nil {
		t.Fatalf("PreviewLight returned nil error; want wrapped status error")
	}
}