package openai_standard

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/shared/prompts"
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

// GenerateChangelogGrouped translates grouped commits into user-facing release notes.
// mode is "area", "stack", or "freeform". All modes use the same changelog prompt template.
// Returns raw markdown directly from the LLM — no JSON parsing or remapping.
func (a *OpenAIStandardAdapter) GenerateChangelogGrouped(formattedGroups string, nameMap map[string]string, customMessage string, mode string) (string, error) {
	// All modes use the same changelog prompt template.
	// The prompt instructs the LLM to translate grouped commits into release notes.
	tmpl, err := prompts.Get("changelog")
	if err != nil {
		return "", fmt.Errorf("prompt not found: %w", err)
	}
	cleanCtx := a.context
	if strings.Contains(cleanCtx, "areas:") {
		var lines []string
		for _, line := range strings.Split(cleanCtx, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "areas:") {
				lines = append(lines, line)
			}
		}
		cleanCtx = strings.TrimSpace(strings.Join(lines, "\n"))
	}

	prompt, err := prompts.Render(tmpl, map[string]string{
		"Context":       cleanCtx,
		"Groups":        formattedGroups,
		"CustomMessage": customMessage,
	})
	if err != nil {
		return "", fmt.Errorf("render changelog prompt: %w", err)
	}
	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "changelog",
		reasoningEffort: "none",
		temperature:     floatPtr(changelogTemp),
		maxTokens:       changelogMaxTokens,
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

// RegenerateChangelog regenerates a release changelog from the previous output and
// user feedback. It uses the dedicated changelog_regenerate prompt template, which
// reuses the same output rules as the changelog template but is grounded in the
// previous changelog and the feedback rather than raw commits.
func (a *OpenAIStandardAdapter) RegenerateChangelog(prevChangelog, feedback string) (string, error) {
	tmpl, err := prompts.Get("changelog_regenerate")
	if err != nil {
		return "", fmt.Errorf("prompt not found: %w", err)
	}
	prompt, err := prompts.Render(tmpl, map[string]string{
		"PreviousChangelog": prevChangelog,
		"Feedback":           feedback,
	})
	if err != nil {
		return "", fmt.Errorf("render changelog_regenerate prompt: %w", err)
	}
	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "changelog",
		reasoningEffort: "none",
		temperature:     floatPtr(changelogTemp),
		maxTokens:       changelogMaxTokens,
	})
	if err != nil {
		return "", err
	}
	return result, nil
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
	prompt, err := prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, "", "", "", chunk.AnnotatedDiff, chunk.Diff, feedback, a.context, commitType, chunk.Scope, breaking, a.why))
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
