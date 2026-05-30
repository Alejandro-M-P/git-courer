package workflow

import (
	"context"
	"fmt"

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

	_, msgs, _, _, err := s.prepareChunksAndMessages(why, "")
	if err != nil {
		return nil, err
	}

	if len(msgs) == 0 {
		return nil, fmt.Errorf("no commit messages generated")
	}

	return msgs, nil
}

// PrepareCommit prepares the commit without executing it.
// Returns generated messages, chunks, deleted files, warnings, reasoning, and error.
func (s *CommitService) PrepareCommit(instruction string) ([]string, []domain.DiffChunk, []string, []string, string, error) {
	chunks, msgs, deleted, warnings, err := s.prepareChunksAndMessages(instruction, "")
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	if s.progress != nil {
		s.progress(4, 6, "Generating commit plan…")
	}

	return msgs, chunks, deleted, warnings, "", nil
}

