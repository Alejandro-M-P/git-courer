package commitstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// validSHA returns a 40-char hex string for testing.
// suffix must be a hex string; the base is padded to 40 chars.
func validSHA(suffix string) string {
	base := "0000000000000000000000000000000000000000"
	if len(suffix) <= 40 {
		padded := suffix + base[len(suffix):]
		return padded[:40]
	}
	return base
}

func makeEntry(t *testing.T, sha, message string, opts ...domain.CommitEntryOption) domain.CommitEntry {
	t.Helper()
	entry, err := domain.NewCommitEntry(sha, message, opts...)
	if err != nil {
		t.Fatalf("makeEntry(%q, %q) failed: %v", sha, message, err)
	}
	return entry
}

func TestFilesystemCommitStore_AppendCreatesFileAndDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: first commit")

	err := store.Append(entry)
	if err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Verify .git/git-courer/ directory was created
	dirPath := filepath.Join(tmpDir, ".git/git-courer")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("expected .git/git-courer/ dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf(".git/git-courer is not a directory")
	}

	// Verify file was created
	filePath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	// Verify the data is valid JSON array matching the entry
	var parsed []map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("file content is not valid JSON array: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 entry in array, got %d", len(parsed))
	}
	if parsed[0]["sha"] != entry.SHA() {
		t.Errorf("parsed sha = %q, want %q", parsed[0]["sha"], entry.SHA())
	}
	if parsed[0]["message"] != entry.Message() {
		t.Errorf("parsed message = %q, want %q", parsed[0]["message"], entry.Message())
	}
}

func TestFilesystemCommitStore_AppendAddsToExistingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: first")
	e2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: second")

	if err := store.Append(e1); err != nil {
		t.Fatalf("first Append() error: %v", err)
	}
	if err := store.Append(e2); err != nil {
		t.Fatalf("second Append() error: %v", err)
	}

	filePath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var parsed []map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("file content is not valid JSON array: %v", err)
	}
	if len(parsed) != 2 {
		t.Errorf("expected 2 entries in array, got %d", len(parsed))
	}
}

func TestFilesystemCommitStore_ReadReturnsAllEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	e1 := makeEntry(t, validSHA("10000000000000000000000000000000000000"), "feat: first", domain.WithAuthor("Alice"), domain.WithDate("2026-05-23T10:00:00Z"))
	e2 := makeEntry(t, validSHA("20000000000000000000000000000000000000"), "fix: second")

	store.Append(e1, e2)

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Read() returned %d entries, want 2", len(entries))
	}
	if entries[0].SHA() != e1.SHA() {
		t.Errorf("entries[0].SHA() = %q, want %q", entries[0].SHA(), e1.SHA())
	}
	if entries[0].Message() != e1.Message() {
		t.Errorf("entries[0].Message() = %q, want %q", entries[0].Message(), e1.Message())
	}
	if entries[0].Author() != "Alice" {
		t.Errorf("entries[0].Author() = %q, want %q", entries[0].Author(), "Alice")
	}
	if entries[0].Date() != "2026-05-23T10:00:00Z" {
		t.Errorf("entries[0].Date() = %q, want %q", entries[0].Date(), "2026-05-23T10:00:00Z")
	}
	if entries[1].Message() != e2.Message() {
		t.Errorf("entries[1].Message() = %q, want %q", entries[1].Message(), e2.Message())
	}
}

func TestFilesystemCommitStore_ReadOnMissingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() on missing file returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Read() on missing file returned %d entries, want 0", len(entries))
	}
}

func TestFilesystemCommitStore_ReadWithCorruptedLine(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Manually write a file with a corrupted line
	dirPath := filepath.Join(tmpDir, ".git/git-courer")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	validEntry := makeEntry(t, validSHA("c1000000000000000000000000000000000000"), "feat: valid entry")
	validJSON, _ := json.Marshal(testJSON{
		SHA:     validEntry.SHA(),
		Message: validEntry.Message(),
		Author:  validEntry.Author(),
		Date:    validEntry.Date(),
	})

	content := string(validJSON) + "\n{corrupted json\n" + string(validJSON) + "\n"
	filePath := filepath.Join(dirPath, "commits.json")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() with corrupted line returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("Read() returned %d entries, want 2 (skipping 1 corrupted)", len(entries))
	}
}

