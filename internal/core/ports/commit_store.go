package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// CommitStore is the port for persisting commit metadata across
// the release cycle. Append stores entries as they are created,
// Read returns all entries from the current cycle (Clear runs
// after every release, so all entries belong to the current cycle),
// and Clear empties the store after a successful release.
type CommitStore interface {
	// Append adds one or more CommitEntry values to the store.
	Append(entries ...domain.CommitEntry) error

	// Read returns all stored CommitEntry values.
	// If no data exists (missing file or empty), returns an empty slice with no error.
	Read() ([]domain.CommitEntry, error)

	// Clear removes all stored entries.
	// If the store is already empty, returns no error.
	Clear() error
}