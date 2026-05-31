//go:build manual

package chunkers_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/infra/classifier"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
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

	t.Log("PIPELINE COMPLETO: diff → chunker → annotate → merge → LLM")

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

	// ─── 2-5. Por cada chunk: annotate → merge → classify → LLM ──────────
	annotator := chunkers.NewUnifiedASTPass(chunkers.NewLanguageCatalog())
	cl := classifier.NewClassifier(nil)
	files := map[string][2][]byte{
		"handler.go":      {handlerBefore, handlerAfter},
		"handler_test.go": {testBefore, testAfter},
	}

	llm := testutil.RequireLLM(t)
	if adapter, ok := llm.(*openai_standard.OpenAIStandardAdapter); ok {
		adapter.SetContext("git-courer project")
	}

	for ci := range chunks {
		chunk := &chunks[ci]

		for _, fname := range chunk.Files {
			fc := files[fname]
			annotator.Annotate(chunk, fname, fc[0], fc[1])
		}

		chunkers.MergeDiffIntoAnnotations(chunk, diff)

		commitType, confidence := cl.Classify(chunk)

		assert.NotEmpty(t, commitType, "chunk %d should have a commit type", ci)

		t.Logf("  CHUNK %d: %v", ci, chunk.Files)
		t.Logf("  Classifier: %s (%.0f%%)", commitType, confidence*100)
		if chunk.Scope != "" {
			t.Logf("  Scope: %s", chunk.Scope)
		}

		// Show type next to each label in the output for verification
		annotatedWithType := strings.ReplaceAll(chunk.AnnotatedDiff, "\n[", "\n"+commitType+" [")

		commitMsg, err := llm.GenerateChunkMessage(*chunk)
		if err != nil {
			t.Skipf("LLM error (skipping test): %v", err)
			return
		}

		t.Logf("  AnnotatedDiff (input al LLM):\n%s", annotatedWithType)
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

	annotator := chunkers.NewUnifiedASTPass(chunkers.NewLanguageCatalog())
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

		for _, fc := range fileContents {
			annotator.Annotate(chunk, fc.Filename, fc.Before, fc.After)
		}

		chunkers.MergeDiffIntoAnnotations(chunk, diff)

		commitType, confidence := cl.Classify(chunk)

		assert.NotEmpty(t, commitType, "chunk %d should have a commit type", ci)

		t.Logf("  CHUNK %d: %v", ci, chunk.Files)
		t.Logf("  Classifier: %s (%.0f%%)", commitType, confidence*100)
		if chunk.Scope != "" {
			t.Logf("  Scope: %s", chunk.Scope)
		}

		annotatedWithType := strings.ReplaceAll(chunk.AnnotatedDiff, "\n[", "\n"+commitType+" [")

		commitMsg, err := llm.GenerateChunkMessage(*chunk)
		if err != nil {
			t.Skipf("LLM error (skipping test): %v", err)
			return
		}

		t.Logf("  AnnotatedDiff (input al LLM):\n%s", annotatedWithType)
		t.Logf("  Respuesta del LLM:\n%s", commitMsg)
	}
}
