package ports

// GitWriteReviewPort for write operations that require user confirmation
type GitWriteReviewPort interface {
	CreateBranch(name string) (string, error)
	DeleteBranch(name string) (string, error)
	RenameBranch(oldName, newName string) (string, error)
	CreateTag(name string) (string, error)
	DeleteTag(name string) (string, error)
	Merge(branch string) (string, error)
	Rebase(branch string) (string, error)
	RebaseContinue() (string, error)
	RebaseAbort() (string, error)
	ResetSoft(commits int) error
	ResetHard(target string) (string, error)
	Clean() (string, error)
	AddRemote(name, url string) (string, error)
	RemoveRemote(name string) (string, error)
	CherryPick(commit string) (string, error)
	Revert(commit string) (string, error)
	Init() (string, error)
	Clone(url string) (string, error)
}
