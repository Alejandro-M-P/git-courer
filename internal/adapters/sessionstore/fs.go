// Package sessionstore provides a file-based SessionStore implementation.
// Each session is persisted as <baseDir>/{id}.json.
package sessionstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// FSSessionStore is a file-based SessionStore. Sessions are stored as JSON
// files named {id}.json under baseDir.
type FSSessionStore struct {
	baseDir string
}

// NewFSSessionStore creates a SessionStore rooted at baseDir. The directory
// is created lazily on the first Save.
func NewFSSessionStore(baseDir string) *FSSessionStore {
	return &FSSessionStore{baseDir: baseDir}
}

// path returns the JSON file path for a session id.
func (s *FSSessionStore) path(id string) string {
	return filepath.Join(s.baseDir, id+".json")
}

// Get reads a session by id. Returns an error if the session file does not
// exist or cannot be parsed.
func (s *FSSessionStore) Get(id string) (*domain.Session, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %q not found", id)
		}
		return nil, fmt.Errorf("read session %q: %w", id, err)
	}
	var sess domain.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session %q: %w", id, err)
	}
	return &sess, nil
}

// Save writes the session to <baseDir>/{id}.json, creating baseDir if needed.
func (s *FSSessionStore) Save(session *domain.Session) error {
	if session == nil || session.ID == "" {
		return fmt.Errorf("save session: id is required")
	}
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(s.path(session.ID), data, 0o644); err != nil {
		return fmt.Errorf("write session %q: %w", session.ID, err)
	}
	return nil
}

// Delete removes the session file for id. A missing file is not an error.
func (s *FSSessionStore) Delete(id string) error {
	err := os.Remove(s.path(id))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete session %q: %w", id, err)
	}
	return nil
}

// List reads all .json session files in baseDir. Returns an empty slice when
// the directory does not exist or is empty.
func (s *FSSessionStore) List() ([]*domain.Session, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Session{}, nil
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	var sessions []*domain.Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(s.baseDir, entry.Name()))
		if rerr != nil {
			continue
		}
		var sess domain.Session
		if perr := json.Unmarshal(data, &sess); perr != nil {
			continue
		}
		sessions = append(sessions, &sess)
	}
	return sessions, nil
}