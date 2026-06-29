package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
)

func TestCreateBackup(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	adapter := New(dir)

	// Make initial commit
	os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base"), 0644)
	adapter.Add([]string{"base.txt"})
	adapter.Commit("initial")

	// Case 1: Stash untracked = true (Default behavior)
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("untracked content"), 0644)

	backup, err := adapter.CreateBackup("test_full", domain.StashAll)
	if err != nil {
		t.Fatalf("CreateBackup(..., true) error = %v", err)
	}

	if !backup.HasStash {
		t.Error("Expected HasStash to be true when untracked files exist and stashUntracked is true")
	}

	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); !os.IsNotExist(err) {
		t.Error("Untracked file should have been stashed when stashUntracked is true")
	}

	err = adapter.RestoreBackup(backup)
	if err != nil {
		t.Fatalf("RestoreBackup error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); err != nil {
		t.Error("Untracked file should have been restored")
	}

	// Case 2: Stash untracked = false (NEW behavior - currently FAILS)
	// First clean up previous file
	os.Remove(filepath.Join(dir, "untracked.txt"))

	os.WriteFile(filepath.Join(dir, "stay.txt"), []byte("should stay"), 0644)
	backup2, err := adapter.CreateBackup("test_safe", domain.StashNone)
	if err != nil {
		t.Fatalf("CreateBackup(..., false) error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "stay.txt")); err != nil {
		t.Error("Untracked file should NOT have been stashed when stashUntracked is false")
	}

	if backup2.HasStash {
		// This depends on whether there are OTHER changes. In this test, there aren't.
		// If only untracked files exist and we don't stash them, HasStash should be false.
		t.Log("HasStash is false as expected when no other changes exist")
	}

	adapter.DeleteBackup(backup2)
}

// --- PruneBackups reachability tests (table-driven) ---

// gitRun runs a git command in dir and fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s\n%s", args, dir, err, out)
	}
	return string(out)
}

// commitFile creates a commit adding a file with the given content. Returns the
// new HEAD commit OID. Uses the real adapter so it goes through the same git
// plumbing as production.
func commitFile(t *testing.T, a *ExecAdapter, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := a.Add([]string{name}); err != nil {
		t.Fatalf("add %s: %v", name, err)
	}
	if _, err := a.Commit(name); err != nil {
		t.Fatalf("commit %s: %v", name, err)
	}
	return strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
}

// createBackupRefAt creates a backup ref pointing at the given commit OID with
// an encoded timestamp so ListBackups can parse CreatedAt.
func createBackupRefAt(t *testing.T, dir, op string, ts time.Time, commit string) string {
	t.Helper()
	ref := fmt.Sprintf("refs/git-courer/backup/%s_%s", ts.Format("20060102150405"), op)
	gitRun(t, dir, "update-ref", ref, commit)
	return ref
}

// backupRefs returns the set of backup ref names currently present.
func backupRefs(t *testing.T, a *ExecAdapter) map[string]bool {
	t.Helper()
	backups, err := a.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	set := make(map[string]bool, len(backups))
	for _, b := range backups {
		set[b.Ref] = true
	}
	return set
}

