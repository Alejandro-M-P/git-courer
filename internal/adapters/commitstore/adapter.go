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
	mu   sync.Mutex
	dir  string
	path string
}

// NewFilesystemCommitStore creates a FilesystemCommitStore that persists
// entries in workDir/.git-courer/commits.json.
// The directory and file are created lazily on first Append.
func NewFilesystemCommitStore(workDir string) *FilesystemCommitStore {
	dir := filepath.Join(workDir, ".git-courer")
	path := filepath.Join(dir, "commits.json")
	return &FilesystemCommitStore{
		dir:  dir,
		path: path,
	}
}

// Append adds one or more CommitEntry values to the store.
// Each entry is written as a single JSON line appended to the file.
// The directory is created lazily if it does not exist.
func (s *FilesystemCommitStore) Append(entries ...domain.CommitEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
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

// sanitizePathError replaces file paths in errors with a generic placeholder
// to prevent leaking filesystem details past the adapter boundary.
func (s *FilesystemCommitStore) sanitizePathError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	msg = filepath.Clean(msg)
	// Replace the internal path with a generic marker
	msg = strings.ReplaceAll(msg, s.path, "<commit-store>")
	msg = strings.ReplaceAll(msg, s.dir, "<commit-store-dir>")
	return errors.New(msg)
}