package chunkers

import (
	"errors"
	"strings"
)

// ErrEmptyLog is returned when the log input is empty.
var ErrEmptyLog = errors.New("log input is empty")

// LogChunker splits git log output into chunks based on commit count.
type LogChunker struct {
	commitsPerChunk int
}

// NewLogChunker creates a new LogChunker with the specified context window.
// It calculates commitsPerChunk as approximately contextWindow/200,
// with a minimum of 10 and maximum of 25.
func NewLogChunker(contextWindow int) *LogChunker {
	commitsPerChunk := contextWindow / 200
	if commitsPerChunk < 10 {
		commitsPerChunk = 10
	}
	if commitsPerChunk > 25 {
		commitsPerChunk = 25
	}
	return &LogChunker{
		commitsPerChunk: commitsPerChunk,
	}
}

// Chunk splits a git log string into chunks of approximately commitsPerChunk commits.
func (c *LogChunker) Chunk(log string, commitsPerChunk int) ([]string, error) {
	if log == "" {
		return nil, ErrEmptyLog
	}

	if commitsPerChunk <= 0 {
		commitsPerChunk = c.commitsPerChunk
	}

	// Split log into individual commits
	commits := splitIntoCommits(log)
	if len(commits) == 0 {
		return nil, nil
	}

	// Group commits into chunks
	var chunks []string
	for i := 0; i < len(commits); i += commitsPerChunk {
		end := i + commitsPerChunk
		if end > len(commits) {
			end = len(commits)
		}
		chunk := strings.Join(commits[i:end], "\n")
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// splitIntoCommits splits a git log output into individual commits.
// Each commit starts with a line containing "commit " (hash).
func splitIntoCommits(log string) []string {
	lines := strings.Split(log, "\n")
	var commits []string
	var current strings.Builder
	firstCommit := true

	for _, line := range lines {
		// Check if this is a commit header line
		isCommitLine := strings.HasPrefix(line, "commit ") ||
			strings.HasPrefix(line, "COMMIT:") ||
			strings.HasPrefix(line, "commit:")

		if isCommitLine && !firstCommit {
			// Save the current commit and start a new one
			if current.Len() > 0 {
				commits = append(commits, strings.TrimSpace(current.String()))
			}
			current.Reset()
		}

		if current.Len() > 0 || line != "" {
			current.WriteString(line)
			current.WriteString("\n")
		}
		firstCommit = false
	}

	// Add the last commit
	if current.Len() > 0 {
		commits = append(commits, strings.TrimSpace(current.String()))
	}

	return commits
}
