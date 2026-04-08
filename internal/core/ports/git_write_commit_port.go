package ports

// GitWriteCommitPort for commit operations with preview mode
type GitWriteCommitPort interface {
	// Plan management
	WritePlan(plan CommitPlan) error
	ReadPlan() (*CommitPlan, error)
	IsPlanExpired() bool
	DeletePlan() error

	// Blocker management (for preview mode)
	CreateBlocker() error
	HasBlocker() bool
	RemoveBlocker() error

	// Lock management
	AcquireLock() error
	ReleaseLock() error
	IsLocked() bool

	// Blocking mechanism
	WaitForConfirmation() bool
	Approve()
	Abort()
	GetState() int

	// Commit execution (existing code, never modified)
	Execute(instruction string, preview bool) (string, error)
}

// CommitPlan represents a commit plan
type CommitPlan struct {
	Files           []string `json:"files"`
	Message         string   `json:"message"`          // Current/generated message
	RejectedMessage string   `json:"rejected_message"` // Previous message rejected by user
	Messages        []string `json:"messages"`         // Generated commit messages (one per chunk)
	SubCmd          string   `json:"subcmd"`
	Preview         bool     `json:"preview"`
	Commits         int      `json:"commits"`
	CreatedAt       int64    `json:"created_at"`
}

// Blocker management
type BlockerManager interface {
	CreateBlocker() error
	HasBlocker() bool
	RemoveBlocker() error
}
