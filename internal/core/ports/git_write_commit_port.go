package ports

// GitWriteCommitPort for commit operations with preview mode
type GitWriteCommitPort interface {
	// Plan management
	WritePlan(plan CommitPlan) error
	ReadPlan() (*CommitPlan, error)
	IsPlanExpired() bool
	DeletePlan() error

	// Lock management
	AcquireLock() error
	ReleaseLock() error
	IsLocked() bool

	// Blocking mechanism
	WaitForConfirmation() bool
	Approve()
	Abort()

	// Commit execution (existing code, never modified)
	Execute(instruction string, preview bool) (string, error)
}

// CommitPlan represents a commit plan
type CommitPlan struct {
	Files     []string `json:"files"`
	Message   string   `json:"message"`
	SubCmd    string   `json:"subcmd"`
	Preview   bool     `json:"preview"`
	Commits   int      `json:"commits"`
	CreatedAt int64    `json:"created_at"`
}