func TestFilesystemCommitStore_ClearTruncatesFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	entry := makeEntry(t, validSHA("cc000000000000000000000000000000000000"), "feat: to be cleared")
	store.Append(entry)

	err := store.Clear()
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	// Verify file exists but is empty
	filePath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() after Clear() error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("file after Clear() has %d bytes, want 0", len(data))
	}

	// Verify Read() returns empty
	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() after Clear() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Read() after Clear() returned %d entries, want 0", len(entries))
	}
}

func TestFilesystemCommitStore_ClearOnAlreadyEmpty(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Clear on store that was never written to — no file exists
	err := store.Clear()
	if err != nil {
		t.Fatalf("Clear() on empty store returned error: %v", err)
	}

	// Clear on store after truncation
	entry := makeEntry(t, validSHA("dd000000000000000000000000000000000000"), "feat: temp")
	store.Append(entry)
	store.Clear()

	err = store.Clear()
	if err != nil {
		t.Fatalf("second Clear() on empty store returned error: %v", err)
	}
}

func TestFilesystemCommitStore_ConcurrentAppendSafety(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	var wg sync.WaitGroup
	goroutines := 2
	entriesPerGoroutine := 5

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(groupID int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				suffix := string(rune('0'+groupID)) + string(rune('0'+i))
				entry := makeEntry(t, validSHA(suffix), "feat: concurrent commit")
				if err := store.Append(entry); err != nil {
					t.Errorf("concurrent Append() error: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()

	// All entries should be readable
	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() after concurrent Append() error: %v", err)
	}
	want := goroutines * entriesPerGoroutine
	if len(entries) != want {
		t.Errorf("Read() after concurrent Append() returned %d entries, want %d", len(entries), want)
	}
}

func TestFilesystemCommitStore_AppendMultiple(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	e1 := makeEntry(t, validSHA("10000000000000000000000000000000000001"), "feat: batch one")
	e2 := makeEntry(t, validSHA("20000000000000000000000000000000000002"), "fix: batch two")
	e3 := makeEntry(t, validSHA("30000000000000000000000000000000000003"), "chore: batch three")

	err := store.Append(e1, e2, e3)
	if err != nil {
		t.Fatalf("Append(multiple) error: %v", err)
	}

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Read() returned %d entries, want 3", len(entries))
	}
	if entries[0].Message() != "feat: batch one" {
		t.Errorf("entries[0].Message() = %q, want %q", entries[0].Message(), "feat: batch one")
	}
	if entries[1].Message() != "fix: batch two" {
		t.Errorf("entries[1].Message() = %q, want %q", entries[1].Message(), "fix: batch two")
	}
	if entries[2].Message() != "chore: batch three" {
		t.Errorf("entries[2].Message() = %q, want %q", entries[2].Message(), "chore: batch three")
	}
}

func TestFilesystemCommitStore_SetWorkspace_BranchName_SetsPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.SetWorkspace("feat/auth")
	if err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	// Verify path is workspace-scoped (branch name sanitized)
	expectedDir := filepath.Join(tmpDir, ".git/git-courer", "workspace", "feat-auth")
	expectedPath := filepath.Join(expectedDir, "commits.json")

	if store.currentDir != expectedDir {
		t.Errorf("currentDir = %q, want %q", store.currentDir, expectedDir)
	}
	if store.path != expectedPath {
		t.Errorf("path = %q, want %q", store.path, expectedPath)
	}
	if store.workspace != "feat/auth" {
		t.Errorf("workspace = %q, want %q", store.workspace, "feat/auth")
	}
}

