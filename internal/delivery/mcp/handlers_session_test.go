package mcp

import (
	"sync/atomic"
	"testing"

	"github.com/blak0p/git-courer/internal/adapters/git"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/mark3labs/mcp-go/server"
)

// TestRegisterTools_WrapsExecAdapterWithSessionGit verifies that when the
// injected git adapter is a *git.ExecAdapter, registerTools replaces
// srv.git with a sessionGit wrapper and seeds srv.activeSession so future
// `session select` calls can publish the active session.
func TestRegisterTools_WrapsExecAdapterWithSessionGit(t *testing.T) {
	t.TempDir()
	execAdapter := git.New(".")
	srv := &Server{git: execAdapter}
	srv.activeSession.Store((*domain.Session)(nil))

	mcpSrv := server.NewMCPServer("test", "0.0.0")
	registerTools(mcpSrv, srv)

	// srv.git must no longer be the bare ExecAdapter; the wrapper is a
	// distinct type (ports.Git). We assert via type assertion failure.
	if _, stillIs := srv.git.(*git.ExecAdapter); stillIs {
		t.Fatalf("srv.git was not replaced by the sessionGit wrapper")
	}
	// activeSession should still hold a typed nil (not unset).
	v := srv.activeSession.Load()
	if sess, ok := v.(*domain.Session); !ok || sess != nil {
		t.Errorf("activeSession should hold a non-nil *domain.Session pointer to nil, got %T %v", v, v)
	}
}

// TestRegisterTools_NilGitDoesNotWrap verifies registerTools tolerates a nil
// git (used in some registration-only tests) without panicking.
func TestRegisterTools_NilGitDoesNotWrap(t *testing.T) {
	srv := &Server{git: nil}
	srv.activeSession.Store((*domain.Session)(nil))

	mcpSrv := server.NewMCPServer("test", "0.0.0")
	registerTools(mcpSrv, srv)

	if srv.git != nil {
		t.Errorf("nil git should remain nil, got %T", srv.git)
	}
}

// TestServer_ActiveSessionShared verifies the *atomic.Value stored on the
// Server is the one read by the workDirFn installed on the ExecAdapter, i.e.
// updates to activeSession are visible to the wrapper's redirect logic.
func TestServer_ActiveSessionShared(t *testing.T) {
	execAdapter := git.New(".")
	srv := &Server{git: execAdapter}
	srv.activeSession.Store((*domain.Session)(nil))

	mcpSrv := server.NewMCPServer("test", "0.0.0")
	registerTools(mcpSrv, srv)

	// Simulate `session select`: publish a session.
	want := &domain.Session{ID: "abc", Worktree: "/wt/abc"}
	srv.activeSession.Store(want)

	// The workDirFn installed on the ExecAdapter must observe this.
	got := execAdapter.WorkDirFn()()
	if got != "/wt/abc" {
		t.Errorf("workDirFn returned %q after select, want %q", got, "/wt/abc")
	}

	// Simulate `session finish` clearing it.
	srv.activeSession.Store((*domain.Session)(nil))
	got = execAdapter.WorkDirFn()()
	if got == "/wt/abc" {
		t.Errorf("workDirFn returned worktree after clear, want base workDir")
	}
}

// no-op reference so atomic is imported in case future helpers need it.
var _ = atomic.Value{}