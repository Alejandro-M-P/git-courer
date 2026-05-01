package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// Execute runs the full commit workflow (prepare + execute).
func (s *CommitService) Execute(instruction string, preview bool) (string, error) {
	log.Printf("[DEBUG] Execute: starting for instruction: %s", instruction)
	state, err := s.prepareStages(instruction)
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			log.Printf("[DEBUG] Execute: nothing to commit error: %v", err)
			resp, _ := json.Marshal(CommitResult{Operation: "commit", Message: err.Error(), Type: "write"})
			return string(resp), nil
		}
		return "", err
	}

	log.Printf("[DEBUG] Execute: prepared %d chunks, %d deleted files", len(state.chunks), len(state.deleted))
	return s.executeSync(instruction, state.chunks, state.deleted)
}

// ExecutePrepared commits using pre-generated messages from PrepareCommit.
func (s *CommitService) ExecutePrepared(messages []string, chunks []domain.DiffChunk, instruction string) (string, error) {
	s.taskLog.logStart()
	var committed []string
	var warnings []string

	// Stage all to get proper diff calculation
	if err := s.git.Add([]string{"."}); err != nil {
		return "", fmt.Errorf("failed to stage files: %w", err)
	}

	// Unstage everything but keep working tree changes
	if _, err := s.git.Reset("HEAD", "."); err != nil {
		return "", fmt.Errorf("failed to reset staging: %w", err)
	}

	for i, chunk := range chunks {
		if messages[i] == "" || messages[i] == "chore: no meaningful changes" {
			continue
		}
		if err := s.git.Add(chunk.Files); err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d stage skipped: %v", i+1, err))
			continue
		}
		if _, err := s.git.Commit(messages[i]); err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d commit skipped: %v", i+1, err))
			continue
		}
		committed = append(committed, messages[i])
		s.taskLog.logCommit(messages[i])
		s.taskLog.logProgress(len(committed), len(chunks))
	}

	if len(committed) == 0 {
		return "", fmt.Errorf("no commits were generated")
	}

	if strings.Contains(strings.ToLower(instruction), "push") {
		pushResult, err := s.git.Push()
		if err != nil {
			return "", fmt.Errorf("push failed: %w", err)
		}
		s.taskLog.logPush(pushResult)
	}

	s.taskLog.logDone(len(committed))
	resp, _ := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	return string(resp), nil
}

// ExecuteFromPlan commits using pre-approved messages and per-chunk file lists from the plan.
// chunkFiles[i] contains the files to stage for messages[i]. If chunkFiles is nil or shorter
// than messages, remaining messages are committed with whatever is currently staged.
// deletedFiles contains files with status "D " to commit separately at the end.
func (s *CommitService) ExecuteFromPlan(messages []string, chunkFiles [][]string, deletedFiles []string, instruction string) (string, error) {
	log.Printf("[DEBUG] ExecuteFromPlan: START")
	s.taskLog.logStart()
	var committed []string
	var warnings []string

	// prepareStages (called during COMMIT_START) leaves all files staged as one block.
	// Reset staging so we can commit per-chunk cleanly, just like ExecutePrepared does.
	log.Printf("[DEBUG] ExecuteFromPlan: resetting staging")
	if err := s.git.Add([]string{"."}); err != nil {
		return "", fmt.Errorf("failed to stage files: %w", err)
	}
	if _, err := s.git.Reset("HEAD", "."); err != nil {
		return "", fmt.Errorf("failed to reset staging area: %w", err)
	}
	log.Printf("[DEBUG] ExecuteFromPlan: staging reset done, starting commits")

	for i, msg := range messages {
		if msg == "" || msg == "chore: no meaningful changes" {
			continue
		}
		if i < len(chunkFiles) && len(chunkFiles[i]) > 0 {
			if err := s.git.Add(chunkFiles[i]); err != nil {
				// File may already be staged (e.g., a deletion staged by prepareStages).
				// Log the warning and attempt the commit anyway — git commit will fail
				// gracefully with "nothing to commit" if nothing is actually staged.
				warnings = append(warnings, fmt.Sprintf("Chunk %d stage warning: %v", i+1, err))
			}
		}
		result, err := s.git.Commit(msg)
		s.taskLog.logError(fmt.Sprintf("Commit %d result: %q, err: %v", i+1, result, err))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Commit %d failed: %v", i+1, err))
			continue
		}
		committed = append(committed, msg)
		s.taskLog.logCommit(msg)
		s.taskLog.logProgress(len(committed), len(messages))
	}

	if strings.Contains(strings.ToLower(instruction), "push") {
		log.Printf("[DEBUG] ExecuteFromPlan: pushing")
		pushResult, err := s.git.Push()
		if err != nil {
			return "", fmt.Errorf("push failed: %w", err)
		}
		s.taskLog.logPush(pushResult)
	}

	log.Printf("[DEBUG] ExecuteFromPlan: handling deleted files")
	if len(deletedFiles) > 0 {
		if err := s.git.Add(deletedFiles); err != nil {
			warnings = append(warnings, fmt.Sprintf("deleted files stage failed: %v", err))
		} else {
			msg := "chore: remove " + strings.Join(deletedFiles, ", ")
			if _, err := s.git.Commit(msg); err != nil {
				warnings = append(warnings, fmt.Sprintf("deleted files commit failed: %v", err))
			} else {
				committed = append(committed, msg)
				s.taskLog.logCommit(msg)
			}
		}
	}

	log.Printf("[DEBUG] ExecuteFromPlan: deleted files handled, committed=%d", len(committed))
	if len(committed) == 0 {
		log.Printf("[DEBUG] ExecuteFromPlan: no commits generated")
		return "", fmt.Errorf("no commits were generated")
	}

	log.Printf("[DEBUG] ExecuteFromPlan: %d commits done, marshalling result", len(committed))
	s.taskLog.logDone(len(committed))
	resp, _ := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	log.Printf("[DEBUG] ExecuteFromPlan: END, returning result")
	return string(resp), nil
}

