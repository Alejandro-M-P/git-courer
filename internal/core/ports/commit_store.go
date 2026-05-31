package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// CommitStore is the port for persisting commit metadata across
// the release cycle. Append stores entries as they are created,
// Read returns all entries from the current cycle (Clear runs
// after every release, so all entries belong to the current cycle),
// and Clear empties the store after a successful release.
//
// SetBranch switches the store to a branch-scoped path:
//   .git-courer/branches/<sanitized>/commits.json
// If SetBranch is never called, the store uses the legacy global
// path .git-courer/commits.json.
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

	// SetBranch switches the store to read/write from a branch-scoped path:
	//   .git-courer/branches/<sanitized>/commits.json
	// If name is empty, returns an error.
	// After calling SetBranch, Append/Read/Clear operate on the branch path.
	// Thread-safe: serialized by the adapter's mutex.
	SetBranch(name string) error

	// RemoveBranch removes the branch's store directory and all contents:
	//   .git-courer/branches/<sanitized>/
	// If the directory does not exist, returns nil (idempotent).
	// If name is empty, returns an error.
	// Thread-safe: serialized by the adapter's mutex.
	RemoveBranch(name string) error

	// Reconcile reconciles the store's entries with the current git log.
	// Stale entries (not in gitEntries) are removed, missing ones are added,
	// and modified entries are updated. The final store state will match gitEntries.
	// If the new state matches the existing file contents exactly, the write is skipped.
	Reconcile(gitEntries []domain.CommitEntry) error
}