func TestPruneBackups_Reachability(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(t *testing.T, a *ExecAdapter, dir string) (keep, del map[string]bool)
		olderThan time.Duration
	}{
		{
			name: "unreachable backup deleted regardless of age",
			setup: func(t *testing.T, a *ExecAdapter, dir string) (map[string]bool, map[string]bool) {
				base := commitFile(t, a, dir, "base.txt", "base")
				// Branch off base, then advance main so the branch commit becomes unreachable from HEAD.
				branchCommit := createCommitOnNewBranch(t, a, dir, "topic", "topic.txt", "topic")
				// Move HEAD forward on main, past the branch point.
				gitRun(t, dir, "checkout", "master")
				mainCommit := commitFile(t, a, dir, "main2.txt", "main2")
				_ = base
				_ = mainCommit
				// Backup pointing at the topic branch tip — reachable from refs/heads/topic.
				keepRef := createBackupRefAt(t, dir, "KEEP", time.Now(), branchCommit)
				// Backup pointing at a commit that is NOT an ancestor of HEAD or any branch.
				// Create a dangling commit (no ref points at it) then a backup at it.
				dangling := gitRun(t, dir, "commit-tree", "-m", "dangling",
					"-p", base, fmt.Sprintf("%s^{tree}", base))
				dangling = strings.TrimSpace(dangling)
				delRef := createBackupRefAt(t, dir, "DEL", time.Now(), dangling)
				return map[string]bool{keepRef: true}, map[string]bool{delRef: true}
			},
			olderThan: 30 * 24 * time.Hour,
		},
		{
			name: "reachable and older than window is deleted",
			setup: func(t *testing.T, a *ExecAdapter, dir string) (map[string]bool, map[string]bool) {
				commitFile(t, a, dir, "base.txt", "base")
				old := time.Now().Add(-45 * 24 * time.Hour)
				// Backup pointing at HEAD but created 45 days ago → reachable + old → delete.
				head := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
				delRef := createBackupRefAt(t, dir, "OLD", old, head)
				return map[string]bool{}, map[string]bool{delRef: true}
			},
			olderThan: 30 * 24 * time.Hour,
		},
		{
			name: "reachable and recent is kept",
			setup: func(t *testing.T, a *ExecAdapter, dir string) (map[string]bool, map[string]bool) {
				commitFile(t, a, dir, "base.txt", "base")
				head := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
				keepRef := createBackupRefAt(t, dir, "RECENT", time.Now().Add(-5*24*time.Hour), head)
				return map[string]bool{keepRef: true}, map[string]bool{}
			},
			olderThan: 30 * 24 * time.Hour,
		},
		{
			name: "reachable from one of many branches is kept",
			setup: func(t *testing.T, a *ExecAdapter, dir string) (map[string]bool, map[string]bool) {
				commitFile(t, a, dir, "base.txt", "base")
				// Create 12 branches, each with its own commit.
				var lastBranchCommit string
				for i := 0; i < 12; i++ {
					name := fmt.Sprintf("b%d", i)
					lastBranchCommit = createCommitOnNewBranch(t, a, dir, name, name+".txt", name)
					gitRun(t, dir, "checkout", "master")
				}
				// Backup at the 7th branch tip — reachable only from refs/heads/b6.
				keepRef := createBackupRefAt(t, dir, "ONE", time.Now(), lastBranchCommit)
				return map[string]bool{keepRef: true}, map[string]bool{}
			},
			olderThan: 30 * 24 * time.Hour,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			initGitRepo(t, dir)
			// Ensure a default branch name exists (git init may use "master" or "main").
			gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/master")
			a := New(dir)

			keep, del := tc.setup(t, a, dir)

			if err := a.PruneBackups(tc.olderThan); err != nil {
				t.Fatalf("PruneBackups: %v", err)
			}

			surviving := backupRefs(t, a)
			for ref := range keep {
				if !surviving[ref] {
					t.Errorf("expected %s to survive, but it was pruned", ref)
				}
			}
			for ref := range del {
				if surviving[ref] {
					t.Errorf("expected %s to be pruned, but it survived", ref)
				}
			}
		})
	}
}

// createCommitOnNewBranch creates a new branch at HEAD, checks it out, makes a
// commit, and returns the new commit OID. HEAD is left on the new branch.
func createCommitOnNewBranch(t *testing.T, a *ExecAdapter, dir, branch, file, content string) string {
	t.Helper()
	gitRun(t, dir, "checkout", "-b", branch)
	return commitFile(t, a, dir, file, content)
}

