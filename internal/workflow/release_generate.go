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

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// formatChangelogMarkdown renders a domain.Changelog as human-readable markdown.
func formatChangelogMarkdown(ch *domain.Changelog) string {
	var sb strings.Builder
	if len(ch.Features) > 0 {
		sb.WriteString("## Features\n")
		for _, f := range ch.Features {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Fixes) > 0 {
		sb.WriteString("## Fixes\n")
		for _, f := range ch.Fixes {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Breaking) > 0 {
		sb.WriteString("## Breaking Changes\n")
		for _, f := range ch.Breaking {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Docs) > 0 {
		sb.WriteString("## Documentation\n")
		for _, f := range ch.Docs {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Perf) > 0 {
		sb.WriteString("## Performance\n")
		for _, f := range ch.Perf {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Internal) > 0 {
		sb.WriteString("## Internal\n")
		for _, f := range ch.Internal {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}
	return strings.TrimSpace(sb.String())
}

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

	results := make([]*domain.Changelog, len(chunks))

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
			changelog, err := s.llm.GenerateChangelog(ch, "", "")
			if err != nil {
				warningsMu.Lock()
				warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
				warningsMu.Unlock()
				s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
				return nil
			}
			results[idx] = changelog
			return nil
		})
	}

	_ = g.Wait()

	// Merge all changelogs into one markdown string
	var allChangelog domain.Changelog
	for _, ch := range results {
		if ch == nil {
			continue
		}
		allChangelog.Features = append(allChangelog.Features, ch.Features...)
		allChangelog.Fixes = append(allChangelog.Fixes, ch.Fixes...)
		allChangelog.Breaking = append(allChangelog.Breaking, ch.Breaking...)
		allChangelog.Docs = append(allChangelog.Docs, ch.Docs...)
		allChangelog.Perf = append(allChangelog.Perf, ch.Perf...)
		allChangelog.Internal = append(allChangelog.Internal, ch.Internal...)
	}

	// If no results, fall back to raw commits
	changelogContent := formatChangelogMarkdown(&allChangelog)
	if changelogContent == "" && len(warnings) > 0 {
		changelogContent = strings.Join(chunks, "\n\n")
	}

	successCount := 0
	for _, ch := range results {
		if ch != nil {
			successCount++
		}
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
		results := make([]*domain.Changelog, len(chunks))

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
				changelog, err := s.llm.GenerateChangelog(ch, "", "")
				if err != nil {
					warningsMu.Lock()
					warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
					warningsMu.Unlock()
					s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
					return nil
				}
				results[idx] = changelog
				return nil
			})
		}

		_ = g.Wait()

		// Merge all changelogs
		var allChangelog domain.Changelog
		var chunksProcessed int
		for _, ch := range results {
			if ch == nil {
				continue
			}
			chunksProcessed++
			s.taskLog.logProgress(chunksProcessed, len(chunks))
			allChangelog.Features = append(allChangelog.Features, ch.Features...)
			allChangelog.Fixes = append(allChangelog.Fixes, ch.Fixes...)
			allChangelog.Breaking = append(allChangelog.Breaking, ch.Breaking...)
			allChangelog.Docs = append(allChangelog.Docs, ch.Docs...)
			allChangelog.Perf = append(allChangelog.Perf, ch.Perf...)
			allChangelog.Internal = append(allChangelog.Internal, ch.Internal...)
		}

		if chunksProcessed == 0 {
			s.taskLog.logError("no chunks succeeded")
			s.taskLog.logDone()
			return
		}

		changelogContent := formatChangelogMarkdown(&allChangelog)
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
