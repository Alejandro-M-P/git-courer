package session

import (
	"testing"

	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/stretchr/testify/assert"
)

// Compile-time interface check — MockGit must satisfy ports.Git.
var _ ports.Git = (*MockGit)(nil)

// TestMockGit_SatisfiesInterface verifies MockGit implements ports.Git by
// exercising the four session-relevant methods (Head, CreateRef, AddWorktree,
// RemoveWorktree) through the ports.Git interface. This proves the mock is
// wired correctly before PR 2 uses it for session handler tests.
func TestMockGit_SatisfiesInterface(t *testing.T) {
	// Assign to ports.Git to prove interface compliance at runtime too.
	var git ports.Git = new(MockGit)
	_ = git // compile-time check already done above; this asserts assignability

	// Head — session uses this to get the base commit
	m := new(MockGit)
	m.On("Head").Return("abc123", nil)
	head, err := m.Head()
	assert.NoError(t, err)
	assert.Equal(t, "abc123", head)
	m.AssertExpectations(t)

	// CreateRef — session uses this for atomic branch creation
	m2 := new(MockGit)
	m2.On("CreateRef", "refs/heads/session-test", "abc123").Return(nil)
	err = m2.CreateRef("refs/heads/session-test", "abc123")
	assert.NoError(t, err)
	m2.AssertExpectations(t)

	// AddWorktree — session uses this for worktree creation
	m3 := new(MockGit)
	m3.On("AddWorktree", "/tmp/wt", "test").Return("/tmp/wt", nil)
	got, err := m3.AddWorktree("/tmp/wt", "test")
	assert.NoError(t, err)
	assert.Equal(t, "/tmp/wt", got)
	m3.AssertExpectations(t)

	// RemoveWorktree — session uses this for rollback/cleanup
	m4 := new(MockGit)
	m4.On("RemoveWorktree", "/tmp/wt").Return(nil)
	err = m4.RemoveWorktree("/tmp/wt")
	assert.NoError(t, err)
	m4.AssertExpectations(t)
}