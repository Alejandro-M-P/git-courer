// Package commitstore provides a filesystem-backed CommitStore adapter
// that persists commit entries as JSONL in .git-courer/commits.json.
package commitstore

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// jsonEntry is the JSON serialization format for CommitEntry.
type jsonEntry struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// FilesystemCommitStore implements ports.CommitStore using a JSONL file.
// Each line is a single JSON object representing one CommitEntry.
// Operations are serialized with a mutex for concurrent safety.
type FilesystemCommitStore struct {
	mu         sync.Mutex
	baseDir    string // workDir + "/.git-courer" — immutable after construction
	currentDir string // active directory: baseDir (legacy) or baseDir + "/branches/<sanitized>"
	path       string // active file path: currentDir + "/commits.json"
	branch     string // current unsanitized branch name (empty = legacy mode)
}

// NewFilesystemCommitStore creates a FilesystemCommitStore that persists
// entries in workDir/.git-courer/commits.json (legacy path).
// When SetBranch is called, the path switches to workDir/.git-courer/branches/<sanitized>/commits.json.
// The directory and file are created lazily on first Append.
func NewFilesystemCommitStore(workDir string) *FilesystemCommitStore {
	baseDir := filepath.Join(workDir, ".git-courer")
	return &FilesystemCommitStore{
		baseDir:    baseDir,
		currentDir: baseDir, // legacy: writes to .git-courer/commits.json
		path:       filepath.Join(baseDir, "commits.json"),
	}
}

// Append adds one or more CommitEntry values to the store.
// Each entry is written as a single JSON line appended to the file.
// The directory is created lazily if it does not exist.
func (s *FilesystemCommitStore) Append(entries ...domain.CommitEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.currentDir, 0o755); err != nil {
		return fmt.Errorf("commit store: create directory: %w", s.sanitizePathError(err))
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("commit store: open file: %w", s.sanitizePathError(err))
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, entry := range entries {
		line := jsonEntry{
			SHA:     entry.SHA(),
			Message: entry.Message(),
			Author:  entry.Author(),
			Date:    entry.Date(),
		}
		data, err := json.Marshal(line)
		if err != nil {
			return fmt.Errorf("commit store: marshal entry: %w", err)
		}
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("commit store: write entry: %w", s.sanitizePathError(err))
		}
		if _, err := writer.WriteRune('\n'); err != nil {
			return fmt.Errorf("commit store: write newline: %w", s.sanitizePathError(err))
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("commit store: flush: %w", s.sanitizePathError(err))
	}

	return nil
}

// Read returns all stored CommitEntry values.
// If the file does not exist, returns an empty slice with no error.
// Corrupted lines are skipped with a warning log.
func (s *FilesystemCommitStore) Read() ([]domain.CommitEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readLocked()
}

// readLocked reads entries from the store file without locking.
// It assumes the caller holds the lock.
func (s *FilesystemCommitStore) readLocked() ([]domain.CommitEntry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domain.CommitEntry{}, nil
		}
		return nil, fmt.Errorf("commit store: open for read: %w", s.sanitizePathError(err))
	}
	defer f.Close()

	var entries []domain.CommitEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		var je jsonEntry
		if err := json.Unmarshal([]byte(line), &je); err != nil {
			log.Printf("commit store: skipping corrupted line %d: %v", lineNum, err)
			continue
		}

		entry, err := domain.NewCommitEntry(je.SHA, je.Message,
			domain.WithAuthor(je.Author),
			domain.WithDate(je.Date),
		)
		if err != nil {
			log.Printf("commit store: skipping invalid entry at line %d: %v", lineNum, err)
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("commit store: scan: %w", s.sanitizePathError(err))
	}

	return entries, nil
}

