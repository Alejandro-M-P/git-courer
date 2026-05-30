package openai_standard

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

// VerifySecrets uses the LLM to verify if a diff contains sensitive information.
func (a *OpenAIStandardAdapter) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	if len(findings) == 0 {
		return false, nil
	}

	var findingsStr strings.Builder
	for _, f := range findings {
		findingsStr.WriteString(fmt.Sprintf("- %s in %s (line %d): %s\n", f.Type, f.File, f.Line, f.Content))
	}

	tmpl, err := prompts.Get("credential_audit")
	if err != nil {
		return false, fmt.Errorf("prompt not found: %w", err)
	}
	prompt, err := prompts.Render(tmpl, map[string]string{
		"Diff":     diff,
		"Findings": findingsStr.String(),
	})
	if err != nil {
		return false, fmt.Errorf("render credential_audit prompt: %w", err)
	}

	response, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:   "secret_verification",
		temperature: floatPtr(verifyTemp),
		maxTokens:   verifyMaxTokens,
	})
	if err != nil {
		return false, fmt.Errorf("verify secrets failed: %w", err)
	}
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(response)), "YES"), nil
}


// GenerateChangelogByArea translates pre-filtered, area-grouped commits into user-facing release notes.
// formattedGroups uses group_N keys (e.g. group_1, group_2) — the LLM never sees area names.
// nameMap maps group_N keys back to area names for remapping the response.
func (a *OpenAIStandardAdapter) GenerateChangelogByArea(formattedGroups string, nameMap map[string]string) (domain.ChangelogByArea, error) {
	tmpl, err := prompts.Get("changelog_areas")
	if err != nil {
		return nil, fmt.Errorf("prompt not found: %w", err)
	}
	prompt, err := prompts.Render(tmpl, map[string]string{
		"Context": a.context,
		"Groups":  formattedGroups,
	})
	if err != nil {
		return nil, fmt.Errorf("render changelog_areas prompt: %w", err)
	}
	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "changelog_areas",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(changelogTemp),
		maxTokens:       changelogMaxTokens,
	})
	if err != nil {
		return nil, err
	}
	var ch domain.ChangelogByArea
	if err := parseJSON(result, &ch); err != nil {
		return nil, fmt.Errorf("parse changelog_areas: %w (raw response: %q)", err, result)
	}
	// Remap group_N keys to area names using nameMap
	if len(nameMap) > 0 {
		ch = remapChangelogByArea(ch, nameMap)
	}
	return ch, nil
}

// remapChangelogByArea replaces group_N keys with actual area names.
func remapChangelogByArea(ch domain.ChangelogByArea, nameMap map[string]string) domain.ChangelogByArea {
	result := make(domain.ChangelogByArea, len(ch))
	for groupKey, items := range ch {
		areaName, ok := nameMap[groupKey]
		if !ok {
			areaName = groupKey // fallback: keep group_N key
		}
		result[areaName] = items
	}
	return result
}

// RegenerateMessage generates new commit messages based on feedback.
func (a *OpenAIStandardAdapter) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	if len(previousMessages) != len(chunks) {
		return nil, fmt.Errorf("previous messages count %d does not match chunks count %d", len(previousMessages), len(chunks))
	}

	newMessages := make([]string, len(chunks))

	// Serial fast-path for NumParallel == 1 (identical to old behavior).
	if a.numParallel <= 1 {
		for i, chunk := range chunks {
			msg, err := a.regenerateChunk(chunk, feedback)
			if err != nil {
				return nil, err
			}
			newMessages[i] = msg
		}
		return newMessages, nil
	}

	// Parallel path: errgroup + semaphore bounded by numParallel.
	g, ctx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(int64(a.numParallel))
	var mu sync.Mutex
	var warnings []string

	for i, chunk := range chunks {
		i, chunk := i, chunk // capture loop vars
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			msg, err := a.regenerateChunk(chunk, feedback)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", i+1, err))
				newMessages[i] = ""
				return nil // do NOT fail entire call
			}
			newMessages[i] = msg
			return nil
		})
	}

	_ = g.Wait()

	if len(warnings) > 0 {
		return newMessages, fmt.Errorf("regenerate warnings (%d): %s", len(warnings), strings.Join(warnings, "; "))
	}
	return newMessages, nil
}

// regenerateChunk is the per-chunk logic extracted for reuse in serial and parallel paths.
func (a *OpenAIStandardAdapter) regenerateChunk(chunk domain.DiffChunk, feedback string) (string, error) {
	commitType, breaking := extractCommitInfo(chunk)
	prompt, err := prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.AnnotatedDiff, chunk.Diff, feedback, a.context, commitType, chunk.Scope, breaking, a.why))
	if err != nil {
		return "", fmt.Errorf("render regenerate prompt: %w", err)
	}

	commit, err := a.commitJSONWithFallback(prompt, regenTemp, regenMaxTokens)
	if err != nil {
		return "", err
	}

	commitType, breaking = extractCommitInfo(chunk)
	return commit.ToConventionalCommit(commitType, chunk.Scope, breaking), nil
}
