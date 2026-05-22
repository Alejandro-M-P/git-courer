//go:build e2e

package pipeline

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/infra/classifier"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

// updateGolden is a flag that regenerates golden files from real pipeline output
// instead of comparing against them.
var updateGolden = flag.Bool("update", false, "regenerate golden files from real pipeline output")

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

// scenarios defines the table-driven test cases for E2E pipeline testing.
var scenarios = []struct {
	name         string
	instruction  string
	preview      bool
	files        map[string]string // initial files
	modifications map[string]string // modified file content (staged but not committed)
}{
	{
		name:         "simple_fix",
		instruction:  "commit the staged changes",
		preview:      false,
		files:        map[string]string{"main.go": "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"},
		modifications: map[string]string{"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello, world\")\n}\n"},
	},
	{
		name:         "multi_file_fix",
		instruction:  "refactor the handlers and tests",
		preview:      true,
		files: map[string]string{
			"handler.go":      "package main\n\nfunc Handle() {\n\trespond(nil)\n}\n",
			"handler_test.go": "package main\n\nfunc TestHandle(t *testing.T) {\n\tHandle()\n}\n",
		},
		modifications: map[string]string{
			"handler.go":      "package main\n\nfunc Handle(w http.ResponseWriter) {\n\tw.WriteHeader(200)\n}\n",
			"handler_test.go": "package main\n\nfunc TestHandle(t *testing.T) {\n\tHandle(httptest.NewRecorder())\n}\n",
		},
	},
}

