package git

import (
	"os/exec"
	"strings"
	"testing"
)

// TestWorkDirResolved_NoFn verifies that when workDirFn is nil, the static
// workDir is used.
func TestWorkDirResolved_NoFn(t *testing.T) {
	adapter := &ExecAdapter{workDir: "/tmp/static-dir"}
	if got := adapter.workDirResolved(); got != "/tmp/static-dir" {
		t.Errorf("workDirResolved() = %q, want %q", got, "/tmp/static-dir")
	}
}

// TestWorkDirResolved_WithFn verifies that when workDirFn is set, its return
// value takes precedence over the static workDir.
func TestWorkDirResolved_WithFn(t *testing.T) {
	adapter := &ExecAdapter{workDir: "/tmp/static-dir"}
	adapter.SetWorkDirFn(func() string { return "/tmp/dynamic-dir" })
	if got := adapter.workDirResolved(); got != "/tmp/dynamic-dir" {
		t.Errorf("workDirResolved() = %q, want %q", got, "/tmp/dynamic-dir")
	}
}

// TestWorkDirResolved_FnOverridesEmpty verifies the callback wins even when
// workDir is empty (ensures sessions redirect even with empty base dir).
func TestWorkDirResolved_FnOverridesEmpty(t *testing.T) {
	adapter := &ExecAdapter{workDir: ""}
	adapter.SetWorkDirFn(func() string { return "/tmp/override" })
	if got := adapter.workDirResolved(); got != "/tmp/override" {
		t.Errorf("workDirResolved() = %q, want %q", got, "/tmp/override")
	}
}

// TestSetWorkDirFn_NilClears verifies that passing nil clears the callback.
func TestSetWorkDirFn_NilClears(t *testing.T) {
	adapter := &ExecAdapter{workDir: "/tmp/static-dir"}
	adapter.SetWorkDirFn(func() string { return "/tmp/dynamic" })
	adapter.SetWorkDirFn(nil)
	if got := adapter.workDirResolved(); got != "/tmp/static-dir" {
		t.Errorf("after nil clear, workDirResolved() = %q, want %q", got, "/tmp/static-dir")
	}
}

// TestRunGit_UsesWorkDirFn verifies that runGit actually executes in the
// directory returned by workDirFn. We create two repos with different HEAD
// commits and confirm the log output reflects the dynamic dir.
func TestRunGit_UsesWorkDirFn(t *testing.T) {
	repoA := t.TempDir()
	repoB := t.TempDir()
	initGitRepo(t, repoA)
	initGitRepo(t, repoB)

	// Distinct marker commits so we can tell the repos apart via git log.
	mkCommit(t, repoA, "marker-A")
	mkCommit(t, repoB, "marker-B")

	adapter := New(repoA)
	adapter.SetWorkDirFn(func() string { return repoB })

	out, err := adapter.runGit("log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("runGit failed: %v", err)
	}
	if !strings.Contains(out, "marker-B") {
		t.Errorf("runGit targeted wrong repo; expected marker-B in %q", out)
	}
}

// TestRunGit_FallsBackToWorkDir verifies runGit uses the static workDir when
// workDirFn is nil.
func TestRunGit_FallsBackToWorkDir(t *testing.T) {
	repoA := t.TempDir()
	initGitRepo(t, repoA)
	mkCommit(t, repoA, "marker-A")

	adapter := New(repoA)
	out, err := adapter.runGit("log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("runGit failed: %v", err)
	}
	if !strings.Contains(out, "marker-A") {
		t.Errorf("runGit targeted wrong repo; expected marker-A in %q", out)
	}
}

// mkCommit creates an empty commit with the given message in dir.
func mkCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", msg)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit in %s failed: %s", dir, out)
	}
}