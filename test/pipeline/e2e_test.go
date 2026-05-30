//go:build e2e

package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/infra/classifier"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StageReport captures per-stage audit data: input bytes, output bytes, latency.
// Per vault "El Reporte Obligatorio", every e2e test MUST report these values.
type StageReport struct {
	Index     int
	Name      string
	InputLen  int
	OutputLen int
	Latency   time.Duration
	Err       error
}

// referenceScenarios are real diffs from git-courer's own history.
// Each entry maps a .diff file to a human-readable description.
// No expected commit type — the classifier uses heuristic inference (no git history),
// and the LLM-generated messages are the real output we care about.
var referenceScenarios = []struct {
	diffFile    string
	description string
}{
	{
		diffFile:    "reference/refactor_21_files.diff",
		description: "Dependency injection refactor (21 files)",
	},
	{
		diffFile:    "reference/feat_annotated_diff.diff",
		description: "AnnotateDiffForRead feature (~10 files)",
	},
	{
		diffFile:    "reference/feat_handler_wiring.diff",
		description: "Handler wiring for ContentProvider (~6 files)",
	},
	{
		diffFile:    "reference/fix_python_classifier.diff",
		description: "Python signature detection fix (5 files)",
	},
	{
		diffFile:    "reference/chore_preserve_best_type.diff",
		description: "Preserve best CommitType (~4 files)",
	},
	{
		diffFile:    "reference/breaking_config.diff",
		description: "Breaking config change (multiline)",
	},
	{
		diffFile:    "reference/docs_sdd_order.diff",
		description: "Docs + SDD order workflow",
	},
}

