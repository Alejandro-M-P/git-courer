package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// GitReadPort for read-only git operations
type GitReadPort interface {
	Status() (domain.Status, error)
	Diff() (string, error)
	DiffStaged() (string, error)
	Log(limit int) (string, error)
	CurrentBranch() (string, error)
}