// chunkGenResult holds the outcome of parallel LLM message generation for a single chunk.
type chunkGenResult struct {
	chunk   domain.DiffChunk
	message string
	index   int
	err     error
}

func (s *CommitService) executeSync(instruction string, chunks []domain.DiffChunk, deleted []string) (string, error) {
	log.Printf("[DEBUG] executeSync: starting with %d chunks", len(chunks))
	s.taskLog.logStart()
	var committed []string
	var warnings []string

	// ---- Phase 1: parallel LLM message generation ----
	results := make([]chunkGenResult, len(chunks))
	var warningsMu sync.Mutex

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
			log.Printf("[DEBUG] executeSync: generating message for chunk %d, files: %v", idx, ch.Files)
			msg, err := s.llm.GenerateChunkMessage(ch)
			results[idx] = chunkGenResult{chunk: ch, message: msg, index: idx, err: err}
			return nil // per-chunk errors are warnings, not group failures
		})
	}

	_ = g.Wait()

	// ---- Phase 2: serial git stage+commit in chunk order ----
	for _, r := range results {
		if r.err != nil {
			log.Printf("[DEBUG] executeSync: chunk %d LLM error: %v", r.index, r.err)
			warningsMu.Lock()
			warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			warningsMu.Unlock()
			s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			continue
		}
		log.Printf("[DEBUG] executeSync: chunk %d message: %s", r.index, r.message)
		log.Printf("[DEBUG] executeSync: staging chunk %d files: %v", r.index, r.chunk.Files)
		if err := s.git.Add(r.chunk.Files); err != nil {
			log.Printf("[DEBUG] executeSync: stage error: %v", err)
			s.rollback(committed)
			return "", fmt.Errorf("failed to stage chunk %d: %w", r.index+1, err)
		}
		log.Printf("[DEBUG] executeSync: committing chunk %d", r.index)
		if _, err := s.git.Commit(r.message); err != nil {
			log.Printf("[DEBUG] executeSync: commit error: %v", err)
			s.rollback(committed)
			return "", fmt.Errorf("failed commit %d: %w", r.index+1, err)
		}
		committed = append(committed, r.message)
		s.taskLog.logCommit(r.message)
		s.taskLog.logProgress(len(committed), len(chunks))
	}

	log.Printf("[DEBUG] executeSync: committed %d chunks", len(committed))

	if strings.Contains(strings.ToLower(instruction), "push") {
		pushResult, err := s.git.Push()
		if err != nil {
			return "", fmt.Errorf("push failed: %w", err)
		}
		s.taskLog.logPush(pushResult)
	}

	if len(deleted) > 0 {
		if err := s.git.Add(deleted); err != nil {
			warnings = append(warnings, fmt.Sprintf("deleted files stage failed: %v", err))
		} else {
			msg := "chore: remove " + strings.Join(deleted, ", ")
			if _, err := s.git.Commit(msg); err != nil {
				warnings = append(warnings, fmt.Sprintf("deleted files commit failed: %v", err))
			} else {
				committed = append(committed, msg)
				s.taskLog.logCommit(msg)
			}
		}
	}

	if len(committed) == 0 {
		return "", fmt.Errorf("no commits were generated")
	}

	s.taskLog.logDone(len(committed))
	resp, _ := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	return string(resp), nil
}

func (s *CommitService) rollback(committed []string) {
	for i := range committed {
		if _, err := s.git.Reset("--soft", "HEAD~1"); err != nil {
			s.taskLog.logError(fmt.Sprintf("rollback failed at step %d: %v", i+1, err))
			return
		}
	}
	s.git.Reset("HEAD", ".")
}