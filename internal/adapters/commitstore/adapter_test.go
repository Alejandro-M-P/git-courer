package commitstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
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
	store := NewFilesystemCommitStore(tmpDir)

	entry := makeEntry(t, validSHA("aa000000000000000000000000000000000000"), "feat: first commit")

	err := store.Append(entry)
	if err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Verify .git-courer/ directory was created
	dirPath := filepath.Join(tmpDir, ".git-courer")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("expected .git-courer/ dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf(".git-courer is not a directory")
	}

	// Verify file was created with one line
	filePath := filepath.Join(tmpDir, ".git-courer", "commits.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := nonEmptyLines(string(data))
	if len(lines) != 1 {
		t.Errorf("expected 1 line in file, got %d", len(lines))
	}

	// Verify the line is valid JSON matching the entry
	var parsed map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if parsed["sha"] != entry.SHA() {
		t.Errorf("parsed sha = %q, want %q", parsed["sha"], entry.SHA())
	}
	if parsed["message"] != entry.Message() {
		t.Errorf("parsed message = %q, want %q", parsed["message"], entry.Message())
	}
}

func TestFilesystemCommitStore_AppendAddsToExistingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir)

	e1 := makeEntry(t, validSHA("a1000000000000000000000000000000000000"), "feat: first")
	e2 := makeEntry(t, validSHA("b2000000000000000000000000000000000000"), "fix: second")

	if err := store.Append(e1); err != nil {
		t.Fatalf("first Append() error: %v", err)
	}
	if err := store.Append(e2); err != nil {
		t.Fatalf("second Append() error: %v", err)
	}

	filePath := filepath.Join(tmpDir, ".git-courer", "commits.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := nonEmptyLines(string(data))
	if len(lines) != 2 {
		t.Errorf("expected 2 lines in file, got %d", len(lines))
	}
}

func TestFilesystemCommitStore_ReadReturnsAllEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewFilesystemCommitStore(tmpDir)

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
	store := NewFilesystemCommitStore(tmpDir)

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
	store := NewFilesystemCommitStore(tmpDir)

	// Manually write a file with a corrupted line
	dirPath := filepath.Join(tmpDir, ".git-courer")
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
	store := NewFilesystemCommitStore(tmpDir)

	entry := makeEntry(t, validSHA("cc000000000000000000000000000000000000"), "feat: to be cleared")
	store.Append(entry)

	err := store.Clear()
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	// Verify file exists but is empty
	filePath := filepath.Join(tmpDir, ".git-courer", "commits.json")
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
	store := NewFilesystemCommitStore(tmpDir)

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
	store := NewFilesystemCommitStore(tmpDir)

	var wg sync.WaitGroup
	goroutines := 2
	entriesPerGoroutine := 5

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(groupID int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				suffix := string(rune('0' + groupID)) + string(rune('0' + i))
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
	store := NewFilesystemCommitStore(tmpDir)

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