// TestE2EPipeline runs the pure stages (03-06) against real diffs
// from git-courer's own history. Stages 03-05 are pure (no external deps).
// Stage 06 uses the LLM to generate a commit message per chunk.
func TestE2EPipeline(t *testing.T) {
	catalog := chunkers.NewLanguageCatalog()
	chunker := chunkers.NewDiffChunker(
		chunkers.WithMaxFilesPerChunk(12),
		chunkers.WithMinForce(3),
	)
	annotator := chunkers.NewChunkAnnotatorAdapter(catalog)
	classifier := classifier.NewClassifierWithCatalog(nil, catalog) // nil git port — InferCommitType fallback handles it
	llm := testutil.RequireLLM(t) // requires running LLM service (LLM_MODEL env)
	// ContentProvider reads files from git-courer's own repo.
	// This gives AnnotateDiffForRead real source code for AST labels.
	contentProvider := git.NewGitContentProvider("/git-courer")

	for _, ref := range referenceScenarios {
		t.Run(ref.description, func(t *testing.T) {
			diffData, err := os.ReadFile(ref.diffFile)
			if err != nil {
				t.Fatalf("read %s: %v", ref.diffFile, err)
			}
			diff := string(diffData)

			auditDir := filepath.Join("audit", "ref_"+filepath.Base(ref.diffFile))
			os.RemoveAll(auditDir)
			os.MkdirAll(auditDir, 0o755)

			// Stage 03: Chunking
			stage03Start := time.Now()
			chunks, err := chunker.Chunk(diff, 4000)
			if err != nil {
				t.Fatalf("chunk %s: %v", ref.description, err)
			}
			stage03Duration := time.Since(stage03Start)
			if len(chunks) == 0 {
				t.Fatalf("chunk %s: no chunks produced", ref.description)
			}
			chunksJSON, _ := json.MarshalIndent(chunks, "", "  ")
			writeAuditFile(t, auditDir, "03_chunks.json", chunksJSON)
			writeAuditFile(t, auditDir, "03_chunks.txt", formatChunksPlain(chunks))

			// Stage 04: Annotation — uses ContentProvider from git-courer's own repo
			// so AnnotateDiffForRead can read real source files and produce AST labels.
			stage04Start := time.Now()
			rawDiff := diff
			for i := range chunks {
				files, err := contentProvider.GetContents(chunks[i].Files)
				if err != nil {
					// Best-effort: skip files we can't read
					continue
				}
				_ = annotator.AnnotateWithContent(&chunks[i], files, rawDiff)
			}
			// Clear internal AST fields
			for i := range chunks {
				chunks[i].BeforeSource = nil
				chunks[i].AfterSource = nil
				chunks[i].CFGBefore = nil
				chunks[i].CFGAfter = nil
			}
			stage04Duration := time.Since(stage04Start)
			// Log warning for chunks with empty annotation output
			for i := range chunks {
				if chunks[i].AnnotatedDiff == "" {
					t.Logf("WARNING: chunk %d has empty annotation output — stage 04 may have failed silently", i)
				}
			}
			annotatedJSON, _ := json.MarshalIndent(chunks, "", "  ")
			writeAuditFile(t, auditDir, "04_annotated.json", annotatedJSON)
			writeAuditFile(t, auditDir, "04_annotated.txt", formatChunksPlain(chunks))

			// Stage 05: Classification
			stage05Start := time.Now()
			for i := range chunks {
				commitType, confidence := classifier.Classify(&chunks[i])
				if commitType == "" {
					commitType = domain.InferCommitType(chunks[i])
				}
				chunks[i].CommitType = commitType
				chunks[i].ConfidenceScore = confidence
			}
			stage05Duration := time.Since(stage05Start)

			// Assertions: every classified chunk must have a type, confidence, and files
			for i := range chunks {
				require.NotEmpty(t, chunks[i].CommitType, "chunk %d should have a commit type", i)
				assert.Greater(t, chunks[i].ConfidenceScore, 0.0, "chunk %d confidence should be > 0", i)
				assert.Greater(t, len(chunks[i].Files), 0, "chunk %d should have at least 1 file", i)
				assert.False(t, chunks[i].Diff == "" && chunks[i].AnnotatedDiff == "",
					"chunk %d should not have both Diff and AnnotatedDiff empty", i)
			}

			// After classification, clear the raw diff where annotated_diff is populated.
			// annotated_diff is the digested version for the LLM — diff is redundant.
			// InferCommitType already used diff for classification, so we don't need it anymore.
			for i := range chunks {
				if chunks[i].AnnotatedDiff != "" {
					chunks[i].Diff = ""
				}
			}

			classifiedJSON, _ := json.MarshalIndent(chunks, "", "  ")
			writeAuditFile(t, auditDir, "05_classified.json", classifiedJSON)
			writeAuditFile(t, auditDir, "05_classified.txt", formatChunksPlain(chunks))

			// Stage 06: LLM — generate a commit message per chunk
			stage06Start := time.Now()
			messages := make([]string, 0, len(chunks))
			for _, chunk := range chunks {
				if len(chunk.Files) == 0 {
					continue
				}
				msg, err := llm.GenerateChunkMessage(chunk)
				if err != nil {
					t.Logf("  WARNING: LLM failed for chunk with files=%v: %v", chunk.Files, err)
					messages = append(messages, formatCommitMessage(chunk))
					continue
				}
				messages = append(messages, msg)
			}
			stage06Duration := time.Since(stage06Start)
			var messagesText strings.Builder
			for i, msg := range messages {
				messagesText.WriteString(fmt.Sprintf("--- Chunk %d ---\n%s\n\n", i, msg))
			}
			writeAuditFile(t, auditDir, "06_messages.txt", []byte(messagesText.String()))

			// Log results
			t.Logf("=== %s ===", ref.description)
			for i, chunk := range chunks {
				t.Logf("  Chunk %d: type=%s confidence=%.2f files=%v",
					i, chunk.CommitType, chunk.ConfidenceScore, chunk.Files)
				if i < len(messages) {
					t.Logf("    Message: %s", messages[i])
				}
			}
			primaryType := chunks[0].CommitType
			if primaryType == "" {
				primaryType = domain.InferCommitType(chunks[0])
			}
			validTypes := map[string]bool{"feat": true, "fix": true, "refactor": true, "chore": true, "test": true, "docs": true, "style": true, "perf": true, "ci": true, "build": true}
			assert.Contains(t, validTypes, primaryType, "primary commit type must be a valid conventional commit type")
			t.Logf("Primary commit type: %s", primaryType)
			t.Logf("Chunks: %d, Duration: chunk=%s annotate=%s classify=%s llm=%s",
				len(chunks), stage03Duration, stage04Duration, stage05Duration, stage06Duration)
			t.Logf("Audit dir: %s", auditDir)

			// Log stage report (El Reporte Obligatorio)
			t.Logf("stage 03 (chunks):          in=%dB  out=%dB  latency=%s", len(diffData), len(chunksJSON), stage03Duration)
			t.Logf("stage 04 (annotation):       in=%dB  out=%dB  latency=%s", len(chunksJSON), len(annotatedJSON), stage04Duration)
			t.Logf("stage 05 (classification):   in=%dB  out=%dB  latency=%s", len(annotatedJSON), len(classifiedJSON), stage05Duration)
			t.Logf("stage 06 (llm messages):      in=%dB  out=%dB  latency=%s", len(classifiedJSON), len(messagesText.String()), stage06Duration)

			// Write audit README
			writeAuditREADME(t, auditDir, ref.description)
		})
	}
}

