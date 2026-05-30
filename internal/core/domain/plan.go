package domain

// OperationPlan represents a pending git operation awaiting user confirmation.
// Persisted to disk and read back when the user approves via *_APPLY.
type OperationPlan struct {
	Operation string            `json:"operation"`
	Args      map[string]string `json:"args"`
	Preview   string            `json:"preview"` // Human-readable description shown to the user
	CreatedAt int64             `json:"created_at"`

	// Operation metadata
	Messages    []string `json:"messages,omitempty"`
	Files       []string `json:"files,omitempty"`
	Reasoning   string   `json:"reasoning,omitempty"`   // Why these changes/files were chosen
	Instruction string   `json:"instruction,omitempty"` // Original user instruction

	// Commit-specific fields
	Chunks          [][]string `json:"chunks,omitempty"`        // Per-message file lists for per-chunk staging
	DeletedFiles    []string   `json:"deleted_files,omitempty"` // Files with status "D " to commit separately
	RejectedMessage string     `json:"rejected_message,omitempty"`
	DiffHash        string     `json:"diff_hash,omitempty"` // Fingerprint of the diff at START time
	Backup          Backup     `json:"backup,omitempty"`    // Backup information for rollback
}
