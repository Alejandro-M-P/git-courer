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

func TestFilesystemCommitStore_SetBranch_SetsPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.SetBranch("feat/auth")
	if err != nil {
		t.Fatalf("SetBranch() error: %v", err)
	}

	// Verify path is branch-scoped
	expectedDir := filepath.Join(tmpDir, ".git/git-courer", "branches", "feat-auth")
	expectedPath := filepath.Join(expectedDir, "commits.json")

	if store.currentDir != expectedDir {
		t.Errorf("currentDir = %q, want %q", store.currentDir, expectedDir)
	}
	if store.path != expectedPath {
		t.Errorf("path = %q, want %q", store.path, expectedPath)
	}
	if store.branch != "feat/auth" {
		t.Errorf("branch = %q, want %q", store.branch, "feat/auth")
	}
}

func TestFilesystemCommitStore_SetBranch_EmptyName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.SetBranch("")
	if err == nil {
		t.Fatal("SetBranch(\"\") should return an error")
	}
	if !strings.Contains(err.Error(), "branch name must not be empty") {
		t.Errorf("error message = %q, want mention of empty branch name", err.Error())
	}
}

func TestFilesystemCommitStore_SetBranch_SwitchesPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	err := store.SetBranch("feat/auth")
	if err != nil {
		t.Fatalf("first SetBranch() error: %v", err)
	}
	firstPath := store.path

	err = store.SetBranch("main")
	if err != nil {
		t.Fatalf("second SetBranch() error: %v", err)
	}
	secondPath := store.path

	if firstPath == secondPath {
		t.Errorf("path did not change after second SetBranch: %q", firstPath)
	}
	if store.branch != "main" {
		t.Errorf("branch = %q, want %q", store.branch, "main")
	}
}

func TestFilesystemCommitStore_RemoveBranch_DeletesDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	// SetBranch and Append to create the directory and file
	if err := store.SetBranch("feat/auth"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
	}
	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: first commit")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Verify directory exists
	branchDir := filepath.Join(tmpDir, ".git/git-courer", "branches", "feat-auth")
	if _, err := os.Stat(branchDir); os.IsNotExist(err) {
		t.Fatalf("branch directory should exist after Append: %v", err)
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
			if err := store.SetBranch(branch); err != nil {
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

func TestFilesystemCommitStore_SanitizePathError_WithBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetBranch("feat/auth"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
	}

	// Trigger an error by making the path unwritable
	branchDir := filepath.Join(tmpDir, ".git/git-courer", "branches", "feat-auth")
	if err := os.MkdirAll(branchDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	// Create a directory where the file should be
	if err := os.MkdirAll(filepath.Join(branchDir, "commits.json"), 0o755); err != nil {
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

// --- T1.4: Integration tests for branch-scoped commit store ---

func TestFilesystemCommitStore_AfterSetBranch_AppendWritesToBranchFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetBranch("feat/auth"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
	}

	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: branch commit")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Verify the file exists at the branch-scoped path
	branchFilePath := filepath.Join(tmpDir, ".git/git-courer", "branches", "feat-auth", "commits.json")
	data, err := os.ReadFile(branchFilePath)
	if err != nil {
		t.Fatalf("branch file should exist at %s: %v", branchFilePath, err)
	}
	if len(data) == 0 {
		t.Errorf("branch file is empty")
	}

	// Verify the legacy file does NOT exist
	legacyPath := filepath.Join(tmpDir, ".git/git-courer", "commits.json")
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("legacy file should not exist when SetBranch was called")
	}
}

func TestFilesystemCommitStore_AfterSetBranch_ReadReturnsBranchEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetBranch("feat/auth"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
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

func TestFilesystemCommitStore_AfterSetBranch_ClearTruncatesBranchFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetBranch("feat/auth"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
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

	// Verify branch directory still exists (only file truncated, dir preserved)
	branchDir := filepath.Join(tmpDir, ".git/git-courer", "branches", "feat-auth")
	info, err := os.Stat(branchDir)
	if err != nil {
		t.Fatalf("branch directory should still exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("branch path is not a directory")
	}
}

func TestFilesystemCommitStore_AfterSetBranch_MkdirAllLazy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir, nil)

	if err := store.SetBranch("feat/auth"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
	}

	// Directory should NOT exist yet (lazy creation)
	branchDir := filepath.Join(tmpDir, ".git/git-courer", "branches", "feat-auth")
	if _, err := os.Stat(branchDir); !os.IsNotExist(err) {
		t.Errorf("branch directory should not exist before Append, got err: %v", err)
	}

	// After Append, directory should exist
	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: trigger mkdir")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	if _, err := os.Stat(branchDir); os.IsNotExist(err) {
		t.Errorf("branch directory should exist after Append")
	}
}

func TestFilesystemCommitStore_BranchIsolation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create store for feat/auth branch
	storeAuth := NewFilesystemCommitStore(tmpDir, nil)
	if err := storeAuth.SetBranch("feat/auth"); err != nil {
		t.Fatalf("SetBranch(feat/auth) error: %v", err)
	}

	// Create store for main branch
	storeMain := NewFilesystemCommitStore(tmpDir, nil)
	if err := storeMain.SetBranch("main"); err != nil {
		t.Fatalf("SetBranch(main) error: %v", err)
	}

	// Write different entries to different branches
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

	if err := store.SetBranch("main"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
	}

	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: first")
	e2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: second")
	if err := store.Append(e1, e2); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

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
	storeA := NewFilesystemCommitStore(tmpDir, nil)
	if err := storeA.SetBranch("feature-a"); err != nil {
		t.Fatalf("SetBranch(feature-a) error: %v", err)
	}
	sharedSHA := validSHA("a1000000000000000000000000000000000000")
	eA1 := makeEntry(t, sharedSHA, "feat: shared commit")
	eA2 := makeEntry(t, validSHA("a2000000000000000000000000000000000000"), "feat: feature-a only")
	storeA.Append(eA1, eA2)

	storeB := NewFilesystemCommitStore(tmpDir, nil)
	if err := storeB.SetBranch("feature-b"); err != nil {
		t.Fatalf("SetBranch(feature-b) error: %v", err)
	}
	eB1 := makeEntry(t, sharedSHA, "feat: shared commit") // same SHA as eA1
	eB2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: feature-b only")
	storeB.Append(eB1, eB2)

	// Now read from a fresh store (to ensure path is not set to a particular branch)
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
	storeValid := NewFilesystemCommitStore(tmpDir, nil)
	if err := storeValid.SetBranch("valid-branch"); err != nil {
		t.Fatalf("SetBranch() error: %v", err)
	}
	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: valid")
	storeValid.Append(e1)

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

	// Create some branch directories
	storeA := NewFilesystemCommitStore(tmpDir, nil)
	storeA.SetBranch("feature-a")
	storeA.Append(makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: a"))

	storeB := NewFilesystemCommitStore(tmpDir, nil)
	storeB.SetBranch("feature-b")
	storeB.Append(makeEntry(t, validSHA("bb000000000000000000000000000000000000"), "feat: b"))

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
