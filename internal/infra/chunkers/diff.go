// Package chunkers provides chunking strategies for various git outputs.
package chunkers

import (
	"regexp"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// LanguageGrammar defines how to find symbols in a specific language.
type LanguageGrammar struct {
	DefRegex *regexp.Regexp // Matches function/class/interface definitions
	RefRegex *regexp.Regexp // Matches function calls or usages
}

// grammars maps file extensions to their semantic rules.
var grammars = map[string]LanguageGrammar{
	".py": {
		DefRegex: regexp.MustCompile(`(?m)^\s*(?:def|class)\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		RefRegex: regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\(`),
	},
	".js": {
		DefRegex: regexp.MustCompile(`(?m)\b(?:function|class|const|let|var)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:=|=>|\()`),
		RefRegex: regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\(`),
	},
	".ts": {
		DefRegex: regexp.MustCompile(`(?m)\b(?:function|class|interface|type|const|let|var)\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		RefRegex: regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\(`),
	},
	".rs": {
		DefRegex: regexp.MustCompile(`(?m)\b(?:fn|struct|enum|trait|type)\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		RefRegex: regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\(`),
	},
	".cpp": {
		DefRegex: regexp.MustCompile(`(?m)\b(?:class|struct|void|int|float|double|auto)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
		RefRegex: regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\(`),
	},
	".java": {
		DefRegex: regexp.MustCompile(`(?m)\b(?:class|interface|enum|void|public|private|protected)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\{|\()`),
		RefRegex: regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\(`),
	},
}

// DiffChunker splits unified diffs into logical chunks using semantic relationship clustering.
type DiffChunker struct {
	maxFilesPerChunk int
	minForce         int
}

// NewDiffChunker creates a new DiffChunker.
func NewDiffChunker() *DiffChunker {
	return &DiffChunker{
		maxFilesPerChunk: 5,
		minForce:         2,
	}
}

// FileSymbols represents semantic information for a file.
type FileSymbols struct {
	Definitions map[string]bool
	References  map[string]bool
}

// Chunk splits a unified diff into logical chunks.
func (c *DiffChunker) Chunk(diff string, maxChunkSize int) ([]domain.DiffChunk, error) {
	if diff == "" {
		return nil, nil
	}

	files, _, err := gitdiff.Parse(strings.NewReader(diff))
	if err != nil {
		return c.fallbackChunk(diff, maxChunkSize), nil
	}

	fileDiffs := c.extractAllFileDiffs(files, diff)
	if len(fileDiffs) == 0 {
		return nil, nil
	}

	// 1. Semantic layer: Extract definitions and references
	symbols := c.extractAllSymbols(fileDiffs)

	// 2. Graph layer: Build relationship graph based on semantics
	graph := c.buildGraph(fileDiffs, symbols)

	// 3. Cluster layer: Group related files
	prunedGraph := c.pruneGraph(graph)
	clusters := c.createClusters(prunedGraph, fileDiffs)

	// 4. Chunk layer: Physical partitioning
	chunks := c.buildChunks(clusters, fileDiffs, maxChunkSize)

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

func (c *DiffChunker) getFileName(f *gitdiff.File) string {
	if f.NewName != "" {
		return f.NewName
	}
	return f.OldName
}