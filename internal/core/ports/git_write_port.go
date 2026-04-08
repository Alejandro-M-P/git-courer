package ports

// GitWritePort for direct write git operations (no preview needed)
type GitWritePort interface {
	Add(paths []string) error
	Checkout(branch string) error
	Switch(branch string) error
	Stash() error
	StashPop() error
	Push() (string, error)
	Pull() (string, error)
	Fetch() (string, error)
	Remove(paths []string) error
}
