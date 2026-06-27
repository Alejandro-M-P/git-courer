package domain

// SessionStatus represents the lifecycle state of a session.
type SessionStatus string

const (
	SessionActive        SessionStatus = "active"
	SessionFinished      SessionStatus = "finished"
	SessionCleanupFailed SessionStatus = "cleanup_failed"
)

// Session represents a started session persisted to metadata.
// This is the canonical domain model for finish/status/discard; the
// session package's SessionMeta is the start-command-specific record.
type Session struct {
	ID         string        `json:"id"`
	Agent      string        `json:"agent"`
	Goal       string        `json:"goal"`
	Branch     string        `json:"branch"`
	Worktree   string        `json:"worktree"`
	BaseCommit string        `json:"base_commit"`
	BaseBranch string        `json:"base_branch"`
	CreatedAt  string        `json:"created_at"`
	FinishedAt string        `json:"finished_at,omitempty"`
	Status     SessionStatus `json:"status"`
}