package git

import (
	"sync/atomic"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// TestSessionGit_CompileTimeAssert verifies the wrapper satisfies ports.Git.
// The package-level var _ ports.Git = (*sessionGit)(nil) provides this at
// build time; this test makes the guarantee explicit at test time.
func TestSessionGit_CompileTimeAssert(t *testing.T) {
	var _ ports.Git = (*sessionGit)(nil)
}

// TestSessionGit_NormalMethodsDelegateToBase verifies that ordinary git
// operations route to base regardless of the active session state.
func TestSessionGit_NormalMethodsDelegateToBase(t *testing.T) {
	base := &fakeGit{}
	mainGit := &fakeGit{}
	wrapper := newSessionGit(base, mainGit, newActiveSession())

	// Read method.
	if _, err := wrapper.Status(); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !contains(base.calls, "base:Status") {
		t.Errorf("expected Status to delegate to base, calls=%v", base.calls)
	}
	if len(mainGit.calls) != 0 {
		t.Errorf("mainGit should not be touched, calls=%v", mainGit.calls)
	}

	// Write method.
	if _, err := wrapper.Commit("msg"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !contains(base.calls, "Commit") {
		t.Errorf("expected Commit to delegate to base, calls=%v", base.calls)
	}
}

// TestSessionGit_WorktreeOpsDelegateToMainGit verifies AddWorktree,
// RemoveWorktree, and CreateRef ALWAYS target mainGit — even with an active
// session. Otherwise session lifecycle would corrupt.
func TestSessionGit_WorktreeOpsDelegateToMainGit(t *testing.T) {
	base := &fakeGit{}
	mainGit := &fakeGit{}
	active := newActiveSession()
	active.Store(&domain.Session{ID: "s1", Worktree: "/wt/s1"})
	wrapper := newSessionGit(base, mainGit, active)

	if _, err := wrapper.AddWorktree("../wt/new", "newbr"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if err := wrapper.RemoveWorktree("/wt/s1"); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if err := wrapper.CreateRef("refs/heads/x", "deadbeef"); err != nil {
		t.Fatalf("CreateRef: %v", err)
	}

	if !contains(mainGit.calls, "AddWorktree") || !contains(mainGit.calls, "RemoveWorktree") || !contains(mainGit.calls, "CreateRef") {
		t.Errorf("expected worktree ops on mainGit, calls=%v", mainGit.calls)
	}
	if len(base.calls) != 0 {
		t.Errorf("base must NOT receive worktree ops, calls=%v", base.calls)
	}
}

// TestSessionGit_WorktreeOpsDelegateToMainGit_NoSession verifies the
// worktree exclusion holds even when no session is active.
func TestSessionGit_WorktreeOpsDelegateToMainGit_NoSession(t *testing.T) {
	base := &fakeGit{}
	mainGit := &fakeGit{}
	wrapper := newSessionGit(base, mainGit, newActiveSession())

	if _, err := wrapper.AddWorktree("../wt/x", "br"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if !contains(mainGit.calls, "AddWorktree") {
		t.Errorf("expected AddWorktree on mainGit even with no session, calls=%v", mainGit.calls)
	}
	if len(base.calls) != 0 {
		t.Errorf("base must not receive worktree ops, calls=%v", base.calls)
	}
}

// TestSessionGit_NilActiveSession_UsesBase verifies that with no active
// session the wrapper still delegates reads/writes to base (no panic).
func TestSessionGit_NilActiveSession_UsesBase(t *testing.T) {
	base := &fakeGit{}
	mainGit := &fakeGit{}
	active := newActiveSession() // nothing stored → Load returns nil
	wrapper := newSessionGit(base, mainGit, active)

	if _, err := wrapper.Head(); err != nil {
		t.Fatalf("Head with nil active session: %v", err)
	}
	if !contains(base.calls, "Head") {
		t.Errorf("expected Head on base with nil session, calls=%v", base.calls)
	}
}

// newActiveSession returns a fresh *atomic.Value ready to hold *domain.Session
// or nil.
func newActiveSession() *atomic.Value {
	v := &atomic.Value{}
	v.Store((*domain.Session)(nil)) // typed nil so subsequent Store of a real ptr is consistent
	return v
}

// contains is a tiny helper since fakeGit.calls is []string.
func contains(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}