func TestFilesystemCommitStore_SetWorkspace_UUID_PassesUnchanged(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	uuid := "a1b2c3d4-e5f6-4789-8a01-234567890abc"
	err := store.SetWorkspace(uuid)
	if err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	// UUIDs are filesystem-safe and pass through unsanitized.
	expectedDir := filepath.Join(tmpDir, ".git/git-courer", "workspace", uuid)
	if store.currentDir != expectedDir {
		t.Errorf("currentDir = %q, want %q", store.currentDir, expectedDir)
	}
}

func TestFilesystemCommitStore_SetWorkspace_EmptyName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.SetWorkspace("")
	if err == nil {
		t.Fatal("SetWorkspace(\"\") should return an error")
	}
	if !strings.Contains(err.Error(), "workspace id must not be empty") {
		t.Errorf("error message = %q, want mention of empty workspace id", err.Error())
	}
}

func TestFilesystemCommitStore_SetWorkspace_SwitchesPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.SetWorkspace("feat/auth")
	if err != nil {
		t.Fatalf("first SetWorkspace() error: %v", err)
	}
	firstPath := store.path

	err = store.SetWorkspace("main")
	if err != nil {
		t.Fatalf("second SetWorkspace() error: %v", err)
	}
	secondPath := store.path

	if firstPath == secondPath {
		t.Errorf("path did not change after second SetWorkspace: %q", firstPath)
	}
	if store.workspace != "main" {
		t.Errorf("workspace = %q, want %q", store.workspace, "main")
	}
}

func TestFilesystemCommitStore_RemoveBranch_DeletesDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Create a branches/ directory manually (legacy coexistence path).
	branchDir := filepath.Join(tmpDir, ".git/git-courer", "branches", "feat-auth")
	if err := os.MkdirAll(branchDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	commitsPath := filepath.Join(branchDir, "commits.json")
	if err := os.WriteFile(commitsPath, []byte("[]"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	err := store.RemoveBranch("feat/auth")
	if err != nil {
		t.Fatalf("RemoveBranch() error: %v", err)
	}

	// Verify directory no longer exists
	if _, err := os.Stat(branchDir); !os.IsNotExist(err) {
		t.Errorf("branch directory should not exist after RemoveBranch")
	}
}

func TestFilesystemCommitStore_RemoveBranch_NonexistentBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Removing a branch that was never created should return nil (idempotent)
	err := store.RemoveBranch("nonexistent")
	if err != nil {
		t.Errorf("RemoveBranch on nonexistent dir should return nil, got: %v", err)
	}
}

func TestFilesystemCommitStore_RemoveBranch_EmptyName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.RemoveBranch("")
	if err == nil {
		t.Fatal("RemoveBranch(\"\") should return an error")
	}
	if !strings.Contains(err.Error(), "branch name must not be empty") {
		t.Errorf("error message = %q, want mention of empty branch name", err.Error())
	}
}

func TestFilesystemCommitStore_NoSetBranch_LegacyPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// No SetBranch call — should write to legacy path
	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: first commit")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Verify file was created at legacy path
	legacyPath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		t.Fatalf("legacy file should exist at %s", legacyPath)
	}

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Read() returned %d entries, want 1", len(entries))
	}
}

func TestFilesystemCommitStore_SetBranch_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent SetBranch + Append goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			branch := "feat/branch"
			if err := store.SetWorkspace(branch); err != nil {
				errCh <- err
				return
			}
			suffix := fmt.Sprintf("%040d", idx)
			entry := makeEntry(t, suffix, "concurrent commit")
			if err := store.Append(entry); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent access error: %v", err)
	}

	// After concurrent access, all entries should be readable
	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() after concurrent access error: %v", err)
	}
	if len(entries) != 10 {
		t.Errorf("Read() returned %d entries, want 10", len(entries))
	}
}