func TestE2EPipeline(t *testing.T) {
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// Step 1: Create real git repo in temp dir
			gitPort, dir := createGitRepo(t, sc.files, sc.modifications)

			// Step 2: Wire real production deps with LLM
			deps := wireRealDeps(t, gitPort, dir)

			// Step 3: Build request JSON
			req := CommitRequest{
				Instruction: sc.instruction,
				Preview:     sc.preview,
			}
			input, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}

			// Step 4: Define stage output file names (used for audit and golden)
			//
			// Pipeline audit directory: /tmp/pipeline-audit/{scenario_name}/
			//
			// Each file contains a header with the stage number, name, input/output sizes,
			// and type hints, followed by the raw output. This makes the full data flow
			// inspectable: open any file and you can see what that stage received and produced.
			//
			// Stage routing (not all stages chain sequentially):
			//   Stage 00: request    → input: user request JSON
			//   Stage 01: diff       → input: request JSON → output: raw git diff
			//   Stage 02: security   → input: diff text from Stage 01
			//   Stage 03: chunks     → input: diff text from Stage 01 (NOT security JSON)
			//   Stage 04: annotation → input: chunks JSON from Stage 03
			//   Stage 05: classifica → input: annotated JSON from Stage 04
			//   Stage 06: llm        → input: classified JSON from Stage 05 → output: plain text message
			//   Stage 07: result     → input: plain text message from Stage 06 → output: final JSON result
			//
			// Additional files in audit directory:
			//   annotated_diff.txt   → extracted annotated diff (human-readable version of Stage 04 output)
			goldenNames := []string{
				"00_request.json", "01_diff.txt", "02_security.json",
				"03_chunks.json", "04_annotated.json", "05_classified.json",
				"06_message.txt", "07_result.json",
			}

			// Step 5: Create audit directory in /tmp for this scenario
			auditDir := filepath.Join(os.TempDir(), "pipeline-audit", sc.name)
			// Clean audit dir so only this run's files remain
			os.RemoveAll(auditDir)
			os.MkdirAll(auditDir, 0o755)
			t.Logf("audit dir: %s", auditDir)
			writeAuditREADME(t, auditDir, sc.name)

			// Step 6: Run full pipeline, collecting StageReport per stage
			// Note: some stages need non-sequential inputs:
			//   Stage 03 (Chunks) needs diff text from Stage 01, not security JSON from Stage 02.
			//   Stage 04 (Annotation) needs chunks from Stage 03.
			//   Stage 06 (LLM) needs classified chunks from Stage 05.
			// We track all outputs and route the correct input to each stage.
			reports := make([]StageReport, 0, NumStages())
			var stageOutputs [][]byte // indexed by stage number
			var stageInputs [][]byte  // what each stage received as input

			// Stage 0: validate request
			out0, err := RunStage(0, input, deps)
			reports = append(reports, mkReport(0, input, out0, err, t))
			stageInputs = append(stageInputs, input)
			writeAuditStage(t, auditDir, 0, "request", input, out0)
			fatalIfErr(t, 0, err, auditDir)
			stageOutputs = append(stageOutputs, out0)

			// Stage 1: get diff
			out1, err := RunStage(1, out0, deps)
			reports = append(reports, mkReport(1, out0, out1, err, t))
			stageInputs = append(stageInputs, out0)
			writeAuditStage(t, auditDir, 1, "diff", out0, out1)
			fatalIfErr(t, 1, err, auditDir)
			stageOutputs = append(stageOutputs, out1)

			// Stage 2: security check (input: diff text from Stage 1)
			out2, err := RunStage(2, out1, deps)
			reports = append(reports, mkReport(2, out1, out2, err, t))
			stageInputs = append(stageInputs, out1)
			writeAuditStage(t, auditDir, 2, "security", out1, out2)
			fatalIfErr(t, 2, err, auditDir)
			stageOutputs = append(stageOutputs, out2)

			// Stage 3: chunking (input: diff text from Stage 1, NOT security JSON from Stage 2)
			out3, err := RunStage(3, out1, deps)
			reports = append(reports, mkReport(3, out1, out3, err, t))
			stageInputs = append(stageInputs, out1)
			writeAuditStage(t, auditDir, 3, "chunks", out1, out3)
			fatalIfErr(t, 3, err, auditDir)
			stageOutputs = append(stageOutputs, out3)

			// Stage 4: annotation (input: chunks JSON from Stage 3)
			out4, err := RunStage(4, out3, deps)
			reports = append(reports, mkReport(4, out3, out4, err, t))
			stageInputs = append(stageInputs, out3)
			writeAuditStage(t, auditDir, 4, "annotated", out3, out4)
			fatalIfErr(t, 4, err, auditDir)
			stageOutputs = append(stageOutputs, out4)

			// Stage 5: classification (input: annotated chunks from Stage 4)
			out5, err := RunStage(5, out4, deps)
			reports = append(reports, mkReport(5, out4, out5, err, t))
			stageInputs = append(stageInputs, out4)
			writeAuditStage(t, auditDir, 5, "classified", out4, out5)
			fatalIfErr(t, 5, err, auditDir)
			stageOutputs = append(stageOutputs, out5)

			// Stage 6: LLM generation (input: classified chunks from Stage 5)
			out6, err := RunStage(6, out5, deps)
			reports = append(reports, mkReport(6, out5, out6, err, t))
			stageInputs = append(stageInputs, out5)
			writeAuditStage(t, auditDir, 6, "message", out5, out6)
			fatalIfErr(t, 6, err, auditDir)
			stageOutputs = append(stageOutputs, out6)

			// Stage 7: result assembly (input: message text from Stage 6)
			out7, err := RunStage(7, out6, deps)
			reports = append(reports, mkReport(7, out6, out7, err, t))
			stageInputs = append(stageInputs, out6)
			writeAuditStage(t, auditDir, 7, "result", out6, out7)
			fatalIfErr(t, 7, err, auditDir)
			stageOutputs = append(stageOutputs, out7)

			// Step 7: Extract annotated diff for easy inspection.
			// The annotated diff is embedded inside 04_annotated.json, but it's
			// hard to read there. Extract it as a separate human-readable file
			// with context about which chunk it belongs to.
			var annotatedChunks []domain.DiffChunk
			if err := json.Unmarshal(out4, &annotatedChunks); err == nil {
				var sb strings.Builder
				sb.WriteString("# === Annotated Diff (extracted from Stage 04) ===\n")
				sb.WriteString("# This is the human-readable version of the AST-labeled diff.\n")
				sb.WriteString("# Each section shows the chunk's files, commit type, and confidence.\n\n")
				for i, chunk := range annotatedChunks {
					sb.WriteString(fmt.Sprintf("# --- Chunk %d: files=%v, type=%q, confidence=%.2f ---\n",
						i, chunk.Files, chunk.CommitType, chunk.ConfidenceScore))
					if chunk.AnnotatedDiff != "" {
						sb.WriteString(chunk.AnnotatedDiff)
						sb.WriteString("\n")
					} else {
						sb.WriteString("# (no annotated diff for this chunk)\n")
					}
					sb.WriteString("\n")
				}
				writeAuditFile(t, auditDir, "annotated_diff.txt", []byte(sb.String()))
			}

			// Step 7: Log StageReport per stage (El Reporte Obligatorio)
			for _, r := range reports {
				t.Logf("stage %02d (%-15s): in=%dB  out=%dB  latency=%s",
					r.Index, r.Name, r.InputLen, r.OutputLen, r.Latency)
			}

			// Step 8: Golden file comparison or regeneration
			for i, output := range stageOutputs {
				if i >= len(goldenNames) {
					break
				}
				goldenName := goldenNames[i]

				if *updateGolden {
					if i <= 5 {
						writeGolden(t, sc.name, goldenName, output)
						t.Logf("updated golden: %s/%s", sc.name, goldenName)
					} else {
						t.Logf("skipping golden update for stage %d (%s) — LLM output is non-deterministic", i, StageNames[i])
					}
					continue
				}

				// Stages 0-5: exact byte comparison
				if i <= 5 {
					expected := readGolden(t, sc.name, goldenName)
					if string(output) != string(expected) {
						t.Errorf("stage %d (%s) golden mismatch for %s/%s:\n  got %d bytes\n  want %d bytes",
							i, StageNames[i], sc.name, goldenName, len(output), len(expected))
					}
					continue
				}

				// Stages 6-7: structural comparison
				if i == 6 {
					assertLLMOutput(t, output)
				}
				if i == 7 {
					assertStructuralResult(t, output)
				}
			}

			// Step 9: Verify all stages succeeded without error
			for _, r := range reports {
				if r.Err != nil {
					t.Errorf("stage %d (%s) had error: %v", r.Index, r.Name, r.Err)
				}
			}

			// Step 10: Clean up temp directory (t.TempDir auto-cleans)
			_ = dir
		})
	}
}

