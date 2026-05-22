// Package pipeline provides file-based pipeline testing infrastructure for
// the commit workflow. Each stage is a standalone function that reads input,
// processes it, and writes output — making the full data flow inspectable.
package pipeline

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// StageDeps holds port references needed by I/O stages (1, 2, 6).
// Pure stages (3, 4, 5, 7) receive these but may not use all fields.
type StageDeps struct {
	Git             ports.Git
	Security        ports.SecurityService
	Chunker         ports.DiffChunker
	Annotator       ports.ChunkAnnotator
	Classifier      ports.MessageClassifier
	LLM             ports.LLM
	ContentProvider ports.ContentProvider
	ChunkSize       int
}

// CommitRequest captures the full MCP handler request payload.
// This is the input to the pipeline (Stage 0).
type CommitRequest struct {
	Instruction string `json:"instruction"`
	Preview     bool   `json:"preview"`
}

// SecurityResult holds the security check result for pipeline serialization.
type SecurityResult struct {
	Blocked bool              `json:"blocked"`
	Files   []FileBlockResult `json:"files,omitempty"`
}

// FileBlockResult represents a single security finding for pipeline serialization.
type FileBlockResult struct {
	File    string `json:"file"`
	Reason  string `json:"reason"`
	Type    string `json:"type"`
	Halted  bool   `json:"halted"`
	Message string `json:"message,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// PipelineResult holds the final assembled output of the pipeline (Stage 7).
type PipelineResult struct {
	Message     string           `json:"message"`
	Chunks      []domain.DiffChunk `json:"chunks"`
	Security    SecurityResult   `json:"security"`
	Instruction string           `json:"instruction"`
	Preview     bool             `json:"preview"`
}