// write overwrites the store file with the provided entries.
// It assumes the caller holds the lock.
func (s *FilesystemCommitStore) write(entries []domain.CommitEntry) error {
	if err := os.MkdirAll(s.currentDir, 0o755); err != nil {
		return fmt.Errorf("commit store: create directory: %w", s.sanitizePathError(err))
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("commit store: open file: %w", s.sanitizePathError(err))
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, entry := range entries {
		line := jsonEntry{
			SHA:     entry.SHA(),
			Message: entry.Message(),
			Author:  entry.Author(),
			Date:    entry.Date(),
		}
		data, err := json.Marshal(line)
		if err != nil {
			return fmt.Errorf("commit store: marshal entry: %w", err)
		}
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("commit store: write entry: %w", s.sanitizePathError(err))
		}
		if _, err := writer.WriteRune('\n'); err != nil {
			return fmt.Errorf("commit store: write newline: %w", s.sanitizePathError(err))
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("commit store: flush: %w", s.sanitizePathError(err))
	}

	return nil
}

// Reconcile reconciles the store's entries with the current git log.
// Stale entries (not in gitEntries) are removed, missing ones are added,
// and modified entries are updated. The final store state will match gitEntries.
// If the new state matches the existing file contents exactly, the write is skipped.
func (s *FilesystemCommitStore) Reconcile(gitEntries []domain.CommitEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.readLocked()
	if err != nil {
		return fmt.Errorf("commit store reconcile: read: %w", err)
	}

	if len(existing) == 0 && len(gitEntries) == 0 {
		if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
			return nil
		}
	}

	if s.entriesEqual(existing, gitEntries) {
		return nil
	}

	if err := s.write(gitEntries); err != nil {
		return fmt.Errorf("commit store reconcile: write: %w", err)
	}

	return nil
}

// entriesEqual checks if two slices of CommitEntry are identical in contents and order.
func (s *FilesystemCommitStore) entriesEqual(a, b []domain.CommitEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].SHA() != b[i].SHA() ||
			a[i].Message() != b[i].Message() ||
			a[i].Author() != b[i].Author() ||
			a[i].Date() != b[i].Date() {
			return false
		}
	}
	return true
}

// Clear truncates the file to zero bytes.
// If the file does not exist, returns no error.
func (s *FilesystemCommitStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If file doesn't exist, nothing to clear
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("commit store: clear: %w", s.sanitizePathError(err))
	}
	defer f.Close()

	return nil
}

// SetBranch switches the store to read/write from a branch-scoped path:
//   .git-courer/branches/<sanitized>/commits.json
// If name is empty, returns an error.
// After calling SetBranch, Append/Read/Clear operate on the branch path.
// Thread-safe: serialized by the adapter's mutex.
func (s *FilesystemCommitStore) SetBranch(name string) error {
	if name == "" {
		return fmt.Errorf("SetBranch: branch name must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sanitized := sanitizeBranchName(name)
	s.currentDir = filepath.Join(s.baseDir, "branches", sanitized)
	s.path = filepath.Join(s.currentDir, "commits.json")
	s.branch = name
	return nil
}

// RemoveBranch removes the branch's store directory and all contents:
//   .git-courer/branches/<sanitized>/
// If the directory does not exist, returns nil (idempotent).
// If name is empty, returns an error.
// Thread-safe: serialized by the adapter's mutex.
func (s *FilesystemCommitStore) RemoveBranch(name string) error {
	if name == "" {
		return fmt.Errorf("RemoveBranch: branch name must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sanitized := sanitizeBranchName(name)
	branchDir := filepath.Join(s.baseDir, "branches", sanitized)
	if _, err := os.Stat(branchDir); os.IsNotExist(err) {
		return nil // idempotent
	}
	return os.RemoveAll(branchDir)
}

// sanitizePathError replaces file paths in errors with a generic placeholder
// to prevent leaking filesystem details past the adapter boundary.
func (s *FilesystemCommitStore) sanitizePathError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	msg = filepath.Clean(msg)
	// Replace paths from most specific to least specific to avoid partial matches
	msg = strings.ReplaceAll(msg, s.path, "<commit-store>")
	msg = strings.ReplaceAll(msg, s.currentDir, "<commit-store-dir>")
	msg = strings.ReplaceAll(msg, s.baseDir, "<commit-store-base>")
	return errors.New(msg)
}