package ports

// OperationPlan represents any pending git operation waiting for user confirmation.
// Commit-specific fields (Messages, Files, RejectedMessage) are populated only for COMMIT.
type OperationPlan struct {
	Operation       string            `json:"operation"`
	Args            map[string]string `json:"args"`
	Preview         string            `json:"preview"`
	CreatedAt       int64             `json:"created_at"`
	// Commit-specific fields (omitted for non-commit operations)
	Messages        []string `json:"messages,omitempty"`
	Files           []string `json:"files,omitempty"`
	RejectedMessage string   `json:"rejected_message,omitempty"`
}

// ConfirmPort handles the plan/blocker lifecycle for any operation requiring confirmation.
// Shared by all operations listed under preview.operations in config.
type ConfirmPort interface {
	WritePlan(plan OperationPlan) error
	ReadPlan() (*OperationPlan, error)
	DeletePlan() error
	CreateBlocker() error
	HasBlocker() bool
	RemoveBlocker() error
	IsPlanExpired() bool
}
