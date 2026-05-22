//go:build !e2e

package pipeline

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// MockChunker implements ports.DiffChunker for testing.
type MockChunker struct {
	ChunkReturns struct {
		Chunks []domain.DiffChunk
		Err    error
	}
	ChunkCalled bool
	ChunkCalledWith struct {
		Diff         string
		MaxChunkSize int
	}
}

func (m *MockChunker) Chunk(diff string, maxChunkSize int) ([]domain.DiffChunk, error) {
	m.ChunkCalled = true
	m.ChunkCalledWith.Diff = diff
	m.ChunkCalledWith.MaxChunkSize = maxChunkSize
	return m.ChunkReturns.Chunks, m.ChunkReturns.Err
}

// MockAnnotator implements ports.ChunkAnnotator for testing.
type MockAnnotator struct {
	AnnotateReturns struct {
		Err error
	}
	AnnotateCalled bool
	AnnotateCalledWith struct {
		Chunk    *domain.DiffChunk
		Filename string
		Before   []byte
		After    []byte
	}

	AnnotateWithContentReturns struct {
		Err error
	}
	AnnotateWithContentCalled bool
	AnnotateWithContentCalledWith struct {
		Chunk   *domain.DiffChunk
		Files   []ports.FileContent
		RawDiff string
	}
}

func (m *MockAnnotator) Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error {
	m.AnnotateCalled = true
	m.AnnotateCalledWith.Chunk = chunk
	m.AnnotateCalledWith.Filename = filename
	m.AnnotateCalledWith.Before = before
	m.AnnotateCalledWith.After = after
	return m.AnnotateReturns.Err
}

func (m *MockAnnotator) AnnotateWithContent(chunk *domain.DiffChunk, files []ports.FileContent, rawDiff string) error {
	m.AnnotateWithContentCalled = true
	m.AnnotateWithContentCalledWith.Chunk = chunk
	m.AnnotateWithContentCalledWith.Files = files
	m.AnnotateWithContentCalledWith.RawDiff = rawDiff
	return m.AnnotateWithContentReturns.Err
}

// MockClassifier implements ports.MessageClassifier for testing.
type MockClassifier struct {
	ClassifyReturns struct {
		CommitType string
		Confidence float64
	}
	ClassifyCalled bool
	ClassifyCalledWith struct {
		Chunk *domain.DiffChunk
	}

	LearnFromHistoryReturns struct {
		Err error
	}
	LearnFromHistoryCalled bool
}

func (m *MockClassifier) Classify(chunk *domain.DiffChunk) (string, float64) {
	m.ClassifyCalled = true
	m.ClassifyCalledWith.Chunk = chunk
	return m.ClassifyReturns.CommitType, m.ClassifyReturns.Confidence
}

func (m *MockClassifier) LearnFromHistory() error {
	m.LearnFromHistoryCalled = true
	return m.LearnFromHistoryReturns.Err
}