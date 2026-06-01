package commitstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

func TestFilesystemCommitStore_Reconcile(t *testing.T) {
	t.Parallel()

	t.Run("Reconcile empty store with empty git log is no-op", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := NewFilesystemCommitStore(tmpDir)

		err := store.Reconcile([]domain.CommitEntry{})
		if err != nil {
			t.Fatalf("Reconcile empty error: %v", err)
		}

		// Verify no file was created
		filePath := filepath.Join(tmpDir, ".git-courer", "commits.json")
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("expected no file to be created for empty reconcile, but it exists")
		}
	})

	t.Run("Reconcile add missing entries to empty store", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := NewFilesystemCommitStore(tmpDir)

		gitEntries := []domain.CommitEntry{
			makeEntry(t, validSHA("1111"), "feat: add user auth"),
			makeEntry(t, validSHA("2222"), "fix: session timeout"),
		}

		err := store.Reconcile(gitEntries)
		if err != nil {
			t.Fatalf("Reconcile error: %v", err)
		}

		// Read and verify
		readEntries, err := store.Read()
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		if len(readEntries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(readEntries))
		}
		if readEntries[0].SHA() != gitEntries[0].SHA() || readEntries[1].SHA() != gitEntries[1].SHA() {
			t.Errorf("SHAs do not match")
		}
	})

	t.Run("Reconcile delete stale entries and keep matching ones", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := NewFilesystemCommitStore(tmpDir)

		e1 := makeEntry(t, validSHA("1111"), "feat: add user auth")
		e2 := makeEntry(t, validSHA("2222"), "fix: session timeout")
		e3 := makeEntry(t, validSHA("3333"), "docs: update readme")

		// Write e1, e2, e3
		if err := store.Append(e1, e2, e3); err != nil {
			t.Fatalf("Append error: %v", err)
		}

		// Reconcile with only e1 and e3 (e2 is stale/removed)
		gitEntries := []domain.CommitEntry{e1, e3}
		if err := store.Reconcile(gitEntries); err != nil {
			t.Fatalf("Reconcile error: %v", err)
		}

		// Read and verify
		readEntries, err := store.Read()
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		if len(readEntries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(readEntries))
		}
		if readEntries[0].SHA() != e1.SHA() || readEntries[1].SHA() != e3.SHA() {
			t.Errorf("expected e1 and e3, got different entries")
		}
	})

	t.Run("Reconcile update modified entries", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := NewFilesystemCommitStore(tmpDir)

		e1 := makeEntry(t, validSHA("1111"), "feat: add user auth")
		if err := store.Append(e1); err != nil {
			t.Fatalf("Append error: %v", err)
		}

		// Reconcile with updated message
		e1Updated := makeEntry(t, validSHA("1111"), "feat: add user authentication")
		if err := store.Reconcile([]domain.CommitEntry{e1Updated}); err != nil {
			t.Fatalf("Reconcile error: %v", err)
		}

		readEntries, err := store.Read()
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		if len(readEntries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(readEntries))
		}
		if readEntries[0].Message() != "feat: add user authentication" {
			t.Errorf("expected updated message, got %q", readEntries[0].Message())
		}
	})

	t.Run("Reconcile skips write if contents are identical", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := NewFilesystemCommitStore(tmpDir)

		e1 := makeEntry(t, validSHA("1111"), "feat: add user auth")
		e2 := makeEntry(t, validSHA("2222"), "fix: session timeout")
		if err := store.Append(e1, e2); err != nil {
			t.Fatalf("Append error: %v", err)
		}

		filePath := filepath.Join(tmpDir, ".git-courer", "commits.json")

		// Change file permissions to 0400 (read-only)
		if err := os.Chmod(filePath, 0400); err != nil {
			t.Fatalf("Chmod error: %v", err)
		}
		defer func() {
			_ = os.Chmod(filePath, 0644)
		}()

		// Reconcile with identical entries should succeed (skips write)
		if err := store.Reconcile([]domain.CommitEntry{e1, e2}); err != nil {
			t.Fatalf("Reconcile on read-only identical file failed: %v", err)
		}

		// Restore write permissions
		if err := os.Chmod(filePath, 0644); err != nil {
			t.Fatalf("Chmod restore error: %v", err)
		}

		// Reconcile with different entries (needs write) should succeed now
		if err := store.Reconcile([]domain.CommitEntry{e1}); err != nil {
			t.Fatalf("Reconcile write failed: %v", err)
		}

		readEntries, err := store.Read()
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		if len(readEntries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(readEntries))
		}
	})
}
