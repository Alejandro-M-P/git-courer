package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Generate generates the changelog from commits.
// Returns the generated changelog and any warnings.
// Supports background processing for large numbers of commits.
func (s *ReleaseService) Generate(commits string) (string, []string, error) {
	s.taskLog.logStartChangelog()

	chunks, err := s.logChunker.Chunk(commits, s.cfg.MaxCommitsPerChunk)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("failed to chunk commits: %v", err))
		return "", []string{err.Error()}, fmt.Errorf("failed to chunk commits: %w", err)
	}

	s.taskLog.logChunks(len(chunks))

	if len(chunks) > s.cfg.BackgroundThreshold {
		return s.generateBackground(chunks)
	}

	return s.generateSync(chunks)
}

func (s *ReleaseService) generateSync(chunks []string) (string, []string, error) {
	s.taskLog.logStart()
	var warnings []string
	var changelogContent string

	// Generate changelog for each chunk
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChan := make(chan chunkChangelogResult, len(chunks))
	go func() {
		defer close(resultChan)
		for i, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}
			result, err := s.llm.GenerateChangelog(chunk, "", "")
			select {
			case <-ctx.Done():
				return
			case resultChan <- chunkChangelogResult{chunk: chunk, index: i, result: result, err: err}:
			}
		}
	}()

	// Collect results
	var results []chunkChangelogResult
	for r := range resultChan {
		if r.err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			continue
		}
		results = append(results, r)
	}

	// If we got results, join them; otherwise use raw commits
	if len(results) > 0 {
		// Sort by index to maintain order
		sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
		for _, r := range results {
			if changelogContent != "" {
				changelogContent += "\n\n"
			}
			changelogContent += r.result
		}
	} else {
		changelogContent = strings.Join(chunks, "\n\n")
	}

	s.taskLog.logChangelogDone(len(results))
	return changelogContent, warnings, nil
}

func (s *ReleaseService) generateBackground(chunks []string) (string, []string, error) {
	s.taskLog.logStart()
	s.taskLog.logProgress(0, len(chunks))

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		var chunksProcessed int
		resultChan := make(chan chunkChangelogResult, len(chunks))
		go func() {
			for i, chunk := range chunks {
				select {
				case <-ctx.Done():
					resultChan <- chunkChangelogResult{index: i, err: ctx.Err()}
					continue
				default:
				}
				result, err := s.llm.GenerateChangelog(chunk, "", "")
				resultChan <- chunkChangelogResult{chunk: chunk, index: i, result: result, err: err}
			}
			close(resultChan)
		}()

		var results []chunkChangelogResult
		for r := range resultChan {
			if r.err != nil {
				s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
				continue
			}
			results = append(results, r)
			chunksProcessed++
			s.taskLog.logProgress(chunksProcessed, len(chunks))
		}

		if chunksProcessed == 0 {
			s.taskLog.logError("no chunks succeeded")
			s.taskLog.logDone()
			return
		}

		sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
		var changelogContent string
		for _, r := range results {
			if changelogContent != "" {
				changelogContent += "\n\n"
			}
			changelogContent += r.result
		}

		s.setChangelog(changelogContent)
		s.setPendingState("")

		s.taskLog.logChangelogDone(chunksProcessed)
		s.taskLog.logDone()
	}()

	resp, _ := json.Marshal(map[string]any{
		"operation": "release", "type": "write", "state": "running",
		"message": fmt.Sprintf(
			"Processing %d chunks in background. Check %q for progress. When done, call RELEASE_APPLY.",
			len(chunks), s.cfg.LogPath,
		),
	})
	return string(resp), nil, nil
}

// chunkChangelogResult holds the result of generating changelog for a chunk.
type chunkChangelogResult struct {
	chunk  string
	index  int
	result string // changelog generated for this chunk
	err    error
}