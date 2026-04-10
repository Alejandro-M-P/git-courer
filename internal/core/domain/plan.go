package domain

// OperationPlan represents a pending git operation awaiting user confirmation.
// Persisted to disk and read back when the user approves via *_APPLY.
type OperationPlan struct {
	Operation string            `json:"operation"`
	Args      map[string]string `json:"args"`
	Preview   string            `json:"preview"` // Human-readable description shown to the user
	CreatedAt int64             `json:"created_at"`

	// Commit-specific fields — populated only for the "commit" operation.
	Messages        []string `json:"messages,omitempty"`
	Files           []string `json:"files,omitempty"`
	RejectedMessage string   `json:"rejected_message,omitempty"`
	Reasoning       string   `json:"reasoning,omitempty"`   // Why these files were chosen
	Instruction     string   `json:"instruction,omitempty"` // Original user instruction
}
