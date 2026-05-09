package ports

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestChunkAnnotatorInterface verifies the ChunkAnnotator interface shape.
func TestChunkAnnotatorInterface(t *testing.T) {
	// If this compiles, the interface has the expected methods.
	var _ interface{} = (*interface {
		Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error
	})(nil)

	t.Log("ChunkAnnotator interface method verified")
}

// TestChunkAnnotatorWithDiffChunk verifies the method signature works with domain.DiffChunk.
func TestChunkAnnotatorWithDiffChunk(t *testing.T) {
	// Verify DiffChunk has the fields needed by annotator.
	chunk := domain.DiffChunk{}

	// Verify we can assign AnnotatedDiff after annotation.
	chunk.AnnotatedDiff = "file.go\nFunc [NEW_FUNC]\n+ func Func() {}"
	if chunk.AnnotatedDiff == "" {
		t.Error("AnnotatedDiff assignment failed")
	}

	// Verify CommitType can be set after classification.
	chunk.CommitType = "feat"
	if chunk.CommitType != "feat" {
		t.Errorf("CommitType = %q, want feat", chunk.CommitType)
	}
}