func TestFilesystemCommitStore_SanitizePathError_WithWorkspace(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetWorkspace("feat/auth"); err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	// Trigger an error by making the path unwritable
	workspaceDir := filepath.Join(tmpDir, ".git/git-courer", "workspace", "feat-auth")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	// Create a directory where the file should be
	if err := os.MkdirAll(filepath.Join(workspaceDir, "commits.json"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	err := store.Append(makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "test"))
	if err == nil {
		t.Fatal("expected error when writing to a directory instead of file")
	}

	// Error should contain sanitized path, not the real filesystem path
	if strings.Contains(err.Error(), tmpDir) {
		t.Errorf("error leaks filesystem path: %v", err)
	}
	if !strings.Contains(err.Error(), "<commit-store>") {
		t.Errorf("error should contain <commit-store> placeholder: %v", err)
	}
}

// --- T1.4: Integration tests for workspace-scoped commit store ---

func TestFilesystemCommitStore_AfterSetWorkspace_AppendWritesToWorkspaceFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetWorkspace("feat/auth"); err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: branch commit")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Verify the file exists at the workspace-scoped path
	workspaceFilePath := filepath.Join(tmpDir, ".git/git-courer", "workspace", "feat-auth", "commits.json")
	data, err := os.ReadFile(workspaceFilePath)
	if err != nil {
		t.Fatalf("workspace file should exist at %s: %v", workspaceFilePath, err)
	}
	if len(data) == 0 {
		t.Errorf("workspace file is empty")
	}

	// Verify the legacy file does NOT exist
	legacyPath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("legacy file should not exist when SetWorkspace was called")
	}
}

func TestFilesystemCommitStore_AfterSetWorkspace_ReadReturnsWorkspaceEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetWorkspace("feat/auth"); err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: first on branch")
	e2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "feat: second on branch")

	store.Append(e1, e2)

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Read() returned %d entries, want 2", len(entries))
	}
	if entries[0].SHA() != e1.SHA() {
		t.Errorf("entries[0].SHA() = %q, want %q", entries[0].SHA(), e1.SHA())
	}
	if entries[1].SHA() != e2.SHA() {
		t.Errorf("entries[1].SHA() = %q, want %q", entries[1].SHA(), e2.SHA())
	}
}

func TestFilesystemCommitStore_AfterSetWorkspace_ClearTruncatesWorkspaceFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetWorkspace("feat/auth"); err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: to be cleared")
	store.Append(entry)

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() after Clear() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Read() after Clear() returned %d entries, want 0", len(entries))
	}

	// Verify workspace directory still exists (only file truncated, dir preserved)
	workspaceDir := filepath.Join(tmpDir, ".git/git-courer", "workspace", "feat-auth")
	info, err := os.Stat(workspaceDir)
	if err != nil {
		t.Fatalf("workspace directory should still exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("workspace path is not a directory")
	}
}

func TestFilesystemCommitStore_AfterSetWorkspace_MkdirAllLazy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetWorkspace("feat/auth"); err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	// Directory should NOT exist yet (lazy creation)
	workspaceDir := filepath.Join(tmpDir, ".git/git-courer", "workspace", "feat-auth")
	if _, err := os.Stat(workspaceDir); !os.IsNotExist(err) {
		t.Errorf("workspace directory should not exist before Append, got err: %v", err)
	}

	// After Append, directory should exist
	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: trigger mkdir")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Errorf("workspace directory should exist after Append")
	}
}

func TestFilesystemCommitStore_WorkspaceIsolation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create store for feat/auth workspace
	storeAuth := NewFilesystemCommitStore(tmpDir, nil)
	if err := storeAuth.SetWorkspace("feat/auth"); err != nil {
		t.Fatalf("SetWorkspace(feat/auth) error: %v", err)
	}

	// Create store for main workspace
	storeMain := NewFilesystemCommitStore(tmpDir, nil)
	if err := storeMain.SetWorkspace("main"); err != nil {
		t.Fatalf("SetWorkspace(main) error: %v", err)
	}

	// Write different entries to different workspaces
	authEntry := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: auth feature")
	mainEntry := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: main bugfix")

	if err := storeAuth.Append(authEntry); err != nil {
		t.Fatalf("storeAuth.Append() error: %v", err)
	}
	if err := storeMain.Append(mainEntry); err != nil {
		t.Fatalf("storeMain.Append() error: %v", err)
	}

	// Verify each store only reads its own entries
	authEntries, err := storeAuth.Read()
	if err != nil {
		t.Fatalf("storeAuth.Read() error: %v", err)
	}
	if len(authEntries) != 1 || authEntries[0].Message() != "feat: auth feature" {
		t.Errorf("storeAuth should have 1 entry with auth message, got %v", authEntries)
	}

	mainEntries, err := storeMain.Read()
	if err != nil {
		t.Fatalf("storeMain.Read() error: %v", err)
	}
	if len(mainEntries) != 1 || mainEntries[0].Message() != "fix: main bugfix" {
		t.Errorf("storeMain should have 1 entry with main message, got %v", mainEntries)
	}
}