// createGitRepo creates a real git repository in a temp directory,
// commits initial files, then stages modifications.
// Returns the Git port adapter and the temp directory path.
func createGitRepo(t *testing.T, files, modifications map[string]string) (ports.Git, string) {
	t.Helper()
	dir := t.TempDir()

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// git config user
	for _, args := range [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}

	// Write initial files
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// git add + commit initial
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add initial: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit initial: %v", err)
	}

	// Write modifications (staged but not committed)
	for name, content := range modifications {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write modified %s: %v", path, err)
		}
	}

	// git add modifications (staged)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add modifications: %v", err)
	}

	// Create real Git port adapter bound to this temp dir
	gitPort := git.New(dir)

	return gitPort, dir
}

// wireRealDeps creates real production adapters for the pipeline.
// Requires a running LLM service (connected via RequireLLM).
func wireRealDeps(t *testing.T, gitPort ports.Git, repoDir string) StageDeps {
	t.Helper()
	llm := testutil.RequireLLM(t)

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Model:    testutil.LLMModel,
		},
	}

	securitySvc := security.New(cfg, llm)

	chunker := chunkers.NewDiffChunker(
		chunkers.WithMaxFilesPerChunk(12),
		chunkers.WithMinForce(3),
	)

	// Annotator and classifier need a language catalog
	catalog := chunkers.NewLanguageCatalog()
	annotator := chunkers.NewChunkAnnotatorAdapter(catalog)

	classifier := classifier.NewClassifierWithCatalog(gitPort, catalog)

	// ContentProvider that reads file contents from the git repo
	contentProvider := git.NewGitContentProvider(repoDir)

	return StageDeps{
		Git:             gitPort,
		Security:        securitySvc,
		Chunker:         chunker,
		Annotator:       annotator,
		Classifier:      classifier,
		LLM:             llm,
		ContentProvider: contentProvider,
		ChunkSize:       4000,
	}
}

// assertLLMOutput verifies that LLM output (stage 6) meets structural criteria:
// - non-empty string
// - at least 10 characters long
// - NOT valid JSON (it's plain text)
func assertLLMOutput(t *testing.T, output []byte) {
	t.Helper()
	text := strings.TrimSpace(string(output))
	if text == "" {
		t.Fatal("LLM produced empty output")
	}
	if len(text) < 10 {
		t.Fatalf("LLM output too short (%d chars), expected >= 10", len(text))
	}
	// LLM output should be plain text, not JSON
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Fatal("LLM output should be plain text, not JSON")
	}
}

