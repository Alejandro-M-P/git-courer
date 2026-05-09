package workflow

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

// TestAnnotateChunks_Integration tests the end-to-end annotation integration
// with mock content provider and AST annotator.
func TestAnnotateChunks_Integration(t *testing.T) {
	contentProvider := testutil.NewMockContentProvider()
	contentProvider.AddFile("go.mod", []byte("module foo"), []byte("module bar"))

	// Test with a chunk containing a real Go file
	chunks := []domain.DiffChunk{
		{
			Files: []string{"go.mod"},
			Diff:  "test diff content",
		},
	}

	// Create a minimal commit service for testing
	svc := &CommitService{
		contentProvider: contentProvider,
		unifiedPass:     chunkers.NewUnifiedASTPass(chunkers.NewLanguageCatalog()),
	}

	// Test the annotation
	err := svc.annotateChunks(chunks, chunks[0].Diff)
	if err != nil {
		t.Fatalf("annotateChunks failed: %v", err)
	}

	// Verify that AnnotatedDiff was populated
	chunk := chunks[0]
	if chunk.AnnotatedDiff == "" {
		t.Error("Expected AnnotatedDiff to be populated, got empty string")
	}

	// Should contain the file header and semantic label
	if !strings.Contains(chunk.AnnotatedDiff, "📄") {
		t.Error("Expected AnnotatedDiff to contain file header emoji")
	}

	if !strings.Contains(chunk.AnnotatedDiff, "go.mod") {
		t.Error("Expected AnnotatedDiff to contain filename")
	}

	// For go.mod, it should be labeled as [DEPS]
	if !strings.Contains(chunk.AnnotatedDiff, "[DEPS]") {
		t.Error("Expected go.mod to be labeled as [DEPS]")
	}
}

// TestAnnotateChunks_EmptyChunks tests that empty chunks are handled gracefully
func TestAnnotateChunks_EmptyChunks(t *testing.T) {
	contentProvider := testutil.NewMockContentProvider()

	svc := &CommitService{
		contentProvider: contentProvider,
		unifiedPass:     chunkers.NewUnifiedASTPass(chunkers.NewLanguageCatalog()),
	}

	// Test with empty chunks slice
	err := svc.annotateChunks(nil, "")
	if err != nil {
		t.Fatalf("annotateChunks with nil should not fail: %v", err)
	}

	// Test with empty chunk
	emptyChunks := []domain.DiffChunk{{}}
	err = svc.annotateChunks(emptyChunks, "")
	if err != nil {
		t.Fatalf("annotateChunks with empty chunk should not fail: %v", err)
	}

	// Test with chunk that has no files
	noFilesChunks := []domain.DiffChunk{{
		Files: []string{},
		Diff:  "test",
	}}
	err = svc.annotateChunks(noFilesChunks, "")
	if err != nil {
		t.Fatalf("annotateChunks with no files should not fail: %v", err)
	}
}

// TestAnnotateChunks_ErrorHandling tests that errors are handled gracefully
func TestAnnotateChunks_ErrorHandling(t *testing.T) {
	contentProvider := testutil.NewMockContentProvider()

	svc := &CommitService{
		contentProvider: contentProvider,
		unifiedPass:     chunkers.NewUnifiedASTPass(chunkers.NewLanguageCatalog()),
	}

	// Test with non-existent file (should error but not fail entire operation)
	chunks := []domain.DiffChunk{
		{
			Files: []string{"non_existent_file_12345.go"},
			Diff:  "test diff",
		},
	}

	err := svc.annotateChunks(chunks, chunks[0].Diff)
	// This should not fail completely - individual file errors should be logged but operation should continue
	if err != nil {
		t.Fatalf("annotateChunks should handle individual file errors gracefully: %v", err)
	}
}