package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// ChunkAnnotator adds semantic AST labels to a DiffChunk.
type ChunkAnnotator interface {
	// Annotate analyses before/after content and populates chunk.AnnotatedDiff and chunk.CommitType.
	Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error
}
