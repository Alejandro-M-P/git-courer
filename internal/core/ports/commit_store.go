package ports

import "github.com/blak0p/git-courer/internal/core/domain"

// CommitStore is the port for persisting commit metadata across
// the release cycle. Append stores entries as they are created,
// Read returns all entries from the current cycle (Clear runs
// after every release, so all entries belong to the current cycle),
// and Clear empties the store after a successful release.
//
// SetWorkspace switches the store to a workspace-scoped path:
//
//	.git/git-courer/workspace/<id>/commits.json
//
// where <id> is the active session ID (a UUID, passed unchanged) or,
// when no session is active, the current branch name (sanitized).
// SetBranchRef sets the branch used for the refs/courer/<branch> ref
// update; it is separate from SetWorkspace because ref naming must
// stay branch-keyed for push/pull while the workspace UUID is
// meaningless as a ref name.
//
// RemoveBranch deletes the branch's store directory and all contents.
type CommitStore interface {
	// Append adds one or more CommitEntry values to the store.
	Append(entries ...domain.CommitEntry) error

	// Read returns all stored CommitEntry values.
	// If no data exists (missing file or empty), returns an empty slice with no error.
	Read() ([]domain.CommitEntry, error)

	// Clear removes all stored entries.
	// If the store is already empty, returns no error.
	Clear() error

	// SetWorkspace switches the store to read/write from a workspace-scoped path:
	//   .git/git-courer/workspace/<id>/commits.json
	// The id is sanitized conditionally: a valid UUID passes unchanged; a non-UUID
	// (e.g. a branch name) is sanitized via the adapter's branch-name sanitizer.
	// If id is empty, returns an error.
	// After calling SetWorkspace, Append/Read/Clear operate on the workspace path.
	// Thread-safe: serialized by the adapter's mutex.
	SetWorkspace(workspaceID string) error

	// SetBranchRef sets the branch name used for the refs/courer/<branch> ref
	// update that accompanies Append. It is separate from SetWorkspace because
	// ref naming must stay branch-keyed for push/pull while the workspace UUID
	// is meaningless as a ref name. Empty branch disables the ref update.
	// Thread-safe: serialized by the adapter's mutex.
	SetBranchRef(branch string) error

	// RemoveBranch removes the branch's store directory and all contents:
	//   .git/git-courer/branches/<sanitized>/
	// If the directory does not exist, returns nil (idempotent).
	// If name is empty, returns an error.
	// Thread-safe: serialized by the adapter's mutex.
	//
	// DEPRECATED: branches/ storage is kept for passive coexistence only.
	// New captures write to workspace/ instead. Removal is a no-op for
	// workspace-keyed stores.
	RemoveBranch(name string) error

	// Reconcile reconciles the store's entries with the current git log.
	// Stale entries (not in gitEntries) are removed, missing ones are added,
	// and modified entries are updated. The final store state will match gitEntries.
	// If the new state matches the existing file contents exactly, the write is skipped.
	Reconcile(gitEntries []domain.CommitEntry) error

	// ReadAllBranches returns all stored CommitEntry values from all branch stores.
	// Returns a map of branch name → entries. If no branches exist, returns
	// an empty map with no error.
	//
	// DEPRECATED: branches/ storage is kept for passive coexistence only.
	// Use ReadAllWorkspaces for new workspace-keyed captures.
	ReadAllBranches() (map[string][]domain.CommitEntry, error)

	// RemoveAllBranchDirs removes all branch directories under .git/git-courer/branches/.
	// It is idempotent: if no directories exist, it returns nil.
	// It removes the entire branches/ directory tree, not individual branch directories.
	//
	// DEPRECATED: branches/ storage is kept for passive coexistence only.
	RemoveAllBranchDirs() error

	// ReadAllWorkspaces returns all stored CommitEntry values from all workspace stores.
	// Returns a map of workspace id → entries. If no workspaces exist, returns
	// an empty map with no error. Malformed directories are skipped with a log warning.
	ReadAllWorkspaces() (map[string][]domain.CommitEntry, error)

	// RemoveAllWorkspaceDirs removes all workspace directories under
	// .git/git-courer/workspace/. It is idempotent: if no directories exist,
	// it returns nil. It removes the entire workspace/ directory tree.
	RemoveAllWorkspaceDirs() error
}
