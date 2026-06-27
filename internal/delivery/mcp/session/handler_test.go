package session

import (
	"testing"

	"github.com/blak0p/git-courer/internal/core/ports"
)

// TestNewHandler_Struct mirrors branch/handler_test.go: NewHandler returns a
// non-nil Handler backed by the provided ports.Git. Structural test — the
// Handler has no behavior of its own; behavior is verified via HandleSession.
func TestNewHandler_Struct(t *testing.T) {
	mockGit := new(MockGit)
	h := NewHandler(mockGit)

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	// Handler must carry the injected git port so HandleSession can use it.
	// We assert the type implements ports.Git to lock the constructor contract.
	var _ *Handler = h
	_ = ports.Git(mockGit) // compile-time: MockGit satisfies ports.Git
}
