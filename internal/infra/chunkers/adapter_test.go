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

// TestAnnotateWithContent_SharedASTPass verifies that ProcessWithContent is called
// exactly once per file, not once per pipeline stage. Before the shared-AST-pass
// refactor, AnnotateDiffForRead created its own UnifiedASTPass and called
// ProcessWithContent independently, causing double parsing per file.
func TestAnnotateWithContent_SharedASTPass(t *testing.T) {
	t.Parallel()

	catalog := NewLanguageCatalog()
	adapter := NewChunkAnnotatorAdapter(catalog)

	// Reset the process count before the test
	adapter.unifiedPass.ProcessCount.Store(0)

	// Create a minimal Go diff with one file so the AST path runs.
	goBefore := []byte("package main\nfunc old() int { return 1 }")
	goAfter := []byte("package main\nfunc new() int { return 2 }")

	rawDiff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
-package main
-func old() int { return 1 }
+func new() int { return 2 }
`

	files := []ports.FileContent{
		{Filename: "main.go", Before: goBefore, After: goAfter},
	}

	chunk := &domain.DiffChunk{
		Files: []string{"main.go"},
		Diff:  rawDiff,
	}

	err := adapter.AnnotateWithContent(chunk, files, rawDiff)
	if err != nil {
		t.Fatalf("AnnotateWithContent failed: %v", err)
	}

	// CRITICAL assertion: ProcessWithContent must be called exactly ONCE per file.
	// Before the shared-AST-pass fix, it was called TWICE (once in the adapter loop,
	// once in AnnotateDiffForRead's own astPass).
	got := adapter.unifiedPass.ProcessCount.Load()
	if got != 1 {
		t.Errorf("ProcessWithContent called %d times for 1 file, want exactly 1", got)
	}

	// Sanity: AnnotatedDiff should be populated.
	if chunk.AnnotatedDiff == "" {
		t.Error("AnnotatedDiff should be populated after AnnotateWithContent")
	}
}

// TestAnnotateWithContent_BeforeSourceAfterSource_WithCatalogFilter verifies that
// AnnotateWithContent populates BeforeSource/AfterSource based on LanguageCatalog
// grammar availability, not a hardcoded .go filter.
// - Go files (has grammar) → BeforeSource/AfterSource populated
// - Python files (has grammar) → BeforeSource/AfterSource populated
// - .yaml files (no grammar) → BeforeSource/AfterSource NOT populated
func TestAnnotateWithContent_BeforeSourceAfterSource_WithCatalogFilter(t *testing.T) {
	t.Parallel()

	catalog := NewLanguageCatalog()
	adapter := NewChunkAnnotatorAdapter(catalog)

	goBefore := []byte("package main\nfunc old() int { return 1 }")
	goAfter := []byte("package main\nfunc new() int { return 2 }")
	pyBefore := []byte("def old():\n    pass\n")
	pyAfter := []byte("def new():\n    pass\n")
	yamlBefore := []byte("key: old\n")
	yamlAfter := []byte("key: new\n")

	// Raw diff touching all three files
	rawDiff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
-package main
-func old() int { return 1 }
+func new() int { return 2 }
diff --git a/utils.py b/utils.py
--- a/utils.py
+++ b/utils.py
@@ -1,2 +1,2 @@
-def old():
+def new():
diff --git a/config.yaml b/config.yaml
--- a/config.yaml
+++ b/config.yaml
@@ -1,1 +1,1 @@
-key: old
+key: new
`

	files := []ports.FileContent{
		{Filename: "main.go", Before: goBefore, After: goAfter},
		{Filename: "utils.py", Before: pyBefore, After: pyAfter},
		{Filename: "config.yaml", Before: yamlBefore, After: yamlAfter},
	}

	chunk := &domain.DiffChunk{
		Files: []string{"main.go", "utils.py", "config.yaml"},
		Diff:  rawDiff,
	}

	err := adapter.AnnotateWithContent(chunk, files, rawDiff)
	if err != nil {
		t.Fatalf("AnnotateWithContent failed: %v", err)
	}

	// Go file: should have BeforeSource and AfterSource
	if _, ok := chunk.BeforeSource["main.go"]; !ok {
		t.Error("Go file should have BeforeSource entry")
	}
	if _, ok := chunk.AfterSource["main.go"]; !ok {
		t.Error("Go file should have AfterSource entry")
	}

	// Python file: should have BeforeSource and AfterSource (Python has a grammar)
	if _, ok := chunk.BeforeSource["utils.py"]; !ok {
		t.Error("Python file should have BeforeSource entry (grammar available in catalog)")
	}
	if _, ok := chunk.AfterSource["utils.py"]; !ok {
		t.Error("Python file should have AfterSource entry (grammar available in catalog)")
	}

	// YAML file: should NOT have BeforeSource or AfterSource (no grammar in catalog)
	if _, ok := chunk.BeforeSource["config.yaml"]; ok {
		t.Error("YAML file should NOT have BeforeSource entry (no grammar in catalog)")
	}
	if _, ok := chunk.AfterSource["config.yaml"]; ok {
		t.Error("YAML file should NOT have AfterSource entry (no grammar in catalog)")
	}
}