// nonEmptyLines returns non-empty lines from a string, trimming trailing whitespace.
func nonEmptyLines(s string) []string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

// testJSON mirrors the JSON structure used for serialization in tests.
type testJSON struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

func TestFilesystemCommitStore_JSONFormatAndFallback(t *testing.T) {
	t.Run("JSON array format on write", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := NewFilesystemCommitStore(tmpDir, nil)

		e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: first")
		e2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: second")

		if err := store.Append(e1, e2); err != nil {
			t.Fatalf("Append error: %v", err)
		}

		filePath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}

		// Verify it is a valid JSON array on disk
		var entries []testJSON
		if err := json.Unmarshal(data, &entries); err != nil {
			t.Fatalf("File contents not a valid JSON array: %v\nData: %s", err, string(data))
		}

		if len(entries) != 2 {
			t.Errorf("Expected 2 serialized entries, got %d", len(entries))
		}

		// Check formatting has double-space indent by looking for "  {"
		content := string(data)
		if !strings.Contains(content, "\n  {") && !strings.Contains(content, "\r\n  {") {
			t.Errorf("Expected double-space indented JSON formatting, got content:\n%s", content)
		}
	})

	t.Run("Fallback to reading legacy JSONL", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := NewFilesystemCommitStore(tmpDir, nil)

		// Create legacy JSONL content manually
		legacyContent := `{"sha":"a100000000000000000000000000000000000000","message":"feat: legacy first","author":"Alice","date":""}
{"sha":"b200000000000000000000000000000000000000","message":"fix: legacy second","author":"Bob","date":""}`

		filePath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("MkdirAll error: %v", err)
		}
		if err := os.WriteFile(filePath, []byte(legacyContent), 0o644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		// Read using adapter
		entries, err := store.Read()
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("Expected to read 2 legacy entries, got %d", len(entries))
		}

		if entries[0].SHA() != validSHA("a100000000000000000000000000000000000000") || entries[0].Message() != "feat: legacy first" {
			t.Errorf("First entry mismatch: %+v", entries[0])
		}
		if entries[1].SHA() != validSHA("b200000000000000000000000000000000000000") || entries[1].Message() != "fix: legacy second" {
			t.Errorf("Second entry mismatch: %+v", entries[1])
		}
	})
}

// --- ReadAllBranches tests ---

