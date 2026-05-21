package chunkers

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

// TestDiffChunker_ImplementsCatalogProvider verifies DiffChunker satisfies the CatalogProvider port.
func TestDiffChunker_ImplementsCatalogProvider(t *testing.T) {
	t.Parallel()

	// Compile-time interface satisfaction check
	var _ ports.CatalogProvider = (*DiffChunker)(nil)

	chunker := NewDiffChunker()
	catalog := chunker.GetLanguageCatalog()
	if catalog == nil {
		t.Error("DiffChunker.GetLanguageCatalog() returned nil")
	}

	// The catalog should be able to identify test files
	if !catalog.IsTestFile("handler_test.go") {
		t.Error("Catalog from DiffChunker should identify Go test files")
	}
}

// TestChunkAnnotatorAdapter_ImplementsPort verifies ChunkAnnotatorAdapter satisfies ChunkAnnotator.
func TestChunkAnnotatorAdapter_ImplementsPort(t *testing.T) {
	t.Parallel()

	// Compile-time interface satisfaction check
	var _ ports.ChunkAnnotator = (*ChunkAnnotatorAdapter)(nil)
}

// TestChunkAnnotatorAdapter_AnnotateWithContent verifies AnnotateWithContent processes
// multiple files and populates chunk annotations.
func TestChunkAnnotatorAdapter_AnnotateWithContent(t *testing.T) {
	t.Parallel()

	catalog := NewLanguageCatalog()
	adapter := NewChunkAnnotatorAdapter(catalog)

	contentProvider := testutil.NewMockContentProvider()
	contentProvider.AddFile("go.mod", []byte("module foo"), []byte("module bar"))

	chunk := &domain.DiffChunk{
		Files: []string{"go.mod"},
		Diff:  "test diff",
	}

	files, _ := contentProvider.GetContents(chunk.Files)
	err := adapter.AnnotateWithContent(chunk, files, chunk.Diff)
	if err != nil {
		t.Fatalf("AnnotateWithContent failed: %v", err)
	}

	if chunk.AnnotatedDiff == "" {
		t.Error("AnnotatedDiff should be populated after AnnotateWithContent")
	}
}

// TestChunkAnnotatorAdapter_AnnotateWithContent_EmptyFiles verifies handling of empty file list.
func TestChunkAnnotatorAdapter_AnnotateWithContent_EmptyFiles(t *testing.T) {
	t.Parallel()

	catalog := NewLanguageCatalog()
	adapter := NewChunkAnnotatorAdapter(catalog)

	chunk := &domain.DiffChunk{Files: []string{}}
	err := adapter.AnnotateWithContent(chunk, nil, "")
	if err != nil {
		t.Fatalf("AnnotateWithContent with empty files should not fail: %v", err)
	}
}