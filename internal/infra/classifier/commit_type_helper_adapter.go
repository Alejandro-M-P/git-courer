package classifier

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// CommitTypeHelperAdapter wraps pure domain functions to implement ports.CommitTypeHelper.
// This is the adapter that connects the domain-level commit type inference
// to the port interface used by the workflow layer.
type CommitTypeHelperAdapter struct{}

// NewCommitTypeHelperAdapter creates a new CommitTypeHelperAdapter.
func NewCommitTypeHelperAdapter() *CommitTypeHelperAdapter {
	return &CommitTypeHelperAdapter{}
}

// Compile-time interface check
var _ ports.CommitTypeHelper = (*CommitTypeHelperAdapter)(nil)

// InferCommitType delegates to domain.InferCommitType.
func (a *CommitTypeHelperAdapter) InferCommitType(chunk domain.DiffChunk) string {
	return domain.InferCommitType(chunk)
}

// CommitTypeWeight delegates to domain.CommitTypeWeight.
func (a *CommitTypeHelperAdapter) CommitTypeWeight(commitType string) int {
	return domain.CommitTypeWeight(commitType)
}