//go:build integration

package workflow

import (
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/infra/chunkers"
	"github.com/blak0p/git-courer/internal/infra/classifier"
	"github.com/blak0p/git-courer/internal/shared/testutil"
)

// newTestAnnotator creates an annotator and type helper for integration tests.
func newTestAnnotator() (ports.ChunkAnnotator, ports.CommitTypeHelper) {
	catalog := chunkers.NewLanguageCatalog()
	annotator := chunkers.NewChunkAnnotatorAdapter(catalog)
	typeHelper := classifier.NewCommitTypeHelperAdapter()
	return annotator, typeHelper
}

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

	annotator, typeHelper := newTestAnnotator()
	// Create a minimal commit service for testing
	svc := &CommitService{
		contentProvider: contentProvider,
		annotator:      annotator,
		typeHelper:     typeHelper,
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
	annotator, typeHelper := newTestAnnotator()

	svc := &CommitService{
		contentProvider: contentProvider,
		annotator:      annotator,
		typeHelper:     typeHelper,
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
	annotator, typeHelper := newTestAnnotator()

	svc := &CommitService{
		contentProvider: contentProvider,
		annotator:      annotator,
		typeHelper:     typeHelper,
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

// TestAnnotateChunks_PropagatesCFGDiff verifies that annotateChunks captures
// cfgDiff from ProcessWithContent and populates chunk.CFGBefore/CFGAfter
// when the CFG data is non-zero (i.e., for code files with grammars).
func TestAnnotateChunks_PropagatesCFGDiff(t *testing.T) {
	contentProvider := testutil.NewMockContentProvider()

	// Go file with a branch change between before and after
	contentProvider.AddFile("test.go",
		[]byte("package main\nfunc foo() {\nreturn\n}"),
		[]byte("package main\nfunc foo() {\nif x > 0 {\nreturn\n}\nreturn\n}"),
	)

	chunks := []domain.DiffChunk{
		{
			Files: []string{"test.go"},
			Diff:  "fake diff content",
		},
	}

	annotator, typeHelper := newTestAnnotator()
	svc := &CommitService{
		contentProvider: contentProvider,
		annotator:      annotator,
		typeHelper:     typeHelper,
	}

	err := svc.annotateChunks(chunks, chunks[0].Diff)
	if err != nil {
		t.Fatalf("annotateChunks failed: %v", err)
	}

	chunk := chunks[0]
	if chunk.CFGBefore == nil {
		t.Error("CFGBefore should not be nil for .go file with branch change")
	}
	if chunk.CFGAfter == nil {
		t.Error("CFGAfter should not be nil for .go file with branch change")
	}

	// After has more branches (if) than before
	if chunk.CFGAfter.Branch <= chunk.CFGBefore.Branch {
		t.Errorf("CFGAfter.Branch=%d should be > CFGBefore.Branch=%d (added an if)",
			chunk.CFGAfter.Branch, chunk.CFGBefore.Branch)
	}
}

// TestAnnotateChunks_NonCodeFile_NilCFG verifies that annotateChunks leaves
// chunk.CFGBefore/CFGAfter nil for non-code files (no CFG computation).
func TestAnnotateChunks_NonCodeFile_NilCFG(t *testing.T) {
	contentProvider := testutil.NewMockContentProvider()

	contentProvider.AddFile("config.json",
		[]byte(`{"key": "old"}`),
		[]byte(`{"key": "new"}`),
	)

	chunks := []domain.DiffChunk{
		{
			Files: []string{"config.json"},
			Diff:  "fake diff",
		},
	}

	annotator, typeHelper := newTestAnnotator()
	svc := &CommitService{
		contentProvider: contentProvider,
		annotator:      annotator,
		typeHelper:     typeHelper,
	}

	err := svc.annotateChunks(chunks, chunks[0].Diff)
	if err != nil {
		t.Fatalf("annotateChunks failed: %v", err)
	}

	chunk := chunks[0]
	if chunk.CFGBefore != nil {
		t.Errorf("CFGBefore should be nil for .json file, got %+v", chunk.CFGBefore)
	}
	if chunk.CFGAfter != nil {
		t.Errorf("CFGAfter should be nil for .json file, got %+v", chunk.CFGAfter)
	}
}

// ---------------------------------------------------------------------------
// annotateChunks OVERWRITE tests (Fix B: single source of truth)
// ---------------------------------------------------------------------------

// TestAnnotateChunks_OverwritesNotAppends verifies that annotateChunks
// OVERWRITES AnnotatedDiff instead of appending to it.
// If Process() pre-populated AnnotatedDiff with generic labels, annotateChunks
// must replace them entirely with entity-level labels from ProcessWithContent.
func TestAnnotateChunks_OverwritesNotAppends(t *testing.T) {
	contentProvider := testutil.NewMockContentProvider()

	// Go file with a new function added — annotateChunks will find NEW_FUNC
	contentProvider.AddFile("handler.go",
		[]byte("package main\nfunc Existing() {}\n"),
		[]byte("package main\nfunc Existing() {}\nfunc NewHelper() {}\n"),
	)

	chunks := []domain.DiffChunk{
		{
			Files:         []string{"handler.go"},
			Diff:          "fake diff",
		AnnotatedDiff: "📄 handler.go\nhandler.go [MOD_BODY] handler.go:0\n", // generic label from Process()
	}

	annotator, typeHelper := newTestAnnotator()
	svc := &CommitService{
		contentProvider: contentProvider,
		annotator:      annotator,
		typeHelper:     typeHelper,
	}

	err := svc.annotateChunks(chunks, chunks[0].Diff)
	if err != nil {
		t.Fatalf("annotateChunks failed: %v", err)
	}

	// The generic MOD_BODY label must NOT appear in the result.
	// Generic labels have no entity name and line 0, e.g. "handler.go [MOD_BODY] handler.go:0"
	// Entity-level labels from ProcessWithContent have entity names, e.g. "Existing [MOD_BODY] handler.go:2"
	if strings.Contains(chunks[0].AnnotatedDiff, "handler.go [MOD_BODY]") {
		t.Errorf("AnnotatedDiff should not contain generic MOD_BODY label (no entity name), got: %q", chunks[0].AnnotatedDiff)
	}

	// The result should contain entity-level labels from ProcessWithContent
	if !strings.Contains(chunks[0].AnnotatedDiff, "📄") {
		t.Errorf("AnnotatedDiff should contain file header emoji, got: %q", chunks[0].AnnotatedDiff)
	}
}

// TestAnnotateChunks_EmptyLabelsProducesEmptyString verifies that when
// annotateChunks produces no labels for a chunk (e.g., content provider fails
// for all files), the AnnotatedDiff is set to empty string — overwriting any
// pre-existing content.
func TestAnnotateChunks_EmptyLabelsProducesEmptyString(t *testing.T) {
	contentProvider := testutil.NewMockContentProvider()

	// Don't add any files to the content provider — GetContents returns empty,
	// so no labels are produced for this chunk.
	chunks := []domain.DiffChunk{
		{
			Files:         []string{"handler.go"},
			Diff:          "fake diff",
			AnnotatedDiff: "📄 handler.go\nhandler.go [MOD_BODY] handler.go:0\n", // pre-populated generic
		},
	}

	annotator, typeHelper := newTestAnnotator()
	svc := &CommitService{
		contentProvider: contentProvider,
		annotator:      annotator,
		typeHelper:     typeHelper,
	}

	err := svc.annotateChunks(chunks, chunks[0].Diff)
	if err != nil {
		t.Fatalf("annotateChunks failed: %v", err)
	}

	// No content returned → no labels produced → AnnotatedDiff should be empty (overwritten)
	if chunks[0].AnnotatedDiff != "" {
		t.Errorf("AnnotatedDiff should be empty when no content available, got: %q", chunks[0].AnnotatedDiff)
	}
}