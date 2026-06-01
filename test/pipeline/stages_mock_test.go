//go:build !e2e

package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// === Stage 03: Chunking ===

func TestStage03Chunks_ValidInputWithMockChunker(t *testing.T) {
	t.Parallel()
	expectedChunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "+added line", CommitType: "", ConfidenceScore: 0},
		{Files: []string{"b.go"}, Diff: "+another line", CommitType: "", ConfidenceScore: 0},
	}
	mockChunker := &MockChunker{
		ChunkReturns: struct {
			Chunks []domain.DiffChunk
			Err    error
		}{Chunks: expectedChunks, Err: nil},
	}

	deps := StageDeps{Chunker: mockChunker}
	input := []byte("some diff content")

	output, err := Stage03Chunks(input, deps)
	if err != nil {
		t.Fatalf("Stage03Chunks: %v", err)
	}

	if !mockChunker.ChunkCalled {
		t.Error("Chunk was not called")
	}
	if mockChunker.ChunkCalledWith.Diff != "some diff content" {
		t.Errorf("Chunk called with diff = %q, want %q", mockChunker.ChunkCalledWith.Diff, "some diff content")
	}

	var result []domain.DiffChunk
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d chunks, want 2", len(result))
	}
}

func TestStage03Chunks_NilChunkerPortReturnsError(t *testing.T) {
	t.Parallel()
	deps := StageDeps{Chunker: nil}
	_, err := Stage03Chunks([]byte("diff"), deps)
	if err == nil {
		t.Fatal("expected error when DiffChunker port is nil")
	}
	if got := err.Error(); got != "stage 03: DiffChunker port is required" {
		t.Errorf("error = %q, want \"stage 03: DiffChunker port is required\"", got)
	}
}

func TestStage03Chunks_ChunkerErrorPropagated(t *testing.T) {
	t.Parallel()
	mockChunker := &MockChunker{
		ChunkReturns: struct {
			Chunks []domain.DiffChunk
			Err    error
		}{Err: errTestChunkerFailure},
	}

	deps := StageDeps{Chunker: mockChunker}
	_, err := Stage03Chunks([]byte("diff"), deps)
	if err == nil {
		t.Fatal("expected error when Chunker returns error")
	}
	if got := err.Error(); got != "stage 03: chunk: "+errTestChunkerFailure.Error() {
		t.Errorf("error = %q, want wrapped chunker error", got)
	}
}

// === Stage 04: Annotation ===

func TestStage04Annotation_ValidInputWithMockAnnotator(t *testing.T) {
	t.Parallel()
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "+added", CommitType: "feat"},
	}
	input, _ := json.Marshal(chunks)

	mockAnnotator := &MockAnnotator{
		AnnotateWithContentReturns: struct {
			Err error
		}{Err: nil},
	}
	mockContentProvider := &testContentProvider{
		files: map[string]ports.FileContent{
			"a.go": {Filename: "a.go", Before: []byte("old"), After: []byte("new")},
		},
	}

	deps := StageDeps{
		Annotator:       mockAnnotator,
		ContentProvider: mockContentProvider,
	}

	output, err := Stage04Annotation(input, deps)
	if err != nil {
		t.Fatalf("Stage04Annotation: %v", err)
	}
	if !mockAnnotator.AnnotateWithContentCalled {
		t.Error("AnnotateWithContent was not called")
	}

	var result []domain.DiffChunk
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d chunks, want 1", len(result))
	}
}

func TestStage04Annotation_NilAnnotatorReturnsError(t *testing.T) {
	t.Parallel()
	input, _ := json.Marshal([]domain.DiffChunk{{Files: []string{"a.go"}, Diff: "+added"}})
	deps := StageDeps{Annotator: nil, ContentProvider: &testContentProvider{}}
	_, err := Stage04Annotation(input, deps)
	if err == nil {
		t.Fatal("expected error when ChunkAnnotator port is nil")
	}
	if got := err.Error(); got != "stage 04: ChunkAnnotator port is required" {
		t.Errorf("error = %q, want \"stage 04: ChunkAnnotator port is required\"", got)
	}
}

