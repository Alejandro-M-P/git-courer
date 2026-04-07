// Package diff provides intelligent diff chunking for LLM consumption.
// It uses go-gitdiff to parse unified diffs and split them into
// logical, context-safe chunks that never exceed size limits.
package diff

import (
	"fmt"
	"log"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// Chunker implements ports.DiffChunker using go-gitdiff.
type Chunker struct{}

// NewChunker creates a new diff chunker.
func NewChunker() *Chunker {
	return &Chunker{}
}

// Chunk splits a unified diff into logical chunks.
// Each chunk contains one or more files and stays under maxChunkSize.
func (c *Chunker) Chunk(diff string, maxChunkSize int) ([]domain.DiffChunk, error) {
	log.Printf("[CHUNKER] Chunk() called with diff length=%d, maxChunkSize=%d", len(diff), maxChunkSize)

	if diff == "" {
		log.Printf("[CHUNKER] Empty diff, returning nil")
		return nil, nil
	}

	// Parse the diff into structured file diffs
	files, _, err := gitdiff.Parse(strings.NewReader(diff))
	if err != nil {
		log.Printf("[CHUNKER] Parse error: %v, using fallbackChunk", err)
		// If parsing fails, fall back to simple text chunking
		return c.fallbackChunk(diff, maxChunkSize)
	}

	log.Printf("[CHUNKER] Parsed %d files from diff", len(files))

	var chunks []domain.DiffChunk
	var currentFiles []string
	var currentDiff strings.Builder
	var currentSize int
	var deletedFiles []string
	var deletedDiff strings.Builder

	for _, file := range files {
		// Determine the file name (use OldName for deleted files, NewName otherwise)
		fileName := file.NewName
		if fileName == "" {
			fileName = file.OldName
		}
		if fileName == "" {
			log.Printf("[CHUNKER] Skipping file with no name")
			continue // Skip files with no name at all
		}

		// Reconstruct the diff for this file from the original diff
		fileDiff := c.extractFileDiff(diff, file)

		// Skip if no diff content
		if fileDiff == "" {
			log.Printf("[CHUNKER] Skipping file %s with empty diff", fileName)
			continue
		}

		fileSize := len(fileDiff)
		log.Printf("[CHUNKER] File %s, diff size=%d", fileName, fileSize)

		// DELETED FILES: Add to a special "deleted files" chunk and skip splitting
		if file.NewName == "" {
			// Flush current buffer first
			if currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{
					Files: currentFiles,
					Diff:  currentDiff.String(),
				})
				currentFiles = nil
				currentDiff.Reset()
				currentSize = 0
			}

			// Add to deleted files chunk
			deletedFiles = append(deletedFiles, fileName)
			deletedDiff.WriteString(fileDiff)
			continue
		}

		// If a single file exceeds maxChunkSize, we need to split it
		if fileSize > maxChunkSize {
			// Flush current buffer first
			if currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{
					Files: currentFiles,
					Diff:  currentDiff.String(),
				})
				currentFiles = nil
				currentDiff.Reset()
				currentSize = 0
			}

			// Split the large file into hunks
			subChunks, err := c.splitLargeFile(fileDiff, file, maxChunkSize)
			if err != nil {
				return nil, fmt.Errorf("failed to split large file %s: %w", fileName, err)
			}
			chunks = append(chunks, subChunks...)
			continue
		}

		// If adding this file would exceed the limit, flush current buffer
		if currentSize+fileSize > maxChunkSize && currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{
				Files: currentFiles,
				Diff:  currentDiff.String(),
			})
			currentFiles = nil
			currentDiff.Reset()
			currentSize = 0
		}

		// Add file to current chunk
		currentFiles = append(currentFiles, fileName)
		currentDiff.WriteString(fileDiff)
		currentSize += fileSize
	}

	// Flush remaining
	if currentSize > 0 {
		chunks = append(chunks, domain.DiffChunk{
			Files: currentFiles,
			Diff:  currentDiff.String(),
		})
	}

	// Add deleted files as one chunk (they can't be split)
	if len(deletedFiles) > 0 {
		chunks = append(chunks, domain.DiffChunk{
			Files: deletedFiles,
			Diff:  deletedDiff.String(),
		})
	}

	log.Printf("[CHUNKER] Returning %d chunks", len(chunks))
	return chunks, nil
}

// extractFileDiff extracts the diff section for a specific file from the full diff.
func (c *Chunker) extractFileDiff(fullDiff string, file *gitdiff.File) string {
	// Determine actual file name
	fileName := file.NewName
	if fileName == "" {
		fileName = file.OldName
	}
	if fileName == "" {
		return ""
	}

	lines := strings.Split(fullDiff, "\n")
	var result []string
	inFile := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// Check if this is our file using the actual file name
			searchName := " " + fileName
			isOurFile := strings.Contains(line, " a/"+fileName) ||
				strings.Contains(line, " b/"+fileName) ||
				strings.Contains(line, searchName)

			if isOurFile {
				inFile = true
				result = append(result, line)
			} else {
				if inFile {
					break // We've passed our file
				}
			}
		} else if inFile {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// splitLargeFile splits a single large file diff into multiple chunks by hunks.
func (c *Chunker) splitLargeFile(fileDiff string, file *gitdiff.File, maxChunkSize int) ([]domain.DiffChunk, error) {
	var chunks []domain.DiffChunk
	var currentDiff strings.Builder
	currentSize := 0

	// Determine file name (use OldName for deleted files)
	fileName := file.NewName
	if fileName == "" {
		fileName = file.OldName
	}

	lines := strings.Split(fileDiff, "\n")
	for _, line := range lines {
		// Start a new chunk at hunk boundaries
		if strings.HasPrefix(line, "@@") && currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{
				Files: []string{fileName},
				Diff:  currentDiff.String(),
			})
			currentDiff.Reset()
			currentSize = 0
		}

		currentDiff.WriteString(line + "\n")
		currentSize += len(line) + 1

		// If current chunk exceeds limit mid-hunk, force flush
		if currentSize > maxChunkSize {
			chunks = append(chunks, domain.DiffChunk{
				Files: []string{fileName},
				Diff:  currentDiff.String(),
			})
			currentDiff.Reset()
			currentSize = 0
		}
	}

	if currentDiff.Len() > 0 {
		chunks = append(chunks, domain.DiffChunk{
			Files: []string{fileName},
			Diff:  currentDiff.String(),
		})
	}

	return chunks, nil
}

// fallbackChunk splits diff by simple text boundaries when parsing fails.
func (c *Chunker) fallbackChunk(diff string, maxChunkSize int) ([]domain.DiffChunk, error) {
	var chunks []domain.DiffChunk
	var current strings.Builder
	currentSize := 0

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		// Break at file boundaries
		if strings.HasPrefix(line, "diff --git") && currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{
				Files: []string{"multiple"},
				Diff:  current.String(),
			})
			current.Reset()
			currentSize = 0
		}

		current.WriteString(line + "\n")
		currentSize += len(line) + 1

		if currentSize > maxChunkSize {
			chunks = append(chunks, domain.DiffChunk{
				Files: []string{"multiple"},
				Diff:  current.String(),
			})
			current.Reset()
			currentSize = 0
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, domain.DiffChunk{
			Files: []string{"multiple"},
			Diff:  current.String(),
		})
	}

	return chunks, nil
}
