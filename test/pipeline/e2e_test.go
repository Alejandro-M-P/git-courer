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

			// Step 4: Run full pipeline, collecting StageReport per stage
			reports := make([]StageReport, 0, NumStages())
			current := input
			var stageOutputs [][]byte

			for i := 0; i < NumStages(); i++ {
				start := time.Now()
				output, err := RunStage(i, current, deps)
				latency := time.Since(start)

				reports = append(reports, StageReport{
					Index:     i,
					Name:      StageNames[i],
					InputLen:  len(current),
					OutputLen: len(output),
					Latency:   latency,
					Err:       err,
				})

				if err != nil {
					t.Fatalf("stage %d (%s) failed: %v", i, StageNames[i], err)
				}

				stageOutputs = append(stageOutputs, output)
				current = output
			}

			// Step 5: Log StageReport per stage (El Reporte Obligatorio)
			for _, r := range reports {
				t.Logf("stage %02d (%-15s): in=%dB  out=%dB  latency=%s",
					r.Index, r.Name, r.InputLen, r.OutputLen, r.Latency)
			}

			// Step 6: Golden file comparison or regeneration
			goldenNames := []string{
				"00_request.json", "01_diff.txt", "02_security.json",
				"03_chunks.json", "04_annotated.json", "05_classified.json",
				"06_message.txt", "07_result.json",
			}

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

			// Step 7: Verify all stages succeeded without error
			for _, r := range reports {
				if r.Err != nil {
					t.Errorf("stage %d (%s) had error: %v", r.Index, r.Name, r.Err)
				}
			}

			// Step 8: Clean up temp directory (t.TempDir auto-cleans)
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

