package sessionstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSSessionStore_SaveGetDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewFSSessionStore(dir)

	sess := &domain.Session{
		ID:         "abc12345",
		Agent:      "claude",
		Goal:       "fix bug",
		Branch:     "courer/session-abc12345",
		Worktree:   "../git-courer-worktrees/abc12345",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		BaseBranch: "main",
		Status:     domain.SessionActive,
	}

	require.NoError(t, store.Save(sess))

	// File must exist at {dir}/{id}.json
	_, err := os.Stat(filepath.Join(dir, "abc12345.json"))
	require.NoError(t, err)

	got, err := store.Get("abc12345")
	require.NoError(t, err)
	assert.Equal(t, sess.ID, got.ID)
	assert.Equal(t, sess.Agent, got.Agent)
	assert.Equal(t, sess.Branch, got.Branch)
	assert.Equal(t, domain.SessionActive, got.Status)

	require.NoError(t, store.Delete("abc12345"))
	_, err = store.Get("abc12345")
	assert.Error(t, err, "deleted session should not be found")
}

func TestFSSessionStore_GetMissing(t *testing.T) {
	dir := t.TempDir()
	store := NewFSSessionStore(dir)

	_, err := store.Get("nope")
	assert.Error(t, err)
}

func TestFSSessionStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewFSSessionStore(dir)

	require.NoError(t, store.Save(&domain.Session{ID: "a1", Status: domain.SessionActive}))
	require.NoError(t, store.Save(&domain.Session{ID: "b2", Status: domain.SessionFinished}))

	sessions, err := store.List()
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestFSSessionStore_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewFSSessionStore(dir)

	sessions, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestFSSessionStore_SaveRequiresID(t *testing.T) {
	dir := t.TempDir()
	store := NewFSSessionStore(dir)

	err := store.Save(&domain.Session{ID: ""})
	assert.Error(t, err)
}

func TestFSSessionStore_DeleteMissingIsNotError(t *testing.T) {
	dir := t.TempDir()
	store := NewFSSessionStore(dir)

	err := store.Delete("never-existed")
	assert.NoError(t, err, "deleting a missing session should not error")
}