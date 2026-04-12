// Package chunkers provides chunking strategies for various git outputs.
package chunkers

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// DiffChunker splits unified diffs into logical chunks using token-based clustering.
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

// Chunk splits a unified diff into logical chunks using token-based clustering.
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

	tokens := c.extractTokens(fileDiffs)
	graph := c.buildGraph(tokens)
	prunedGraph := c.pruneGraph(graph)
	clusters := c.createClusters(prunedGraph, fileDiffs)
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
		if name == "" {
			continue
		}

		if seen[name] {
			continue
		}
		seen[name] = true

		if f.IsBinary || f.NewName == "" {
			continue
		}

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
	if f.OldName != "" {
		return f.OldName
	}
	return ""
}

func (c *DiffChunker) extractTokens(files []fileInfo) map[string][]string {
	tokens := make(map[string][]string)

	for _, f := range files {
		tokens[f.name] = c.extractTokensFromDiff(f.diff)
	}

	return tokens
}

func (c *DiffChunker) extractTokensFromDiff(diff string) []string {
	var result []string
	lines := strings.Split(diff, "\n")

	wordRegex := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)

	for _, line := range lines {
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			continue
		}

		words := wordRegex.FindAllStringSubmatch(line, -1)
		for _, word := range words {
			if len(word) > 2 && !isCommonWord(word[0]) {
				result = append(result, word[0])
			}
		}
	}

	return result
}

func isCommonWord(word string) bool {
	common := map[string]bool{
		"func": true, "type": true, "var": true, "const": true,
		"import": true, "return": true, "if": true, "else": true,
		"for": true, "range": true, "switch": true, "case": true,
		"default": true, "break": true, "continue": true,
		"true": true, "false": true, "nil": true,
		"package": true, "struct": true, "interface": true,
		"go": true, "defer": true, "select": true,
	}
	return common[word]
}

func (c *DiffChunker) buildGraph(tokens map[string][]string) map[string]map[string]int {
	graph := make(map[string]map[string]int)
	fileNames := make([]string, 0, len(tokens))
	for name := range tokens {
		fileNames = append(fileNames, name)
	}

	for i := 0; i < len(fileNames); i++ {
		for j := i + 1; j < len(fileNames); j++ {
			name1 := fileNames[i]
			name2 := fileNames[j]

			force := c.calculateForce(name1, name2, tokens[name1], tokens[name2])

			if force > 0 {
				if graph[name1] == nil {
					graph[name1] = make(map[string]int)
				}
				if graph[name2] == nil {
					graph[name2] = make(map[string]int)
				}
				graph[name1][name2] = force
				graph[name2][name1] = force
			}
		}
	}

	return graph
}

func (c *DiffChunker) calculateForce(name1, name2 string, tokens1, tokens2 []string) int {
	force := 0

	prefix1 := getPrefix(name1)
	prefix2 := getPrefix(name2)
	if prefix1 == prefix2 && prefix1 != "" {
		force += 100
	}

	tokenSet1 := make(map[string]bool)
	for _, t := range tokens1 {
		tokenSet1[t] = true
	}

	shared := 0
	for _, t := range tokens2 {
		if tokenSet1[t] {
			shared++
		}
	}

	force += shared

	return force
}

func getPrefix(name string) string {
	ext := strings.LastIndex(name, ".")
	if ext > 0 {
		return name[:ext]
	}
	return name
}

func (c *DiffChunker) pruneGraph(graph map[string]map[string]int) map[string]map[string]int {
	pruned := make(map[string]map[string]int)

	for name, connections := range graph {
		for other, force := range connections {
			if force >= c.minForce {
				if pruned[name] == nil {
					pruned[name] = make(map[string]int)
				}
				pruned[name][other] = force
			}
		}
	}

	return pruned
}

func (c *DiffChunker) createClusters(graph map[string]map[string]int, files []fileInfo) [][]string {
	visited := make(map[string]bool)
	var clusters [][]string

	for _, f := range files {
		name := f.name

		if visited[name] {
			continue
		}

		if len(graph[name]) == 0 {
			visited[name] = true
			clusters = append(clusters, []string{name})
			continue
		}

		cluster := c.bfsCluster(name, graph, visited)
		clusters = append(clusters, cluster)
	}

	clusters = c.sortClustersByForce(clusters, graph)

	return clusters
}

