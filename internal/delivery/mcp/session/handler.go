package session

import "github.com/blak0p/git-courer/internal/core/ports"

// Handler holds dependencies for the session MCP tool. It mirrors
// internal/delivery/mcp/branch/handler.go: a thin struct wrapping the git port.
// Behavior lives in session.go (HandleSession dispatch + handleStart).
type Handler struct {
	git     ports.Git
	metaDir string // base directory for session metadata files; overridable for tests
}

// NewHandler creates a new session Handler backed by the given git port.
// The metadata directory defaults to .git/git-courer/sessions (see
// domain.MetadataDir). Tests may override it via WithMetaDir.
func NewHandler(git ports.Git) *Handler {
	return &Handler{
		git:     git,
		metaDir: defaultSessionMetaDir,
	}
}
