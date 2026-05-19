package workflow

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// GenerateCommitMessage generates per-chunk commit messages WITHOUT executing git.
// why is the user's reason for the change — it flows into the LLM prompt.
// When why is empty, behavior is identical to the existing pipeline (zero regression).
// Returns per-chunk messages in chunk order, or an error if staging is empty.
func (s *CommitService) GenerateCommitMessage(ctx context.Context, why string) ([]string, error) {
	// Set why on the LLM adapter — cleared on exit to ensure zero regression
	s.SetWhy(why)
	defer s.ClearWhy()

	state, err := s.prepareStages(why)
	if err != nil {
		return nil, err
	}

	messages, _ := s.generateMessages(state.chunks, why, "")

	if len(messages) == 0 {
		return nil, fmt.Errorf("no commit messages generated")
	}

	return messages, nil
}

// PrepareCommit prepares the commit without executing it.
// Returns generated messages, chunks, deleted files, warnings, reasoning, and error.
func (s *CommitService) PrepareCommit(instruction string) ([]string, []domain.DiffChunk, []string, []string, string, error) {
	state, err := s.prepareStages(instruction)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	messages := make([]string, len(state.chunks))
	var warnings []string
	var warningsMu sync.Mutex

	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(int64(s.cfg.NumParallel))

	for i, chunk := range state.chunks {
		idx := i
		ch := chunk
		if err := sem.Acquire(ctx, 1); err != nil {
			// Context cancelled — stop launching new goroutines
			break
		}
		g.Go(func() error {
			defer sem.Release(1)
			msg, err := s.llm.GenerateChunkMessage(ch)
			if err != nil {
				warningsMu.Lock()
				warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", idx+1, err))
				warningsMu.Unlock()
				return nil // per-chunk error is a warning, not a group failure
			}
			messages[idx] = msg
			return nil
		})
	}

	_ = g.Wait()

	if s.progress != nil {
		s.progress(4, 6, "Generating commit plan…")
	}

	return messages, state.chunks, state.deleted, warnings, "", nil
}
