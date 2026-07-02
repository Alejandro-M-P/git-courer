package chunkers

import (
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/shared/testutil"
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

// TestChunkAnnotatorAdapter_AnnotateWithContent_PopulatesAnnotatedEntries
// verifies that AnnotateWithContent populates chunk.AnnotatedEntries (the new
// structured typed path) alongside the legacy chunk.AnnotatedDiff when the
// diff contains a grammar-supported code file with a real symbol change.
func TestChunkAnnotatorAdapter_AnnotateWithContent_PopulatesAnnotatedEntries(t *testing.T) {
	t.Parallel()

	catalog := NewLanguageCatalog()
	adapter := NewChunkAnnotatorAdapter(catalog)

	goBefore := []byte("package main\n\nfunc Old() int { return 1 }\n")
	goAfter := []byte("package main\n\nfunc New() int { return 2 }\n")

	rawDiff := "diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,3 +1,3 @@\n" +
		" package main\n" +
		" \n" +
		"-func Old() int { return 1 }\n" +
		"+func New() int { return 2 }\n"

	files := []ports.FileContent{
		{Filename: "main.go", Before: goBefore, After: goAfter},
	}

	chunk := &domain.DiffChunk{
		Files: []string{"main.go"},
		Diff:  rawDiff,
	}

	if err := adapter.AnnotateWithContent(chunk, files, rawDiff); err != nil {
		t.Fatalf("AnnotateWithContent failed: %v", err)
	}

	if len(chunk.AnnotatedEntries) == 0 {
		t.Fatal("AnnotatedEntries should be populated for a Go symbol change")
	}

	// Entries must reference main.go and carry no emoji in any field.
	for _, e := range chunk.AnnotatedEntries {
		if e.File != "main.go" {
			t.Errorf("entry %q File = %q, want main.go", e.Symbol, e.File)
		}
		for _, r := range e.File + e.Symbol + e.Type + e.Before + e.After {
			if r >= 0x1F000 {
				t.Errorf("entry %q contains emoji rune %U; fields must be emoji-free", e.Symbol, r)
			}
		}
	}

	// Legacy AnnotatedDiff must still be populated (backward compat).
	if chunk.AnnotatedDiff == "" {
		t.Error("AnnotatedDiff should still be populated for backward compat")
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

// TestChunkAnnotatorAdapter_CFGAccumulation verifies CFG accumulation and nil preservation.
func TestChunkAnnotatorAdapter_CFGAccumulation(t *testing.T) {
	t.Parallel()

	t.Run("multi-file chunk CFG accumulation", func(t *testing.T) {
		catalog := NewLanguageCatalog()
		adapter := NewChunkAnnotatorAdapter(catalog)

		file1Before := []byte("package main\nfunc a() {\n\tif x {\n\t\treturn\n\t}\n}")
		file1After := []byte("package main\nfunc a() {\n\tif x {\n\t\treturn\n\t}\n\tif y {\n\t\treturn\n\t}\n}")

		file2Before := []byte("package main\nfunc b() {\n}")
		file2After := []byte("package main\nfunc b() {\n\tfor {\n\t\tif z {\n\t\t\tbreak\n\t\t}\n\t}\n}")

		files := []ports.FileContent{
			{Filename: "file1.go", Before: file1Before, After: file1After},
			{Filename: "file2.go", Before: file2Before, After: file2After},
		}

		chunk := &domain.DiffChunk{
			Files: []string{"file1.go", "file2.go"},
			Diff:  "fake diff",
		}

		err := adapter.AnnotateWithContent(chunk, files, chunk.Diff)
		if err != nil {
			t.Fatalf("AnnotateWithContent failed: %v", err)
		}

		if chunk.CFGBefore == nil {
			t.Fatal("CFGBefore should not be nil")
		}
		if chunk.CFGAfter == nil {
			t.Fatal("CFGAfter should not be nil")
		}

		_, file1Diff, err := adapter.unifiedPass.ProcessWithContent("file1.go", file1Before, file1After, nil)
		if err != nil {
			t.Fatalf("ProcessWithContent file1 failed: %v", err)
		}
		_, file2Diff, err := adapter.unifiedPass.ProcessWithContent("file2.go", file2Before, file2After, nil)
		if err != nil {
			t.Fatalf("ProcessWithContent file2 failed: %v", err)
		}

		expectedBefore := domain.CFGCount{
			Branch: file1Diff.Before.Branch + file2Diff.Before.Branch,
			Loop:   file1Diff.Before.Loop + file2Diff.Before.Loop,
			Return: file1Diff.Before.Return + file2Diff.Before.Return,
			Error:  file1Diff.Before.Error + file2Diff.Before.Error,
		}

		expectedAfter := domain.CFGCount{
			Branch: file1Diff.After.Branch + file2Diff.After.Branch,
			Loop:   file1Diff.After.Loop + file2Diff.After.Loop,
			Return: file1Diff.After.Return + file2Diff.After.Return,
			Error:  file1Diff.After.Error + file2Diff.After.Error,
		}

		if *chunk.CFGBefore != expectedBefore {
			t.Errorf("CFGBefore mismatch. Got %+v, want %+v", *chunk.CFGBefore, expectedBefore)
		}
		if *chunk.CFGAfter != expectedAfter {
			t.Errorf("CFGAfter mismatch. Got %+v, want %+v", *chunk.CFGAfter, expectedAfter)
		}
	})

	t.Run("zero CFG nil preservation", func(t *testing.T) {
		catalog := NewLanguageCatalog()
		adapter := NewChunkAnnotatorAdapter(catalog)

		file1Before := []byte("package main")
		file1After := []byte("package main")
		file2Before := []byte("package main")
		file2After := []byte("package main")

		files := []ports.FileContent{
			{Filename: "file1.go", Before: file1Before, After: file1After},
			{Filename: "file2.go", Before: file2Before, After: file2After},
		}

		chunk := &domain.DiffChunk{
			Files: []string{"file1.go", "file2.go"},
			Diff:  "fake diff",
		}

		err := adapter.AnnotateWithContent(chunk, files, chunk.Diff)
		if err != nil {
			t.Fatalf("AnnotateWithContent failed: %v", err)
		}

		if chunk.CFGBefore != nil {
			t.Errorf("CFGBefore should be nil when all files have zero CFG, got %+v", chunk.CFGBefore)
		}
		if chunk.CFGAfter != nil {
			t.Errorf("CFGAfter should be nil when all files have zero CFG, got %+v", chunk.CFGAfter)
		}
	})

	t.Run("one file with CFG and one without", func(t *testing.T) {
		catalog := NewLanguageCatalog()
		adapter := NewChunkAnnotatorAdapter(catalog)

		file1Before := []byte("package main\nfunc a() {\n\tif x {\n\t\treturn\n\t}\n}")
		file1After := []byte("package main\nfunc a() {\n\tif x {\n\t\treturn\n\t}\n}")
		file2Before := []byte("key: value")
		file2After := []byte("key: newValue")

		files := []ports.FileContent{
			{Filename: "file1.go", Before: file1Before, After: file1After},
			{Filename: "config.yaml", Before: file2Before, After: file2After},
		}

		chunk := &domain.DiffChunk{
			Files: []string{"file1.go", "config.yaml"},
			Diff:  "fake diff",
		}

		err := adapter.AnnotateWithContent(chunk, files, chunk.Diff)
		if err != nil {
			t.Fatalf("AnnotateWithContent failed: %v", err)
		}

		if chunk.CFGBefore == nil || chunk.CFGAfter == nil {
			t.Fatal("CFG count pointers should not be nil")
		}

		_, file1Diff, err := adapter.unifiedPass.ProcessWithContent("file1.go", file1Before, file1After, nil)
		if err != nil {
			t.Fatalf("ProcessWithContent file1 failed: %v", err)
		}

		if *chunk.CFGBefore != file1Diff.Before {
			t.Errorf("CFGBefore should equal file1 before counts, got %+v, want %+v", *chunk.CFGBefore, file1Diff.Before)
		}
		if *chunk.CFGAfter != file1Diff.After {
			t.Errorf("CFGAfter should equal file1 after counts, got %+v, want %+v", *chunk.CFGAfter, file1Diff.After)
		}
	})
}
