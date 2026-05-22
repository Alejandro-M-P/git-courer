// Package pipeline provides file-based pipeline testing infrastructure for
// the commit workflow. Each stage is a standalone function that reads input,
// processes it, and writes output — making the full data flow inspectable.
package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Stage00Request validates and serializes a CommitRequest.
// Input: raw JSON bytes (or empty for identity).
// Output: pretty-printed JSON of the request.
func Stage00Request(input []byte, deps StageDeps) ([]byte, error) {
	var req CommitRequest
	if len(input) == 0 {
		return nil, fmt.Errorf("stage 00: empty input")
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("stage 00: invalid request JSON: %w", err)
	}
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("stage 00: marshal request: %w", err)
	}
	return data, nil
}

// Stage01Diff calls git.Status and git.DiffStaged to get the diff output.
// Input: request JSON bytes (Stage 0 output).
// Output: raw diff text.
func Stage01Diff(input []byte, deps StageDeps) ([]byte, error) {
	if deps.Git == nil {
		return nil, fmt.Errorf("stage 01: Git port is required")
	}
	// Get staged diff
	diff, err := deps.Git.DiffStaged()
	if err != nil {
		return nil, fmt.Errorf("stage 01: git diff staged: %w", err)
	}
	return []byte(diff), nil
}

// Stage02Security checks files for security issues.
// Input: diff text bytes.
// Output: JSON SecurityResult.
func Stage02Security(input []byte, deps StageDeps) ([]byte, error) {
	if deps.Security == nil {
		return nil, fmt.Errorf("stage 02: SecurityService port is required")
	}
	diff := string(input)
	// Extract file list from diff for security check
	files := extractFilesFromDiff(diff)
	result := deps.Security.CheckFiles(files, diff)

	secResult := SecurityResult{
		Blocked: result.Blocked,
	}
	for _, f := range result.Files {
		secResult.Files = append(secResult.Files, FileBlockResult{
			File:    f.File,
			Reason:  f.Reason,
			Type:    f.Type,
			Halted:  f.Halted,
			Message: f.Message,
			Line:    f.Line,
		})
	}

	data, err := json.MarshalIndent(secResult, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("stage 02: marshal security result: %w", err)
	}
	return data, nil
}

// Stage03Chunks splits a diff into chunks. This is a pure stage.
// Input: diff text bytes.
// Output: JSON array of DiffChunk.
func Stage03Chunks(input []byte, deps StageDeps) ([]byte, error) {
	if deps.Chunker == nil {
		return nil, fmt.Errorf("stage 03: DiffChunker port is required")
	}
	chunkSize := deps.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 4000
	}
	chunks, err := deps.Chunker.Chunk(string(input), chunkSize)
	if err != nil {
		return nil, fmt.Errorf("stage 03: chunk: %w", err)
	}
	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("stage 03: marshal chunks: %w", err)
	}
	return data, nil
}

// Stage04Annotation annotates chunks with AST labels. This is a pure stage
// (no external I/O beyond file read/write).
// Input: JSON array of DiffChunk.
// Output: JSON array of annotated DiffChunk.
func Stage04Annotation(input []byte, deps StageDeps) ([]byte, error) {
	if deps.Annotator == nil {
		return nil, fmt.Errorf("stage 04: ChunkAnnotator port is required")
	}
	if deps.ContentProvider == nil {
		return nil, fmt.Errorf("stage 04: ContentProvider port is required")
	}
	var chunks []domain.DiffChunk
	if err := json.Unmarshal(input, &chunks); err != nil {
		return nil, fmt.Errorf("stage 04: unmarshal chunks: %w", err)
	}

	// Get raw diff content for annotation merging
	// We need the original diff for MergeDiffIntoAnnotations
	var rawDiff string
	if len(chunks) > 0 {
		rawDiff = chunks[0].Diff
	}

	for i := range chunks {
		files, err := deps.ContentProvider.GetContents(chunks[i].Files)
		if err != nil {
			// Continue without file content — annotation is best-effort
			continue
		}
		// Convert ports.FileContent to the format AnnotateWithContent expects
		portFiles := make([]ports.FileContent, len(files))
		copy(portFiles, files)

		if err := deps.Annotator.AnnotateWithContent(&chunks[i], portFiles, rawDiff); err != nil {
			// Annotation failure is non-fatal — continue with unannotated chunks
			continue
		}
	}

	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("stage 04: marshal annotated chunks: %w", err)
	}
	return data, nil
}

// Stage05Classification classifies chunks by commit type. This is a pure stage.
// Input: JSON array of annotated DiffChunk.
// Output: JSON array of classified DiffChunk.
func Stage05Classification(input []byte, deps StageDeps) ([]byte, error) {
	if deps.Classifier == nil {
		return nil, fmt.Errorf("stage 05: MessageClassifier port is required")
	}
	var chunks []domain.DiffChunk
	if err := json.Unmarshal(input, &chunks); err != nil {
		return nil, fmt.Errorf("stage 05: unmarshal chunks: %w", err)
	}

	for i := range chunks {
		commitType, confidence := deps.Classifier.Classify(&chunks[i])
		chunks[i].CommitType = commitType
		chunks[i].ConfidenceScore = confidence
	}

	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("stage 05: marshal classified chunks: %w", err)
	}
	return data, nil
}

// Stage06LLM generates a commit message using the LLM. This stage requires
// an LLM service connection and should only run in e2e mode.
// Input: JSON array of classified DiffChunk.
// Output: plain text commit message (NOT JSON).
func Stage06LLM(input []byte, deps StageDeps) ([]byte, error) {
	if deps.LLM == nil {
		return nil, fmt.Errorf("stage 06: LLM port is required")
	}
	var chunks []domain.DiffChunk
	if err := json.Unmarshal(input, &chunks); err != nil {
		return nil, fmt.Errorf("stage 06: unmarshal chunks: %w", err)
	}

	// Use the first chunk that has files to generate a message
	for _, chunk := range chunks {
		if len(chunk.Files) > 0 {
			message, err := deps.LLM.GenerateChunkMessage(chunk)
			if err != nil {
				return nil, fmt.Errorf("stage 06: generate message: %w", err)
			}
			return []byte(message), nil
		}
	}
	return nil, fmt.Errorf("stage 06: no chunks with files found")
}

// Stage07Result assembles the final pipeline result. This is a pure stage.
// Input: plain text message from Stage 6.
// Output: JSON PipelineResult.
func Stage07Result(input []byte, deps StageDeps) ([]byte, error) {
	// Stage 7 takes the message and assembles the final result.
	// In a real pipeline, we'd also have the chunks and security info,
	// but for golden file testing we only need the message.
	message := string(input)

	result := PipelineResult{
		Message: strings.TrimSpace(message),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("stage 07: marshal result: %w", err)
	}
	return data, nil
}

// extractFilesFromDiff is a helper that pulls filenames from a unified diff.
func extractFilesFromDiff(diff string) []string {
	var files []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			// Strip prefix after +++ or ---
			// +++ b/file.go → file.go
			// --- a/file.go → file.go
			name := line[4:]
			name = strings.TrimPrefix(name, "a/")
			name = strings.TrimPrefix(name, "b/")
			name = strings.TrimSpace(name)
			if name != "" && name != "/dev/null" && !seen[name] {
				seen[name] = true
				files = append(files, name)
			}
		}
	}
	return files
}