// assertStructuralResult verifies that the final pipeline result (stage 7) is
// valid JSON with a non-empty message field.
func assertStructuralResult(t *testing.T, data []byte) {
	t.Helper()
	var result PipelineResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("stage 07 output is not valid JSON: %v", err)
	}
	if result.Message == "" {
		t.Fatal("stage 07 result has empty message")
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

// writeAuditStage writes a pipeline stage's input and output to the audit directory.
// Each file gets a human-readable header with context, followed by the raw data.
//
// Audit directory structure:
//
//	/tmp/pipeline-audit/{scenario}/
//	  00_request.json      - Stage 00: validated request
//	  01_diff.txt          - Stage 01: raw git diff
//	  02_security.json     - Stage 02: security check result
//	  03_chunks.json       - Stage 03: chunked diff (input: raw diff, NOT security JSON)
//	  04_annotated.json    - Stage 04: AST-annotated chunks
//	  05_classified.json   - Stage 05: classified chunks with commit type & confidence
//	  06_message.txt        - Stage 06: LLM-generated commit message (plain text, NOT JSON)
//	  07_result.json        - Stage 07: final pipeline result
//	  annotated_diff.txt   - Extracted human-readable annotated diff
//	  README.md            - Documentation explaining every file and the data flow
func writeAuditStage(t *testing.T, auditDir string, stageNum int, stageName string, input, output []byte) {
	t.Helper()
	filename := fmt.Sprintf("%02d_%s.json", stageNum, stageName)
	if stageNum == 1 {
		filename = "01_diff.txt" // diff is plain text, not JSON
	}
	if stageNum == 6 {
		filename = "06_message.txt" // LLM output is plain text
	}

	// Input format: what kind of data this stage received
	inputHint := "json"
	switch stageNum {
	case 0, 1:
		inputHint = "request json"
	case 2, 3:
		inputHint = "diff text (from stage 01)"
	case 4:
		inputHint = "chunks json (from stage 03)"
	case 5:
		inputHint = "annotated chunks json (from stage 04)"
	case 6:
		inputHint = "classified chunks json (from stage 05)"
	case 7:
		inputHint = "plain text commit message (from stage 06)"
	}

	// Output format: what kind of data this stage produced
	outputHint := "json"
	switch stageNum {
	case 1:
		outputHint = "unified diff"
	case 6:
		outputHint = "plain text commit message"
	}

	// Write the raw data file (no comment header — keep data parseable)
	writeAuditFile(t, auditDir, filename, output)

	// Append metadata to README for this stage
	readmeLine := fmt.Sprintf("| %02d | %-13s | %s | %s | %dB | %dB |\n",
		stageNum, stageName, inputHint, outputHint, len(input), len(output))
	writeAuditFileAppend(t, auditDir, "README.md", []byte(readmeLine))
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

// writeAuditREADME creates the README.md header in the audit directory.
// Each stage appends its own row via writeAuditFileAppend.
func writeAuditREADME(t *testing.T, auditDir, scenario string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("# Pipeline Audit: %s\n\n")
	sb.WriteString("This directory contains the output of each pipeline stage for inspection.\n")
	sb.WriteString("Every file is the raw output of its stage — no headers, no comments — so you\n")
	sb.WriteString("can parse JSON files with `jq` or open text files directly.\n\n")
	sb.WriteString("## Data Flow\n\n")
	sb.WriteString("Not all stages chain sequentially. Stage 03 (chunks) and Stage 02 (security)\n")
	sb.WriteString("both receive the **diff text from Stage 01**, not each other's output.\n\n")
	sb.WriteString("```\n")
	sb.WriteString("Request (00) → Diff (01) → Security (02)  // both read diff from 01\n")
	sb.WriteString("                 Diff (01) → Chunks (03) → Annotation (04) → Classification (05) → LLM (06) → Result (07)\n")
	sb.WriteString("```\n\n")
	sb.WriteString("## Stage Details\n\n")
	sb.WriteString("| # | Stage | Input | Output | In | Out |\n")
	sb.WriteString("|---|-------|-------|--------|----|-----|\n")
	writeAuditFile(t, auditDir, "README.md", []byte(fmt.Sprintf(sb.String(), scenario)))
}

// fatalIfErr fails the test if a pipeline stage returned an error,
// printing the audit directory path for debugging.
func fatalIfErr(t *testing.T, stage int, err error, auditDir string) {
	t.Helper()
	if err != nil {
		t.Fatalf("stage %d (%s) failed: %v (audit files: %s)", stage, StageNames[stage], err, auditDir)
	}
}