// formatChunksPlain renders chunks as human-readable text.
// Shows key fields per chunk without JSON noise.
//
// Format per chunk:
//
//	=== Chunk N ===
//	Type: feat  Confidence: 0.80
//	Files: file1.go, file2.go
//	Scope: core
//	Labels: [MOD_BODY_CALL: funcName], [NEW_FUNC: OtherFunc]
//
//	@@ -1,5 +1,7 @@
//	...diff lines...
func formatChunksPlain(chunks []domain.DiffChunk) []byte {
	var sb strings.Builder
	for i, c := range chunks {
		sb.WriteString(fmt.Sprintf("=== Chunk %d ===\n", i))
		commitType := c.CommitType
		if commitType == "" {
			commitType = "(empty)"
		}
		sb.WriteString(fmt.Sprintf("Type: %s  Confidence: %.2f\n", commitType, c.ConfidenceScore))
		sb.WriteString(fmt.Sprintf("Files: %s\n", strings.Join(c.Files, ", ")))
		if c.Scope != "" {
			sb.WriteString(fmt.Sprintf("Scope: %s\n", c.Scope))
		}

		// Extract labels from annotated_diff and show them separately
		// Labels appear as [LABEL: name] inside @@ headers, e.g.:
		//   @@ -1,5 +1,7 @@ [MOD_BODY_CALL: funcName]
		var labels []string
		diffContent := c.Diff
		if c.AnnotatedDiff != "" {
			labels = extractLabels(c.AnnotatedDiff)
			diffContent = c.AnnotatedDiff
		}

		if len(labels) > 0 {
			sb.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(labels, ", ")))
		}

		// Show diff content — first 600 chars, truncated
		if diffContent != "" {
			preview := diffContent
			if len(preview) > 600 {
				preview = preview[:600] + "\n..."
			}
			// Replace tabs with spaces for readability
			preview = strings.ReplaceAll(preview, "\t", "  ")
			sb.WriteString("\n")
			for _, line := range strings.Split(preview, "\n") {
				sb.WriteString("  " + line + "\n")
			}
		}
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

// extractLabels finds all [LABEL: value] patterns in an annotated diff.
func extractLabels(annotated string) []string {
	var labels []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(annotated, "\n") {
		// Labels appear inside @@ headers like: @@ -1,5 +1,7 @@ [MOD_BODY_CALL: funcName]
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		// Find all [LABEL: value] patterns after the @@ header
		idx := strings.Index(line, "@@ ")
		if idx == -1 {
			continue
		}
		rest := line[idx+3:]
		for {
			start := strings.Index(rest, "[")
			if start == -1 {
				break
			}
			end := strings.Index(rest[start:], "]")
			if end == -1 {
				break
			}
			label := rest[start : start+end+1]
			if !seen[label] {
				seen[label] = true
				labels = append(labels, label)
			}
			rest = rest[start+end+1:]
		}
	}
	return labels
}

// formatCommitMessage generates a heuristic commit message from a classified chunk.
// Format: type(scope): summary of changed files
func formatCommitMessage(chunk domain.DiffChunk) string {
	commitType := chunk.CommitType
	if commitType == "" {
		commitType = domain.InferCommitType(chunk)
	}

	scope := inferScope(chunk.Files)

	if len(chunk.Files) == 1 {
		return fmt.Sprintf("%s(%s): update %s", commitType, scope, chunk.Files[0])
	}
	return fmt.Sprintf("%s(%s): update %s and %d more", commitType, scope, chunk.Files[0], len(chunk.Files)-1)
}

