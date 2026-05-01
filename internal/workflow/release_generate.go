package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
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
	var warningsMu sync.Mutex

	results := make([]chunkChangelogResult, len(chunks))

	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(int64(s.cfg.NumParallel))

	for i, chunk := range chunks {
		idx := i
		ch := chunk
		if err := sem.Acquire(ctx, 1); err != nil {
			break
		}
		g.Go(func() error {
			defer sem.Release(1)
			result, err := s.llm.GenerateChangelog(ch, "", "")
			if err != nil {
				warningsMu.Lock()
				warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
				warningsMu.Unlock()
				s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
				return nil
			}
			results[idx] = chunkChangelogResult{chunk: ch, index: idx, result: result}
			return nil
		})
	}

	_ = g.Wait()

	var changelogContent string
	successCount := 0
	for _, r := range results {
		if r.result == "" {
			continue
		}
		successCount++
		if changelogContent != "" {
			changelogContent += "\n\n"
		}
		changelogContent += r.result
	}

	// If no results, fall back to raw commits
	if successCount == 0 {
		changelogContent = strings.Join(chunks, "\n\n")
	}

	s.taskLog.logChangelogDone(successCount)
	return changelogContent, warnings, nil
}

func (s *ReleaseService) generateBackground(chunks []string) (string, []string, error) {
	s.taskLog.logStart()
	s.taskLog.logProgress(0, len(chunks))

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		var warningsMu sync.Mutex
		var warnings []string
		results := make([]chunkChangelogResult, len(chunks))

		g, ctx := errgroup.WithContext(ctx)
		sem := semaphore.NewWeighted(int64(s.cfg.NumParallel))

		for i, chunk := range chunks {
			idx := i
			ch := chunk
			if err := sem.Acquire(ctx, 1); err != nil {
				break
			}
			g.Go(func() error {
				defer sem.Release(1)
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				result, err := s.llm.GenerateChangelog(ch, "", "")
				if err != nil {
					warningsMu.Lock()
					warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
					warningsMu.Unlock()
					s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
					return nil
				}
				results[idx] = chunkChangelogResult{chunk: ch, index: idx, result: result}
				return nil
			})
		}

		_ = g.Wait()

		var chunksProcessed int
		var changelogContent string
		for _, r := range results {
			if r.result == "" {
				continue
			}
			chunksProcessed++
			s.taskLog.logProgress(chunksProcessed, len(chunks))
			if changelogContent != "" {
				changelogContent += "\n\n"
			}
			changelogContent += r.result
		}

		if chunksProcessed == 0 {
			s.taskLog.logError("no chunks succeeded")
			s.taskLog.logDone()
			return
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
