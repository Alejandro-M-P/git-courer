package chunkers

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

func (c *DiffChunker) buildGraph(files []fileInfo, symbols map[string]FileSymbols) map[string]map[string]int {
	graph := make(map[string]map[string]int)
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			f1, f2 := files[i], files[j]
			force := c.calculateForce(f1, f2, symbols[f1.name], symbols[f2.name])
			if force > 0 {
				if graph[f1.name] == nil {
					graph[f1.name] = make(map[string]int)
				}
				if graph[f2.name] == nil {
					graph[f2.name] = make(map[string]int)
				}
				graph[f1.name][f2.name] = force
				graph[f2.name][f1.name] = force
			}
		}
	}
	return graph
}

func (c *DiffChunker) calculateForce(f1, f2 fileInfo, s1, s2 FileSymbols) int {
	force := 0
	// 1. Code-Test Pair (+1000)
	if strings.TrimSuffix(f1.name, "_test.go") == strings.TrimSuffix(f2.name, "_test.go") ||
		strings.TrimSuffix(f1.name, ".test.ts") == strings.TrimSuffix(f2.name, ".ts") {
		force += 1000
	}
	// 2. Directory Affinity (+100)
	if filepath.Dir(f1.name) == filepath.Dir(f2.name) {
		force += 100
	}
	// 3. Semantic Link (Caller-Callee) (+500)
	for ref := range s1.References {
		if s2.Definitions[ref] {
			force += 500
		}
	}
	for ref := range s2.References {
		if s1.Definitions[ref] {
			force += 500
		}
	}
	// 4. Shared Symbols (+50 per match)
	for ref := range s1.References {
		if s2.References[ref] {
			force += 50
		}
	}
	return force
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
		if visited[f.name] {
			continue
		}
		if len(graph[f.name]) == 0 {
			visited[f.name] = true
			clusters = append(clusters, []string{f.name})
			continue
		}
		cluster := c.bfsCluster(f.name, graph, visited)
		clusters = append(clusters, cluster)
	}
	return c.sortClustersByForce(clusters, graph)
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

func (c *DiffChunker) buildChunks(clusters [][]string, files []fileInfo, maxChunkSize int) []domain.DiffChunk {
	fileMap := make(map[string]fileInfo)
	for _, f := range files {
		fileMap[f.name] = f
	}
	var chunks []domain.DiffChunk
	var orphans []string

	for _, cluster := range clusters {
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
					chunks = append(chunks, domain.DiffChunk{Files: currentFiles, Diff: currentDiff.String()})
					currentFiles, currentSize = nil, 0
					currentDiff.Reset()
				}
				chunks = append(chunks, domain.DiffChunk{Files: []string{f.name}, Diff: f.diff})
				continue
			}
			if (currentSize+f.size > maxChunkSize || len(currentFiles) >= c.maxFilesPerChunk) && currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{Files: currentFiles, Diff: currentDiff.String()})
				currentFiles, currentSize = nil, 0
				currentDiff.Reset()
			}
			currentFiles = append(currentFiles, f.name)
			currentDiff.WriteString("## " + f.name + "\n\n" + f.diff + "\n\n")
			currentSize += f.size
		}
		if currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{Files: currentFiles, Diff: currentDiff.String()})
		}
	}

	for _, name := range orphans {
		f := fileMap[name]
		if f.size > maxChunkSize {
			chunks = append(chunks, c.splitLargeFile(f.diff, f.name, maxChunkSize)...)
		} else {
			chunks = append(chunks, domain.DiffChunk{Files: []string{f.name}, Diff: f.diff})
		}
	}
	return chunks
}

func (c *DiffChunker) splitLargeFile(fileDiff, fileName string, maxChunkSize int) []domain.DiffChunk {
	var chunks []domain.DiffChunk
	var current strings.Builder
	currentSize := 0
	lines := strings.Split(fileDiff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") && currentSize > 0 {
			chunks = append(chunks, domain.DiffChunk{Files: []string{fileName}, Diff: current.String()})
			current.Reset()
			currentSize = 0
		}
		current.WriteString(line + "\n")
		currentSize += len(line) + 1
		if currentSize > maxChunkSize {
			chunks = append(chunks, domain.DiffChunk{Files: []string{fileName}, Diff: current.String()})
			current.Reset()
			currentSize = 0
		}
	}
	if current.Len() > 0 {
		chunks = append(chunks, domain.DiffChunk{Files: []string{fileName}, Diff: current.String()})
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

func (c *DiffChunker) fallbackChunk(diff string, maxChunkSize int) []domain.DiffChunk {
	var chunks []domain.DiffChunk
	var currentFiles []string
	var current strings.Builder
	currentSize := 0
	lines := strings.Split(diff, "\n")
	filePattern := regexp.MustCompile(`diff --git a/(.*) b/(.*)`)

	for _, line := range lines {
		if matches := filePattern.FindStringSubmatch(line); len(matches) > 0 {
			if (currentSize > maxChunkSize || len(currentFiles) >= c.maxFilesPerChunk) && currentSize > 0 {
				chunks = append(chunks, domain.DiffChunk{Files: currentFiles, Diff: current.String()})
				currentFiles, currentSize = nil, 0
				current.Reset()
			}
			currentFiles = append(currentFiles, matches[2])
		}
		current.WriteString(line + "\n")
		currentSize += len(line) + 1
	}
	if current.Len() > 0 {
		chunks = append(chunks, domain.DiffChunk{Files: currentFiles, Diff: current.String()})
	}
	return chunks
}