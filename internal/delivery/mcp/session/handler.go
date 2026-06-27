package session

import "github.com/blak0p/git-courer/internal/core/ports"

// Handler holds dependencies for the session MCP tool. It mirrors
// internal/delivery/mcp/branch/handler.go: a thin struct wrapping the git port
// plus the session store and a workDir for preview/finish workflows.
// Behavior lives in session.go (HandleSession dispatch + handlers).
type Handler struct {
	git     ports.Git
	store   ports.SessionStore // nil in start-only contexts; finish/status/discard require it
	workDir string            // repo root for preview engine + config loading
	metaDir string            // base directory for session metadata files; overridable for tests
}

// NewHandler creates a new session Handler backed by the given git port.
// The metadata directory defaults to .git/git-courer/sessions (see
// domain.MetadataDir). Tests may override it via h.metaDir.
func NewHandler(git ports.Git) *Handler {
	return &Handler{
		git:     git,
		metaDir: defaultSessionMetaDir,
	}
}

// NewHandlerWithStore creates a session Handler with a SessionStore and workDir
// so finish/status/discard can load and persist canonical domain.Session
// records. start continues to write the SessionMeta JSON file for back-compat.
func NewHandlerWithStore(git ports.Git, store ports.SessionStore, workDir string) *Handler {
	return &Handler{
		git:     git,
		store:   store,
		workDir: workDir,
		metaDir: defaultSessionMetaDir,
	}
}