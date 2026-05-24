package workflow

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// Execute runs the full commit workflow (prepare + execute).
func (s *CommitService) Execute(instruction string, preview bool) (string, error) {
	log.Printf("[DEBUG] Execute: starting for instruction: %s", instruction)
	chunks, msgs, deleted, warnings, err := s.prepareChunksAndMessages(instruction, "")
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			log.Printf("[DEBUG] Execute: nothing to commit error: %v", err)
			resp, jsonErr := json.Marshal(CommitResult{Operation: "commit", Message: err.Error(), Type: "write"})
			if jsonErr != nil {
				return fmt.Sprintf(`{"operation":"commit","message":%q}`, err.Error()), nil
			}
			return string(resp), nil
		}
		return "", err
	}

	log.Printf("[DEBUG] Execute: prepared %d chunks, %d deleted files", len(chunks), len(deleted))
	return s.executeSync(instruction, chunks, msgs, deleted, warnings)
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
		s.captureCommit(messages[i])
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
	resp, jsonErr := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	if jsonErr != nil {
		return "", fmt.Errorf("marshal result: %w", jsonErr)
	}
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
		s.captureCommit(msg)
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
				s.captureCommit(msg)
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
	resp, jsonErr := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	if jsonErr != nil {
		return "", fmt.Errorf("marshal result: %w", jsonErr)
	}
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

func (s *CommitService) executeSync(instruction string, chunks []domain.DiffChunk, messages []string, deleted []string, warnings []string) (string, error) {
	log.Printf("[DEBUG] executeSync: starting with %d chunks", len(chunks))
	s.taskLog.logStart()
	var committed []string

	// ---- Stage + commit in chunk order using pre-generated messages ----
	for i, chunk := range chunks {
		if i >= len(messages) || messages[i] == "" || messages[i] == "chore: no meaningful changes" {
			continue
		}
		msg := messages[i]
		log.Printf("[DEBUG] executeSync: chunk %d message: %s", i, msg)
		log.Printf("[DEBUG] executeSync: staging chunk %d files: %v", i, chunk.Files)
		if err := s.git.Add(chunk.Files); err != nil {
			log.Printf("[DEBUG] executeSync: stage error: %v", err)
			s.rollback(committed)
			return "", fmt.Errorf("failed to stage chunk %d: %w", i+1, err)
		}
		log.Printf("[DEBUG] executeSync: committing chunk %d", i)
		if _, err := s.git.Commit(msg); err != nil {
			log.Printf("[DEBUG] executeSync: commit error: %v", err)
			s.rollback(committed)
			return "", fmt.Errorf("failed commit %d: %w", i+1, err)
		}
		committed = append(committed, msg)
		s.taskLog.logCommit(msg)
		s.taskLog.logProgress(len(committed), len(chunks))
		s.captureCommit(msg)
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
				s.captureCommit(msg)
			}
		}
	}

	if len(committed) == 0 {
		return "", fmt.Errorf("no commits were generated")
	}

	s.taskLog.logDone(len(committed))
	resp, jsonErr := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	if jsonErr != nil {
		return "", fmt.Errorf("marshal result: %w", jsonErr)
	}
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

// captureCommit captures the commit metadata after a successful git commit.
// It calls git.Head() for the SHA, gets the author from git config and the
// current date, constructs a CommitEntry, and appends it to the CommitStore.
// If commitStore is nil, it's a no-op. Append failures are logged but do not
// fail the commit operation.
func (s *CommitService) captureCommit(msg string) {
	if s.commitStore == nil {
		return
	}

	sha, err := s.git.Head()
	if err != nil {
		log.Printf("[WARN] Failed to get HEAD after commit: %v", err)
		return
	}

	// Get author from git config (may be empty if not configured)
	author, _ := s.git.ConfigGet("user.name")
	date := time.Now().UTC().Format(time.RFC3339)

	opts := []domain.CommitEntryOption{domain.WithDate(date)}
	if author != "" {
		opts = append(opts, domain.WithAuthor(author))
	}

	entry, err := domain.NewCommitEntry(sha, msg, opts...)
	if err != nil {
		log.Printf("[WARN] Failed to create CommitEntry: %v", err)
		return
	}

	if err := s.commitStore.Append(entry); err != nil {
		log.Printf("[WARN] Failed to append commit entry: %v", err)
	}
}
