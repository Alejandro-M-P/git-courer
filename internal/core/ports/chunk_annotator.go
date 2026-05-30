package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// ChunkAnnotator adds semantic AST labels to a DiffChunk.
type ChunkAnnotator interface {
	// Annotate analyses before/after content and populates chunk.AnnotatedDiff and chunk.CommitType.
	Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error

	// AnnotateWithContent enriches a chunk with AST-based semantic labels from
	// multiple file contents and merges diff annotations. It processes all files
	// in the content list, populates AnnotatedDiff, CFGBefore/CFGAfter, and
	// BeforeSource/AfterSource on the chunk.
	AnnotateWithContent(chunk *domain.DiffChunk, files []FileContent, rawDiff string) error
}