// inferScope derives a scope from file paths.
func inferScope(files []string) string {
	if len(files) == 0 {
		return "general"
	}
	// Take the first path segment after internal/ or pkg/ or cmd/
	for _, prefix := range []string{"internal/", "pkg/", "cmd/"} {
		for _, f := range files {
			if strings.HasPrefix(f, prefix) {
				parts := strings.Split(strings.TrimPrefix(f, prefix), "/")
				if len(parts) > 1 {
					return parts[0]
				}
			}
		}
	}
	// Fallback: use file extension as scope
	ext := filepath.Ext(files[0])
	if ext != "" {
		return strings.TrimPrefix(ext, ".")
	}
	return "general"
}

// writeAuditFile writes raw bytes to a file in the audit directory.
func writeAuditFile(t *testing.T, auditDir, filename string, data []byte) {
	t.Helper()
	path := filepath.Join(auditDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Logf("warning: could not write audit file %s: %v", path, err)
	}
}

// writeAuditFileAppend appends bytes to a file in the audit directory.
func writeAuditFileAppend(t *testing.T, auditDir, filename string, data []byte) {
	t.Helper()
	path := filepath.Join(auditDir, filename)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Logf("warning: could not open audit file %s: %v", path, err)
		return
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Logf("warning: could not append to audit file %s: %v", path, err)
	}
}

// writeAuditREADME creates the README.md for the audit directory.
// Explains every output file and why annotation is best-effort.
func writeAuditREADME(t *testing.T, auditDir, scenario string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Pipeline Audit: %s\n\n", scenario))
	sb.WriteString("Output of pipeline stages (03–06) against a real diff.\n\n")
	sb.WriteString("## Files\n\n")
	sb.WriteString("| File | Format | Description |\n")
	sb.WriteString("|------|--------|-------------|\n")
	sb.WriteString("| `03_chunks.txt` | Plain text | Each chunk: files, diff preview |\n")
	sb.WriteString("| `03_chunks.json` | JSON | Raw chunk data (machine-readable) |\n")
	sb.WriteString("| `04_annotated.txt` | Plain text | Same chunks after annotation attempt |\n")
	sb.WriteString("| `04_annotated.json` | JSON | Raw annotated chunk data |\n")
	sb.WriteString("| `05_classified.txt` | Plain text | Chunks with type, confidence, files |\n")
	sb.WriteString("| `05_classified.json` | JSON | Raw classified chunk data |\n")
	sb.WriteString("| `06_messages.txt` | Plain text | LLM-generated commit message per chunk |\n\n")
	sb.WriteString("## Annotation\n\n")
	sb.WriteString("Stage 04 uses `GitContentProvider` pointing at git-courer's own repo,\n")
	sb.WriteString("so `AnnotateDiffForRead` can read real Go source files and produce AST labels\n")
	sb.WriteString("(`NEW_FUNC`, `MOD_SIG`, etc.). Files not present in the working tree are skipped.\n\n")
	sb.WriteString("## Data Flow\n\n")
	sb.WriteString("```\n")
	sb.WriteString("Diff text → Chunks (03) → Annotation (04) → Classification (05) → LLM messages (06)\n")
	sb.WriteString("```\n")
	writeAuditFile(t, auditDir, "README.md", []byte(sb.String()))
}

// fatalIfErr fails the test if a pipeline stage returned an error,
// printing the audit directory path for debugging.
func fatalIfErr(t *testing.T, stage int, err error, auditDir string) {
	t.Helper()
	if err != nil {
		t.Fatalf("stage %d failed: %v (audit files: %s)", stage, err, auditDir)
	}
}

// mkReport creates a StageReport for a pipeline step.
func mkReport(idx int, input, output []byte, err error, t *testing.T) StageReport {
	t.Helper()
	return StageReport{
		Index:     idx,
		Name:      StageNames[idx],
		InputLen:  len(input),
		OutputLen: len(output),
		Err:       err,
	}
}