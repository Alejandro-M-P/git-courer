package workflow

import (
	"context"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// PrepareCommit prepares the commit without executing it.
// Returns generated messages, chunks, deleted files, warnings, reasoning, and error.
func (s *CommitService) PrepareCommit(instruction string) ([]string, []domain.DiffChunk, []string, []string, string, error) {
	state, err := s.prepareStages(instruction)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	messages := make([]string, len(state.chunks))
	var warnings []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChan := make(chan chunkResult, len(state.chunks))
	go func() {
		defer close(resultChan)
		for i, chunk := range state.chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}
			msg, err := s.llm.GenerateChunkMessage(chunk)
			select {
			case <-ctx.Done():
				return
			case resultChan <- chunkResult{chunk: chunk, message: msg, index: i, err: err}:
			}
		}
	}()

	for r := range resultChan {
		if r.err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			continue
		}
		messages[r.index] = r.message
	}

	return messages, state.chunks, state.deleted, warnings, "", nil
}