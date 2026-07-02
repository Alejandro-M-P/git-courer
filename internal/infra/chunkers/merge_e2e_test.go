//go:build manual

package chunkers_test

import (
	"os/exec"
	"testing"

	"github.com/blak0p/git-courer/internal/adapters/llm/openai_standard"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/infra/chunkers"
	"github.com/blak0p/git-courer/internal/infra/classifier"
	"github.com/blak0p/git-courer/internal/shared/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMergeE2E_RealDiff(t *testing.T) {
	// Un solo diff con 2 archivos: handler.go (código) + handler_test.go (test)
	diff := "" +
		"diff --git a/handler.go b/handler.go\n" +
		"--- a/handler.go\n" +
		"+++ b/handler.go\n" +
		"@@ -1,5 +1,12 @@\n" +
		" package main\n" +
		" \n" +
		" func HandleRequest(w http.ResponseWriter, r *http.Request) {\n" +
		"-	respondJSON(w, 200, \"ok\")\n" +
		"+	if r.Method != http.MethodGet {\n" +
		"+		respondError(w, 405, \"method not allowed\")\n" +
		"+		return\n" +
		"+	}\n" +
		"+	respondJSON(w, 200, \"ok\")\n" +
		" }\n" +
		"+func respondError(w http.ResponseWriter, code int, msg string) {\n" +
		"+	json.NewEncoder(w).Encode(map[string]string{\"error\": msg})\n" +
		"+}\n" +
		"diff --git a/handler_test.go b/handler_test.go\n" +
		"--- a/handler_test.go\n" +
		"+++ b/handler_test.go\n" +
		"@@ -1,3 +1,9 @@\n" +
		" package main\n" +
		" \n" +
		" func TestHandleRequest(t *testing.T) {\n" +
		"+	req := httptest.NewRequest(http.MethodGet, \"/\", nil)\n" +
		"+	rr := httptest.NewRecorder()\n" +
		"+	HandleRequest(rr, req)\n" +
		"+	if rr.Code != 200 {\n" +
		"+		t.Fatalf(\"expected 200, got %d\", rr.Code)\n" +
		"+	}\n" +
		" }\n"

	handlerBefore := []byte("package main\n\nfunc HandleRequest(w http.ResponseWriter, r *http.Request) {\n\trespondJSON(w, 200, \"ok\")\n}\n")
	handlerAfter := []byte("package main\n\nfunc HandleRequest(w http.ResponseWriter, r *http.Request) {\n\tif r.Method != http.MethodGet {\n\t\trespondError(w, 405, \"method not allowed\")\n\t\treturn\n\t}\n\trespondJSON(w, 200, \"ok\")\n}\nfunc respondError(w http.ResponseWriter, code int, msg string) {\n\tjson.NewEncoder(w).Encode(map[string]string{\"error\": msg})\n}\n")
	testBefore := []byte("package main\n\nfunc TestHandleRequest(t *testing.T) {\n}\n")
	testAfter := []byte("package main\n\nfunc TestHandleRequest(t *testing.T) {\n\treq := httptest.NewRequest(http.MethodGet, \"/\", nil)\n\trr := httptest.NewRecorder()\n\tHandleRequest(rr, req)\n\tif rr.Code != 200 {\n\t\tt.Fatalf(\"expected 200, got %d\", rr.Code)\n\t}\n}\n")

	t.Log("PIPELINE COMPLETO: diff → chunker → annotate → structured entries → LLM")

	// ─── 1. Chunker: separa en chunks ─────────────────────────────────────
	chunker := chunkers.NewDiffChunker()
	chunks, err := chunker.Chunk(diff, 4096)
	if err != nil {
		t.Fatal(err)
	}
	arePaired := chunker.GetLanguageCatalog().ArePaired("handler.go", "handler_test.go")
	assert.True(t, arePaired, "handler.go and handler_test.go should be paired")
	assert.Greater(t, len(chunks), 0, "should produce at least one chunk")

	t.Logf("  CHUNKER: ArePaired=%v, %d chunks", arePaired, len(chunks))
	for _, c := range chunks {
		t.Logf("       • %v", c.Files)
	}

	// ─── 2-5. Por cada chunk: annotate → classify → LLM ─────────────────
	// The new structured path: AnnotateWithContent populates chunk.AnnotatedEntries
	// (with hunk-only before/after per symbol) via buildAnnotatedEntries,
	// replacing the legacy emoji MergeDiffIntoAnnotations flow.
	annotator := chunkers.NewChunkAnnotatorAdapter(chunkers.NewLanguageCatalog())
	cl := classifier.NewClassifier(nil)

	llm := testutil.RequireLLM(t)
	if adapter, ok := llm.(*openai_standard.OpenAIStandardAdapter); ok {
		adapter.SetContext("git-courer project")
	}

	for ci := range chunks {
		chunk := &chunks[ci]

		fileContents := []ports.FileContent{
			{Filename: "handler.go", Before: handlerBefore, After: handlerAfter},
			{Filename: "handler_test.go", Before: testBefore, After: testAfter},
		}
		if err := annotator.AnnotateWithContent(chunk, fileContents, diff); err != nil {
			t.Logf("[WARN] AnnotateWithContent chunk %d: %v", ci, err)
		}

		commitType, confidence := cl.Classify(chunk)

		assert.NotEmpty(t, commitType, "chunk %d should have a commit type", ci)

		t.Logf("  CHUNK %d: %v", ci, chunk.Files)
		t.Logf("  Classifier: %s (%.0f%%)", commitType, confidence*100)
		if chunk.Scope != "" {
			t.Logf("  Scope: %s", chunk.Scope)
		}
		t.Logf("  AnnotatedEntries (%d):", len(chunk.AnnotatedEntries))
		for _, e := range chunk.AnnotatedEntries {
			t.Logf("    %s [%s] line=%d", e.Symbol, e.Type, e.Line)
		}

		commitMsg, err := llm.GenerateChunkMessage(*chunk)
		if err != nil {
			t.Skipf("LLM error (skipping test): %v", err)
			return
		}

		t.Logf("  AnnotatedDiff (legacy, input al LLM):\n%s", chunk.AnnotatedDiff)
		t.Logf("  Respuesta del LLM:\n%s", commitMsg)
	}
}

