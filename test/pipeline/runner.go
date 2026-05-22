// Package pipeline provides file-based pipeline testing infrastructure for
// the commit workflow. Each stage is a standalone function that reads input,
// processes it, and writes output — making the full data flow inspectable.
package pipeline

import (
	"fmt"
)

// StageFunc is the signature for all pipeline stage functions.
type StageFunc func(input []byte, deps StageDeps) ([]byte, error)

// stages is the ordered list of pipeline stages.
// Index 0 → Stage00, index 7 → Stage07.
var stages = []StageFunc{
	Stage00Request,
	Stage01Diff,
	Stage02Security,
	Stage03Chunks,
	Stage04Annotation,
	Stage05Classification,
	Stage06LLM,
	Stage07Result,
}

// StageNames maps stage index to human-readable name.
var StageNames = map[int]string{
	0: "request",
	1: "diff",
	2: "security",
	3: "chunks",
	4: "annotation",
	5: "classification",
	6: "llm",
	7: "result",
}

// RunStage executes a single pipeline stage by index.
func RunStage(index int, input []byte, deps StageDeps) ([]byte, error) {
	if index < 0 || index >= len(stages) {
		return nil, fmt.Errorf("stage index %d out of range [0, %d]", index, len(stages)-1)
	}
	return stages[index](input, deps)
}

// RunRange executes pipeline stages from start through end (inclusive).
// Each stage's output becomes the next stage's input.
func RunRange(start, end int, input []byte, deps StageDeps) ([]byte, error) {
	if start < 0 || end >= len(stages) || start > end {
		return nil, fmt.Errorf("invalid range [%d, %d], valid range is [0, %d]", start, end, len(stages)-1)
	}
	current := input
	for i := start; i <= end; i++ {
		output, err := stages[i](current, deps)
		if err != nil {
			return nil, fmt.Errorf("stage %d (%s): %w", i, StageNames[i], err)
		}
		current = output
	}
	return current, nil
}

// RunAll executes the full pipeline (stages 0-7).
func RunAll(request []byte, deps StageDeps) ([]byte, error) {
	return RunRange(0, len(stages)-1, request, deps)
}

// NumStages returns the number of pipeline stages.
func NumStages() int {
	return len(stages)
}