func (c *DiffChunker) sortClustersByForce(clusters [][]string, graph map[string]map[string]int) [][]string {
	type scoredCluster struct {
		files []string
		score int
	}

	var scored []scoredCluster

	for _, cluster := range clusters {
		score := 0
		for _, name := range cluster {
			for _, force := range graph[name] {
				score += force
			}
		}
		scored = append(scored, scoredCluster{files: cluster, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([][]string, len(scored))
	for i, s := range scored {
		result[i] = s.files
	}

	return result
}

func (c *DiffChunker) bfsCluster(start string, graph map[string]map[string]int, visited map[string]bool) []string {
	var cluster []string
	queue := []string{start}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}

		visited[current] = true
		cluster = append(cluster, current)

		for neighbor := range graph[current] {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	return cluster
}

func (c *DiffChunker) buildChunks(clusters [][]string, files []fileInfo, maxChunkSize int) []domain.DiffChunk {
	fileMap := make(map[string]fileInfo)
	for _, f := range files {
		fileMap[f.name] = f
	}

	var chunks []domain.DiffChunk
	var orphans []string

	for _, cluster := range clusters {
		if len(cluster) == 1 && len(fileMap[cluster[0]].diff) == 0 {
			continue
		}

		if len(cluster) == 1 {
			orphans = append(orphans, cluster[0])
			continue
		}

		var currentFiles []string
		var currentDiff strings.Builder
		currentSize := 0

		for _, name := range cluster {
			f := fileMap[name]
			if f.size > maxChunkSize {
				if currentSize > 0 {
					chunks = append(chunks, domain.DiffChunk{
						Files: currentFiles,
						Diff:  currentDiff.String(),
					})
					currentFiles = nil
					currentDiff.Reset()
					currentSize = 0
				}

				chunks = append(chunks, domain.DiffChunk{
					Files: []string{f.name},
					Diff:  f.diff,
				})
				continue
			}

			if currentSize+f.size > maxChunkSize && currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{
					Files: currentFiles,
					Diff:  currentDiff.String(),
				})
				currentFiles = nil
				currentDiff.Reset()
				currentSize = 0
			}

			if len(currentFiles) >= c.maxFilesPerChunk && currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{
					Files: currentFiles,
					Diff:  currentDiff.String(),
				})
				currentFiles = nil
				currentDiff.Reset()
				currentSize = 0
			}

			currentFiles = append(currentFiles, f.name)
			currentDiff.WriteString(fmtFileHeader(f.name))
			currentDiff.WriteString(f.diff)
			currentDiff.WriteString("\n\n")
			currentSize += f.size
		}

		if currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{
				Files: currentFiles,
				Diff:  currentDiff.String(),
			})
		}
	}

	if len(orphans) > 0 {
		var currentFiles []string
		var currentDiff strings.Builder
		currentSize := 0

		for _, name := range orphans {
			f := fileMap[name]

			if f.size > maxChunkSize {
				if currentSize > 0 {
					chunks = append(chunks, domain.DiffChunk{
						Files: currentFiles,
						Diff:  currentDiff.String(),
					})
					currentFiles = nil
					currentDiff.Reset()
					currentSize = 0
				}

				chunks = append(chunks, domain.DiffChunk{
					Files: []string{f.name},
					Diff:  f.diff,
				})
				continue
			}

			if currentSize+f.size > maxChunkSize && currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{
					Files: currentFiles,
					Diff:  currentDiff.String(),
				})
				currentFiles = nil
				currentDiff.Reset()
				currentSize = 0
			}

			if len(currentFiles) >= c.maxFilesPerChunk && currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{
					Files: currentFiles,
					Diff:  currentDiff.String(),
				})
				currentFiles = nil
				currentDiff.Reset()
				currentSize = 0
			}

			currentFiles = append(currentFiles, f.name)
			currentDiff.WriteString(fmtFileHeader(f.name))
			currentDiff.WriteString(f.diff)
			currentDiff.WriteString("\n\n")
			currentSize += f.size
		}

		if currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{
				Files: currentFiles,
				Diff:  currentDiff.String(),
			})
		}
	}

	return chunks
}

func fmtFileHeader(name string) string {
	return "## " + name + "\n\n"
}

func (c *DiffChunker) splitLargeFile(fileDiff, fileName string, maxChunkSize int) []domain.DiffChunk {
	var chunks []domain.DiffChunk
	var current strings.Builder
	currentSize := 0

	lines := strings.Split(fileDiff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") && currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{
				Files: []string{fileName},
				Diff:  current.String(),
			})
			current.Reset()
			currentSize = 0
		}

		current.WriteString(line + "\n")
		currentSize += len(line) + 1

		if currentSize > maxChunkSize {
			chunks = append(chunks, domain.DiffChunk{
				Files: []string{fileName},
				Diff:  current.String(),
			})
			current.Reset()
			currentSize = 0
		}
	}

	if currentSize > 0 {
		chunks = append(chunks, domain.DiffChunk{
			Files: []string{fileName},
			Diff:  current.String(),
		})
	}

	return chunks
}

func (c *DiffChunker) extractFileDiff(fullDiff string, file *gitdiff.File) string {
	fileName := c.getFileName(file)
	if fileName == "" {
		return ""
	}

	lines := strings.Split(fullDiff, "\n")
	var result []string
	inFile := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			searchName := " " + fileName
			isOurFile := strings.Contains(line, " a/"+fileName) ||
				strings.Contains(line, " b/"+fileName) ||
				strings.Contains(line, searchName)

			if isOurFile {
				inFile = true
				result = append(result, line)
			} else {
				if inFile {
					break
				}
			}
		} else if inFile {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func (c *DiffChunker) fallbackChunk(diff string, maxChunkSize int) []domain.DiffChunk {
	var chunks []domain.DiffChunk
	var currentFiles []string
	var current strings.Builder
	currentSize := 0

	lines := strings.Split(diff, "\n")
	filePattern := regexp.MustCompile(`diff --git a/(.*) b/(.*)`)

	for _, line := range lines {
		if matches := filePattern.FindStringSubmatch(line); len(matches) > 0 {
			if currentSize > 0 && len(currentFiles) >= c.maxFilesPerChunk {
				chunks = append(chunks, domain.DiffChunk{
					Files: currentFiles,
					Diff:  current.String(),
				})
				currentFiles = nil
				current.Reset()
				currentSize = 0
			}
			currentFiles = append(currentFiles, matches[2])
		}

		current.WriteString(line + "\n")
		currentSize += len(line) + 1

		if currentSize > maxChunkSize {
			chunks = append(chunks, domain.DiffChunk{
				Files: currentFiles,
				Diff:  current.String(),
			})
			currentFiles = nil
			current.Reset()
			currentSize = 0
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, domain.DiffChunk{
			Files: currentFiles,
			Diff:  current.String(),
		})
	}

	return chunks
}