func TestStage04Annotation_NilContentProviderReturnsError(t *testing.T) {
	t.Parallel()
	input, _ := json.Marshal([]domain.DiffChunk{{Files: []string{"a.go"}, Diff: "+added"}})
	deps := StageDeps{
		Annotator:       &MockAnnotator{},
		ContentProvider: nil,
	}
	_, err := Stage04Annotation(input, deps)
	if err == nil {
		t.Fatal("expected error when ContentProvider port is nil")
	}
	if got := err.Error(); got != "stage 04: ContentProvider port is required" {
		t.Errorf("error = %q, want \"stage 04: ContentProvider port is required\"", got)
	}
}

// === Stage 05: Classification ===

func TestStage05Classification_ValidInputWithMockClassifier(t *testing.T) {
	t.Parallel()
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "+added", CommitType: "", ConfidenceScore: 0},
	}
	input, _ := json.Marshal(chunks)

	mockClassifier := &MockClassifier{
		ClassifyReturns: struct {
			CommitType string
			Confidence float64
		}{CommitType: "feat", Confidence: 0.9},
	}

	deps := StageDeps{Classifier: mockClassifier}
	output, err := Stage05Classification(input, deps)
	if err != nil {
		t.Fatalf("Stage05Classification: %v", err)
	}

	if !mockClassifier.ClassifyCalled {
		t.Error("Classify was not called")
	}

	var result []domain.DiffChunk
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result[0].CommitType != "feat" {
		t.Errorf("CommitType = %q, want %q", result[0].CommitType, "feat")
	}
	if result[0].ConfidenceScore != 0.9 {
		t.Errorf("ConfidenceScore = %f, want 0.9", result[0].ConfidenceScore)
	}
}

func TestStage05Classification_NilClassifierReturnsError(t *testing.T) {
	t.Parallel()
	input, _ := json.Marshal([]domain.DiffChunk{{Files: []string{"a.go"}, Diff: "+added"}})
	deps := StageDeps{Classifier: nil}
	_, err := Stage05Classification(input, deps)
	if err == nil {
		t.Fatal("expected error when MessageClassifier port is nil")
	}
	if got := err.Error(); got != "stage 05: MessageClassifier port is required" {
		t.Errorf("error = %q, want \"stage 05: MessageClassifier port is required\"", got)
	}
}

func TestStage05Classification_InvalidJSONInputReturnsError(t *testing.T) {
	t.Parallel()
	mockClassifier := &MockClassifier{
		ClassifyReturns: struct {
			CommitType string
			Confidence float64
		}{CommitType: "feat", Confidence: 0.9},
	}
	deps := StageDeps{Classifier: mockClassifier}
	_, err := Stage05Classification([]byte("not json at all"), deps)
	if err == nil {
		t.Fatal("expected error for invalid JSON input")
	}
	// Should wrap the JSON error
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

// testContentProvider is a simple in-memory ContentProvider for testing.
// (separate from testutil.MockContentProvider to keep pipeline tests self-contained)
type testContentProvider struct {
	files map[string]ports.FileContent
}

func (t *testContentProvider) AddFile(filename string, before, after []byte) {
	if t.files == nil {
		t.files = make(map[string]ports.FileContent)
	}
	t.files[filename] = ports.FileContent{Filename: filename, Before: before, After: after}
}

func (t *testContentProvider) GetContents(filenames []string) ([]ports.FileContent, error) {
	var result []ports.FileContent
	for _, f := range filenames {
		if fc, ok := t.files[f]; ok {
			result = append(result, fc)
		}
	}
	return result, nil
}

// Sentinel error for test assertions
var errTestChunkerFailure = errSentinel("chunker failure")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