func TestMergeE2E_RealRepoDiff(t *testing.T) {
	diffBytes, err := exec.Command("git", "diff").Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	diff := string(diffBytes)
	if diff == "" {
		t.Fatal("no unstaged changes to test")
	}

	t.Logf("PIPELINE CON DIFF REAL DEL REPO (%d bytes)", len(diff))

	chunker := chunkers.NewDiffChunker()
	chunks, err := chunker.Chunk(diff, 8192)
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, len(chunks), 0, "should produce at least one chunk")
	t.Logf("  CHUNKER: %d chunks", len(chunks))
	for _, c := range chunks {
		t.Logf("       • %v", c.Files)
	}

	annotator := chunkers.NewChunkAnnotatorAdapter(chunkers.NewLanguageCatalog())
	cl := classifier.NewClassifier(nil)
	contentProvider := testutil.NewMockContentProvider()

	llm := testutil.RequireLLM(t)
	if adapter, ok := llm.(*openai_standard.OpenAIStandardAdapter); ok {
		adapter.SetContext("git-courer project")
	}

	for ci := range chunks {
		chunk := &chunks[ci]
		if len(chunk.Files) == 0 {
			continue
		}

		fileContents, err := contentProvider.GetContents(chunk.Files)
		if err != nil {
			t.Logf("  [WARN] chunk %d: GetContents: %v", ci, err)
			continue
		}

		// New structured path: AnnotateWithContent populates AnnotatedEntries
		// with hunk-only before/after per symbol, replacing MergeDiffIntoAnnotations.
		if err := annotator.AnnotateWithContent(chunk, fileContents, diff); err != nil {
			t.Logf("[WARN] AnnotateWithContent chunk %d: %v", ci, err)
		}

		commitType, confidence := cl.Classify(chunk)

		assert.NotEmpty(t, commitType, "chunk %d should have a commit type", ci)

		t.Logf("  CHUNK %d: %v", ci, chunk.Files)
		t.Logf("  Classifier: %s (%.0f%%)", commitType, confidence*100)
		if chunk.Scope != "" {
			t.Logf("  Scope: %s", chunk.Scope)
		}
		t.Logf("  AnnotatedEntries (%d):", len(chunk.AnnotatedEntries))
		for _, e := range chunk.AnnotatedEntries {
			t.Logf("    %s [%s] line=%d", e.Symbol, e.Type, e.Line)
		}

		commitMsg, err := llm.GenerateChunkMessage(*chunk)
		if err != nil {
			t.Skipf("LLM error (skipping test): %v", err)
			return
		}

		t.Logf("  AnnotatedDiff (legacy, input al LLM):\n%s", chunk.AnnotatedDiff)
		t.Logf("  Respuesta del LLM:\n%s", commitMsg)
	}
}