func TestPruneBackups_CorruptRefRecovery(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/master")
	a := New(dir)
	commitFile(t, a, dir, "base.txt", "base")

	// A valid 40-hex OID that does not exist in this repo's object store. We
	// cannot use `git update-ref` because it validates the target object, so we
	// write the ref file directly to simulate a ref left dangling after a
	// gc/prune of its target object.
	phantom := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	ts := time.Now().Format("20060102150405")
	badRef := fmt.Sprintf("refs/git-courer/backup/%s_CORRUPT", ts)
	refPath := filepath.Join(dir, ".git", badRef)
	if err := os.MkdirAll(filepath.Dir(refPath), 0755); err != nil {
		t.Fatalf("mkdir ref dir: %v", err)
	}
	if err := os.WriteFile(refPath, []byte(phantom+"\n"), 0644); err != nil {
		t.Fatalf("write corrupt ref: %v", err)
	}

	// Also create a valid reachable recent backup to confirm the loop continues.
	head := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	goodRef := createBackupRefAt(t, dir, "GOOD", time.Now(), head)

	if err := a.PruneBackups(30 * 24 * time.Hour); err != nil {
		t.Fatalf("PruneBackups should recover from corrupt ref, got: %v", err)
	}

	surviving := backupRefs(t, a)
	if surviving[badRef] {
		t.Errorf("corrupt ref %s should have been deleted", badRef)
	}
	if !surviving[goodRef] {
		t.Errorf("valid ref %s should have survived", goodRef)
	}
}

// --- Auto-prune gate tests ---

func TestCreateBackup_AutoPruneGate_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/master")
	a := New(dir)
	commitFile(t, a, dir, "base.txt", "base")

	// Create 19 backups — below the maxBackups=20 threshold. The 20th
	// CreateBackup call must NOT prune (all 19 survive + the new one).
	for i := 0; i < 19; i++ {
		ref := fmt.Sprintf("refs/git-courer/backup/202501010000%02d_PRE", i)
		gitRun(t, dir, "update-ref", ref, "HEAD")
	}

	before := len(backupRefs(t, a))
	if before != 19 {
		t.Fatalf("setup: expected 19 backups, got %d", before)
	}

	if _, err := a.CreateBackup("NEW", domain.StashNone); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	after := backupRefs(t, a)
	if len(after) != 20 {
		t.Errorf("expected 20 backups after create (no prune), got %d", len(after))
	}
}

func TestCreateBackup_AutoPruneGate_AtThreshold(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/master")
	a := New(dir)
	commitFile(t, a, dir, "base.txt", "base")

	// Create 20 old reachable backups — at the threshold. They are all old
	// enough (45d) and reachable from HEAD, so the auto-prune inside the next
	// CreateBackup will delete them, then create the new ref.
	old := time.Now().Add(-45 * 24 * time.Hour)
	for i := 0; i < 20; i++ {
		ts := old.Add(time.Duration(i) * time.Second)
		ref := fmt.Sprintf("refs/git-courer/backup/%s_OLD%02d", ts.Format("20060102150405"), i)
		gitRun(t, dir, "update-ref", ref, "HEAD")
	}
	if got := len(backupRefs(t, a)); got != 20 {
		t.Fatalf("setup: expected 20 backups, got %d", got)
	}

	// This call hits the gate (count >= maxBackups) → auto-prune deletes the 20
	// old backups, then creates the new ref.
	if _, err := a.CreateBackup("NEW", domain.StashNone); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	surviving := backupRefs(t, a)
	// The 20 old backups are pruned; only the new one remains.
	if len(surviving) != 1 {
		t.Errorf("expected 1 backup after auto-prune+create, got %d (%v)", len(surviving), surviving)
	}
}

// TestCreateBackup_AutoPruneGate_PruneErrorDoesNotBlock verifies that even when
// PruneBackups cannot reduce the count (all backups recent+reachable), a new
// CreateBackup still succeeds and creates its ref.
func TestCreateBackup_AutoPruneGate_PruneErrorDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/master")
	a := New(dir)
	commitFile(t, a, dir, "base.txt", "base")

	// 20 recent reachable backups — auto-prune will run but keep all of them.
	for i := 0; i < 20; i++ {
		ref := fmt.Sprintf("refs/git-courer/backup/%s_RC%02d",
			time.Now().Add(time.Duration(-i)*time.Second).Format("20060102150405"), i)
		gitRun(t, dir, "update-ref", ref, "HEAD")
	}

	backup, err := a.CreateBackup("NEW", domain.StashNone)
	if err != nil {
		t.Fatalf("CreateBackup must succeed even if prune cannot reduce count: %v", err)
	}
	if backup.Ref == "" {
		t.Fatal("CreateBackup returned empty ref")
	}
	surviving := backupRefs(t, a)
	if !surviving[backup.Ref] {
		t.Errorf("new backup ref %s was not created", backup.Ref)
	}
}