// writeBranchStore writes entries to a branches/<sanitized>/commits.json file
// under the store's baseDir. Used to set up legacy branches/ data for
// ReadAllBranches tests without going through SetWorkspace (which writes
// to workspace/ instead).
func writeBranchStore(t *testing.T, tmpDir, branchName string, entries []domain.CommitEntry) {
	t.Helper()
	sanitized := SanitizeBranchName(branchName)
	branchDir := filepath.Join(tmpDir, ".git/git-courer", "branches", sanitized)
	if err := os.MkdirAll(branchDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error: %v", branchDir, err)
	}
	var jEntries []jsonEntry
	for _, e := range entries {
		jEntries = append(jEntries, jsonEntry{
			SHA:     e.SHA(),
			Message:  e.Message(),
			Author:   e.Author(),
			Date:     e.Date(),
			Branch:   e.Branch(),
		})
	}
	data, err := json.MarshalIndent(jEntries, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(branchDir, "commits.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func TestFilesystemCommitStore_ReadAllBranches_EmptyDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	result, err := store.ReadAllBranches()
	if err != nil {
		t.Fatalf("ReadAllBranches() on empty dir returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("ReadAllBranches() on empty dir returned %d branches, want 0", len(result))
	}
}

func TestFilesystemCommitStore_ReadAllBranches_SingleBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: first")
	e2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: second")
	writeBranchStore(t, tmpDir, "main", []domain.CommitEntry{e1, e2})

	result, err := store.ReadAllBranches()
	if err != nil {
		t.Fatalf("ReadAllBranches() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("ReadAllBranches() returned %d branches, want 1", len(result))
	}
	entries, ok := result["main"]
	if !ok {
		t.Fatal("ReadAllBranches() missing 'main' branch")
	}
	if len(entries) != 2 {
		t.Errorf("ReadAllBranches()['main'] returned %d entries, want 2", len(entries))
	}
}

func TestFilesystemCommitStore_ReadAllBranches_MultipleBranches_Dedup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create two branch stores with entries, one SHA shared
	sharedSHA := validSHA("a1000000000000000000000000000000000000")
	eA1 := makeEntry(t, sharedSHA, "feat: shared commit")
	eA2 := makeEntry(t, validSHA("a2000000000000000000000000000000000000"), "feat: feature-a only")
	writeBranchStore(t, tmpDir, "feature-a", []domain.CommitEntry{eA1, eA2})

	eB1 := makeEntry(t, sharedSHA, "feat: shared commit") // same SHA as eA1
	eB2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: feature-b only")
	writeBranchStore(t, tmpDir, "feature-b", []domain.CommitEntry{eB1, eB2})

	// Read from a fresh store
	store := NewFilesystemCommitStore(tmpDir, nil)
	result, err := store.ReadAllBranches()
	if err != nil {
		t.Fatalf("ReadAllBranches() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("ReadAllBranches() returned %d branches, want 2", len(result))
	}

	// Each branch should have both its entries (dedup is in service layer, not adapter)
	featureA, ok := result["feature-a"]
	if !ok {
		t.Fatal("ReadAllBranches() missing 'feature-a' branch")
	}
	if len(featureA) != 2 {
		t.Errorf("ReadAllBranches()['feature-a'] returned %d entries, want 2", len(featureA))
	}

	featureB, ok := result["feature-b"]
	if !ok {
		t.Fatal("ReadAllBranches() missing 'feature-b' branch")
	}
	if len(featureB) != 2 {
		t.Errorf("ReadAllBranches()['feature-b'] returned %d entries, want 2", len(featureB))
	}
}

func TestFilesystemCommitStore_ReadAllBranches_CorruptBranchDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".git/git-courer", "branches")

	// Create a valid branch
	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: valid")
	writeBranchStore(t, tmpDir, "valid-branch", []domain.CommitEntry{e1})

	// Create a corrupt branch manually (invalid JSON)
	if err := os.MkdirAll(filepath.Join(baseDir, "corrupt-branch"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	corruptPath := filepath.Join(baseDir, "corrupt-branch", "commits.json")
	if err := os.WriteFile(corruptPath, []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	store := NewFilesystemCommitStore(tmpDir, nil)
	result, err := store.ReadAllBranches()
	if err != nil {
		t.Fatalf("ReadAllBranches() with corrupt branch should not error: %v", err)
	}

	// Valid branch should still be readable
	validEntries, ok := result["valid-branch"]
	if !ok {
		t.Fatal("ReadAllBranches() missing 'valid-branch'")
	}
	if len(validEntries) != 1 {
		t.Errorf("ReadAllBranches()['valid-branch'] returned %d entries, want 1", len(validEntries))
	}

	// Corrupt branch should be present but empty (error logged, not propagated)
	corruptEntries, ok := result["corrupt-branch"]
	if !ok {
		t.Fatal("ReadAllBranches() missing 'corrupt-branch'")
	}
	if len(corruptEntries) != 0 {
		t.Errorf("ReadAllBranches()['corrupt-branch'] returned %d entries, want 0 (corrupt JSON)", len(corruptEntries))
	}
}

// --- RemoveAllBranchDirs tests ---

func TestFilesystemCommitStore_RemoveAllBranchDirs_RemovesDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Create some branch directories manually (legacy coexistence path)
	writeBranchStore(t, tmpDir, "feature-a", []domain.CommitEntry{
		makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: a"),
	})
	writeBranchStore(t, tmpDir, "feature-b", []domain.CommitEntry{
		makeEntry(t, validSHA("bb000000000000000000000000000000000000"), "feat: b"),
	})

	// Verify branches directory exists
	branchesDir := filepath.Join(tmpDir, ".git/git-courer", "branches")
	if _, err := os.Stat(branchesDir); os.IsNotExist(err) {
		t.Fatalf("branches directory should exist before RemoveAllBranchDirs")
	}

	err := store.RemoveAllBranchDirs()
	if err != nil {
		t.Fatalf("RemoveAllBranchDirs() error: %v", err)
	}

	// Verify branches directory no longer exists
	if _, err := os.Stat(branchesDir); !os.IsNotExist(err) {
		t.Errorf("branches directory should not exist after RemoveAllBranchDirs")
	}
}

func TestFilesystemCommitStore_RemoveAllBranchDirs_Idempotent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Calling RemoveAllBranchDirs on a non-existent directory should not error
	err := store.RemoveAllBranchDirs()
	if err != nil {
		t.Errorf("RemoveAllBranchDirs() on non-existent dir should return nil, got: %v", err)
	}

	// Calling it twice should still work
	err = store.RemoveAllBranchDirs()
	if err != nil {
		t.Errorf("RemoveAllBranchDirs() second call should return nil, got: %v", err)
	}
}

// --- Phase 3.3: workspace storage tests ---

// writeWorkspaceStore writes entries to a workspace/<id>/commits.json file
// under the store's baseDir. Used to set up workspace/ data for
// ReadAllWorkspaces tests.
func writeWorkspaceStore(t *testing.T, tmpDir, workspaceID string, entries []domain.CommitEntry) {
	t.Helper()
	workspaceDir := filepath.Join(tmpDir, ".git/git-courer", "workspace", workspaceID)
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error: %v", workspaceDir, err)
	}
	var jEntries []jsonEntry
	for _, e := range entries {
		jEntries = append(jEntries, jsonEntry{
			SHA:     e.SHA(),
			Message: e.Message(),
			Author:  e.Author(),
			Date:    e.Date(),
			Branch:  e.Branch(),
		})
	}
	data, err := json.MarshalIndent(jEntries, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(workspaceDir, "commits.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func TestFilesystemCommitStore_ReadAllWorkspaces_EmptyDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	result, err := store.ReadAllWorkspaces()
	if err != nil {
		t.Fatalf("ReadAllWorkspaces() on empty dir returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("ReadAllWorkspaces() on empty dir returned %d workspaces, want 0", len(result))
	}
}

func TestFilesystemCommitStore_ReadAllWorkspaces_SingleWorkspace(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	uuid := "a1b2c3d4-e5f6-4789-8a01-234567890abc"
	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: first")
	e2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: second")
	writeWorkspaceStore(t, tmpDir, uuid, []domain.CommitEntry{e1, e2})

	result, err := store.ReadAllWorkspaces()
	if err != nil {
		t.Fatalf("ReadAllWorkspaces() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("ReadAllWorkspaces() returned %d workspaces, want 1", len(result))
	}
	entries, ok := result[uuid]
	if !ok {
		t.Fatalf("ReadAllWorkspaces() missing %q workspace", uuid)
	}
	if len(entries) != 2 {
		t.Errorf("ReadAllWorkspaces()[%q] returned %d entries, want 2", uuid, len(entries))
	}
}

func TestFilesystemCommitStore_ReadAllWorkspaces_MultipleWorkspaces(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	uuidA := "11111111-1111-4111-8111-111111111111"
	uuidB := "22222222-2222-4222-8222-222222222222"
	writeWorkspaceStore(t, tmpDir, uuidA, []domain.CommitEntry{
		makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: a"),
	})
	writeWorkspaceStore(t, tmpDir, uuidB, []domain.CommitEntry{
		makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: b"),
	})

	result, err := store.ReadAllWorkspaces()
	if err != nil {
		t.Fatalf("ReadAllWorkspaces() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("ReadAllWorkspaces() returned %d workspaces, want 2", len(result))
	}
	if _, ok := result[uuidA]; !ok {
		t.Errorf("ReadAllWorkspaces() missing %q", uuidA)
	}
	if _, ok := result[uuidB]; !ok {
		t.Errorf("ReadAllWorkspaces() missing %q", uuidB)
	}
}

func TestFilesystemCommitStore_RemoveAllWorkspaceDirs_RemovesDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Create workspace directories
	uuidA := "11111111-1111-4111-8111-111111111111"
	uuidB := "22222222-2222-4222-8222-222222222222"
	writeWorkspaceStore(t, tmpDir, uuidA, []domain.CommitEntry{
		makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: a"),
	})
	writeWorkspaceStore(t, tmpDir, uuidB, []domain.CommitEntry{
		makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "feat: b"),
	})

	workspacesDir := filepath.Join(tmpDir, ".git/git-courer", "workspace")
	if _, err := os.Stat(workspacesDir); os.IsNotExist(err) {
		t.Fatalf("workspace directory should exist before RemoveAllWorkspaceDirs")
	}

	err := store.RemoveAllWorkspaceDirs()
	if err != nil {
		t.Fatalf("RemoveAllWorkspaceDirs() error: %v", err)
	}

	if _, err := os.Stat(workspacesDir); !os.IsNotExist(err) {
		t.Errorf("workspace directory should not exist after RemoveAllWorkspaceDirs")
	}
}

func TestFilesystemCommitStore_RemoveAllWorkspaceDirs_Idempotent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.RemoveAllWorkspaceDirs()
	if err != nil {
		t.Errorf("RemoveAllWorkspaceDirs() on non-existent dir should return nil, got: %v", err)
	}

	err = store.RemoveAllWorkspaceDirs()
	if err != nil {
		t.Errorf("RemoveAllWorkspaceDirs() second call should return nil, got: %v", err)
	}
}

func TestFilesystemCommitStore_jsonEntry_BranchRoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// Append an entry WITH a branch; verify it round-trips through the file.
	entry := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: with branch",
		domain.WithBranch("feature/foo"))
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Read() returned %d entries, want 1", len(entries))
	}
	if got := entries[0].Branch(); got != "feature/foo" {
		t.Errorf("entries[0].Branch() = %q, want %q", got, "feature/foo")
	}
}

func TestFilesystemCommitStore_entriesEqual_BranchDifference(t *testing.T) {
	t.Parallel()

	store := NewFilesystemCommitStore(t.TempDir(), nil)

	sha := validSHA("a1000000000000000000000000000000000000")
	withBranchA := makeEntry(t, sha, "feat: same", domain.WithBranch("branch-a"))
	withBranchB := makeEntry(t, sha, "feat: same", domain.WithBranch("branch-b"))
	noBranch := makeEntry(t, sha, "feat: same")

	if store.entriesEqual([]domain.CommitEntry{withBranchA}, []domain.CommitEntry{withBranchB}) {
		t.Error("entriesEqual should return false when branches differ")
	}
	if store.entriesEqual([]domain.CommitEntry{withBranchA}, []domain.CommitEntry{noBranch}) {
		t.Error("entriesEqual should return false when one entry has branch and the other does not")
	}
	if !store.entriesEqual([]domain.CommitEntry{withBranchA}, []domain.CommitEntry{withBranchA}) {
		t.Error("entriesEqual should return true when entries are identical including branch")
	}
}
