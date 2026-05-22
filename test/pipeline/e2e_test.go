//go:build e2e

package pipeline

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/config"
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

			// Step 6: Run full pipeline, collecting StageReport per stage
			// Note: some stages need non-sequential inputs:
			//   Stage 03 (Chunks) needs diff text from Stage 01, not security JSON from Stage 02.
			//   Stage 04 (Annotation) needs chunks from Stage 03.
			//   Stage 06 (LLM) needs classified chunks from Stage 05.
			// We track all outputs and route the correct input to each stage.
			reports := make([]StageReport, 0, NumStages())
			var stageOutputs [][]byte // indexed by stage number

			// Stage 0: validate request
			out0, err := RunStage(0, input, deps)
			reports = append(reports, mkReport(0, input, out0, err, t))
			writeAudit(t, auditDir, goldenNames[0], out0)
			fatalIfErr(t, 0, err, auditDir)
			stageOutputs = append(stageOutputs, out0)

			// Stage 1: get diff
			out1, err := RunStage(1, out0, deps)
			reports = append(reports, mkReport(1, out0, out1, err, t))
			writeAudit(t, auditDir, goldenNames[1], out1)
			fatalIfErr(t, 1, err, auditDir)
			stageOutputs = append(stageOutputs, out1)

			// Stage 2: security check (input: diff text from Stage 1)
			out2, err := RunStage(2, out1, deps)
			reports = append(reports, mkReport(2, out1, out2, err, t))
			writeAudit(t, auditDir, goldenNames[2], out2)
			fatalIfErr(t, 2, err, auditDir)
			stageOutputs = append(stageOutputs, out2)

			// Stage 3: chunking (input: diff text from Stage 1, NOT security JSON from Stage 2)
			out3, err := RunStage(3, out1, deps)
			reports = append(reports, mkReport(3, out1, out3, err, t))
			writeAudit(t, auditDir, goldenNames[3], out3)
			fatalIfErr(t, 3, err, auditDir)
			stageOutputs = append(stageOutputs, out3)

			// Stage 4: annotation (input: chunks JSON from Stage 3)
			out4, err := RunStage(4, out3, deps)
			reports = append(reports, mkReport(4, out3, out4, err, t))
			writeAudit(t, auditDir, goldenNames[4], out4)
			fatalIfErr(t, 4, err, auditDir)
			stageOutputs = append(stageOutputs, out4)

			// Stage 5: classification (input: annotated chunks from Stage 4)
			out5, err := RunStage(5, out4, deps)
			reports = append(reports, mkReport(5, out4, out5, err, t))
			writeAudit(t, auditDir, goldenNames[5], out5)
			fatalIfErr(t, 5, err, auditDir)
			stageOutputs = append(stageOutputs, out5)

			// Stage 6: LLM generation (input: classified chunks from Stage 5)
			out6, err := RunStage(6, out5, deps)
			reports = append(reports, mkReport(6, out5, out6, err, t))
			writeAudit(t, auditDir, goldenNames[6], out6)
			fatalIfErr(t, 6, err, auditDir)
			stageOutputs = append(stageOutputs, out6)

			// Stage 7: result assembly (input: message text from Stage 6)
			out7, err := RunStage(7, out6, deps)
			reports = append(reports, mkReport(7, out6, out7, err, t))
			writeAudit(t, auditDir, goldenNames[7], out7)
			fatalIfErr(t, 7, err, auditDir)
			stageOutputs = append(stageOutputs, out7)

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

// writeAudit writes a stage output to the audit directory.
func writeAudit(t *testing.T, auditDir, filename string, data []byte) {
	t.Helper()
	path := filepath.Join(auditDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Logf("warning: could not write audit file %s: %v", path, err)
	}
}

// fatalIfErr fails the test if a pipeline stage returned an error,
// printing the audit directory path for debugging.
func fatalIfErr(t *testing.T, stage int, err error, auditDir string) {
	t.Helper()
	if err != nil {
		t.Fatalf("stage %d (%s) failed: %v (audit files: %s)", stage, StageNames[stage], err, auditDir)
	}
}

