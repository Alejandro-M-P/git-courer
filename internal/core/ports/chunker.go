package ports

import "github.com/blak0p/git-courer/internal/core/domain"

// DiffChunker splits a large unified diff into smaller, LLM-friendly chunks.
type DiffChunker interface {
	// Chunk splits a unified diff into logical chunks.
	// Each chunk is small enough to fit in an LLM context window.
	// maxChunkSize is the maximum size in characters per chunk.
	Chunk(diff string, maxChunkSize int) ([]domain.DiffChunk, error)
}
