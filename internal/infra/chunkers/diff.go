// Package chunkers provides chunking strategies for various git outputs.
package chunkers

import (
	"regexp"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/infra/filters"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// DiffChunker splits unified diffs into logical chunks using semantic relationship clustering.
type DiffChunker struct {
	maxFilesPerChunk int
	minForce         int
	chunkSize        int
	catalog          *LanguageCatalog
	unifiedPass      *UnifiedASTPass
}

// Option configures a DiffChunker.
type Option func(*DiffChunker)

// WithMaxFilesPerChunk sets the maximum files per chunk.
func WithMaxFilesPerChunk(n int) Option {
	return func(c *DiffChunker) { c.maxFilesPerChunk = n }
}

// WithMinForce sets the minimum semantic force for graph edges.
func WithMinForce(n int) Option {
	return func(c *DiffChunker) { c.minForce = n }
}

// WithChunkSize sets the default chunk size fallback.
func WithChunkSize(n int) Option {
	return func(c *DiffChunker) { c.chunkSize = n }
}

// WithLanguageCatalog sets a custom language catalog.
func WithLanguageCatalog(catalog *LanguageCatalog) Option {
	return func(c *DiffChunker) { c.catalog = catalog }
}

// GetLanguageCatalog returns the language catalog used by this chunker.
func (c *DiffChunker) GetLanguageCatalog() *LanguageCatalog {
	return c.catalog
}

// NewDiffChunker creates a new DiffChunker.
func NewDiffChunker(opts ...Option) *DiffChunker {
	catalog := NewLanguageCatalog()
	c := &DiffChunker{
		maxFilesPerChunk: 12,
		minForce:         2,
		chunkSize:        0,
		catalog:          catalog,
		unifiedPass:      NewUnifiedASTPass(catalog),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Chunk splits a unified diff into logical chunks.
func (c *DiffChunker) Chunk(diff string, maxChunkSize int) ([]domain.DiffChunk, error) {
	if diff == "" {
		return nil, nil
	}

	if maxChunkSize == 0 && c.chunkSize > 0 {
		maxChunkSize = c.chunkSize
	}

	parsedFiles, _, err := gitdiff.Parse(strings.NewReader(diff))
	
	// If parsing succeeded but no fragments were found, the diff might be malformed
	// (e.g. missing @@ headers like in some tests). We must use fallback.
	hasFragments := false
	for _, f := range parsedFiles {
		if len(f.TextFragments) > 0 {
			hasFragments = true
			break
		}
	}
	
	if err != nil || (!hasFragments && strings.Contains(diff, "diff --git")) {
		chunks := c.fallbackChunk(diff, maxChunkSize)
		for i := range chunks {
			chunks[i].Diff = filters.FilterDiffNoise(chunks[i].Diff)
		}
		return chunks, nil
	}

	// 1. Unified Pass: Extract symbols, build semantic clusters, and generate chunks
	chunks, _, err := c.unifiedPass.Process(parsedFiles, maxChunkSize)
	if err != nil {
		return nil, err
	}

	// For now, UnifiedASTPass returns basic chunks. 
	// Future: use the extracted symbols to run the graph clustering.
	
	for i := range chunks {
		chunks[i].Diff = filters.FilterDiffNoise(chunks[i].Diff)
	}

	return chunks, nil
}

type fileInfo struct {
	name string
	diff string
	size int
}

func (c *DiffChunker) extractAllFileDiffs(files []*gitdiff.File, fullDiff string) []fileInfo {
	var result []fileInfo
	seen := make(map[string]bool)

	for _, f := range files {
		name := c.getFileName(f)
		if name == "" || seen[name] || f.IsBinary {
			continue
		}
		seen[name] = true

		fileDiff := c.extractFileDiff(fullDiff, f)
		if fileDiff == "" {
			continue
		}

		result = append(result, fileInfo{
			name: name,
			diff: fileDiff,
			size: len(fileDiff),
		})
	}
	return result
}

func (c *DiffChunker) fallbackChunk(diff string, maxChunkSize int) []domain.DiffChunk {
	if maxChunkSize <= 0 {
		maxChunkSize = 4000
	}

	var chunks []domain.DiffChunk
	lines := strings.Split(diff, "\n")
	var current strings.Builder
	currentSize := 0
	var currentFiles []string
	fileRe := regexp.MustCompile(`^diff --git a/(.*) b/.*`)

	for _, line := range lines {
		if m := fileRe.FindStringSubmatch(line); len(m) > 1 {
			currentFiles = append(currentFiles, m[1])
		}

		if currentSize+len(line) > maxChunkSize && current.Len() > 0 {
			chunks = append(chunks, domain.DiffChunk{Files: currentFiles, Diff: current.String()})
			current.Reset()
			currentFiles = nil
			currentSize = 0
		}

		current.WriteString(line + "\n")
		currentSize += len(line) + 1
	}

	if current.Len() > 0 {
		chunks = append(chunks, domain.DiffChunk{Files: currentFiles, Diff: current.String()})
	}

	return chunks
}

func (c *DiffChunker) extractFileDiff(fullDiff string, file *gitdiff.File) string {
	fileName := c.getFileName(file)
	lines := strings.Split(fullDiff, "\n")
	var result []string
	inFile := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			if strings.Contains(line, " a/"+fileName) || strings.Contains(line, " b/"+fileName) {
				inFile = true
				result = append(result, line)
			} else if inFile {
				break
			}
		} else if inFile {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func (c *DiffChunker) getFileName(f *gitdiff.File) string {
	if f.NewName != "" {
		return f.NewName
	}
	return f.OldName
}
