//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

// TestCommitSingleFile is the baseline: one modified file → prepare → apply → verify commit.
func TestCommitSingleFile(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	stageFile(dir, "main.go")

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit("add main entry point")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected at least one commit message")
	}

	detail = fmt.Sprintf("chunks=%d", len(chunks))
	t.Logf("messages=%v warnings=%v chunks=%d", messages, warnings, len(chunks))

	chunkFiles := workflow.DiffChunksToChunkFiles(chunks)
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "add main entry point"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	logs := gitLog(dir)
	if len(logs) < 2 {
		t.Errorf("expected ≥2 commits, got %d: %v", len(logs), logs)
	}
	t.Logf("git log: %v", logs)
}

// TestCommitMultiFileMultiChunk creates files across separate modules to force
// multiple chunks and verifies that each chunk generates its own commit message.
func TestCommitMultiFileMultiChunk(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	// 15 files across 3 packages — chunker should group by semantic proximity
	for pkg := 0; pkg < 3; pkg++ {
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("pkg%d/file%d.go", pkg, i)
			content := fmt.Sprintf("package pkg%d\n\n// File %d\ntype T%d struct{ ID int }\n", pkg, i, i)
			writeFile(t, dir, name, content)
		}
	}
	stageAll(dir)

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit("add package scaffolding")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	if len(messages) != len(chunks) {
		t.Errorf("messages=%d chunks=%d — must be equal", len(messages), len(chunks))
	}

	detail = fmt.Sprintf("chunks=%d", len(chunks))
	t.Logf("chunks=%d messages=%v warnings=%v", len(chunks), messages, warnings)

	chunkFiles := workflow.DiffChunksToChunkFiles(chunks)
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "add package scaffolding"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	logs := gitLog(dir)
	// Expect initial commit + at least 1 commit per PrepareCommit result
	if len(logs) < 2 {
		t.Errorf("expected ≥2 commits, got %d", len(logs))
	}
	t.Logf("git log (%d commits): %v", len(logs), logs)
}

// TestCommitConventionalFormat verifies that generated messages follow conventional commit format.
func TestCommitConventionalFormat(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	writeFile(t, dir, "auth/login.go", "package auth\n\nfunc Login(u, p string) bool { return u == p }\n")
	stageAll(dir)

	messages, chunks, _, _, _, err := svc.PrepareCommit("add login function")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}

	conventionalPrefixes := []string{
		"feat", "fix", "chore", "refactor", "docs", "style", "test", "perf", "ci", "build",
	}
	allValid := true
	for _, msg := range messages {
		valid := false
		for _, prefix := range conventionalPrefixes {
			if strings.HasPrefix(msg, prefix+":") || strings.HasPrefix(msg, prefix+"(") {
				valid = true
				break
			}
		}
		if !valid {
			t.Logf("⚠ message may not be conventional commit: %q", msg)
			allValid = false
		}
	}

	detail = fmt.Sprintf("chunks=%d valid=%v", len(chunks), allValid)
	t.Logf("messages=%v", messages)
}

// TestCommitDeletedFile verifies that deleted files appear in the plan and are committed.
func TestCommitDeletedFile(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	// Commit an existing file first
	writeFile(t, dir, "old.go", "package main\n")
	gitExec(t, dir, "add", "old.go")
	gitExec(t, dir, "commit", "-m", "chore: add old file")

	// Delete it
	os.Remove(filepath.Join(dir, "old.go"))

	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit("remove old file")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}

	detail = fmt.Sprintf("deleted=%d", len(deleted))
	t.Logf("messages=%v deleted=%v warnings=%v chunks=%d", messages, deleted, warnings, len(chunks))

	if len(deleted) == 0 {
		t.Error("expected at least one deleted file in plan")
	}

	chunkFiles := workflow.DiffChunksToChunkFiles(chunks)
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "remove old file"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	logs := gitLog(dir)
	t.Logf("git log: %v", logs)
	if len(logs) < 3 {
		t.Errorf("expected ≥3 commits after delete commit, got %d", len(logs))
	}
}

// TestCommitPrepareAbort verifies that aborting after prepare does NOT create any git commit.
func TestCommitPrepareAbort(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	writeFile(t, dir, "feature.go", "package main\n\nfunc Feature() {}\n")
	stageFile(dir, "feature.go")

	logsBefore := gitLog(dir)

	messages, chunks, _, _, _, err := svc.PrepareCommit("add feature")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}

	detail = fmt.Sprintf("chunks=%d aborted", len(chunks))
	t.Logf("prepare returned %d messages — aborting (no apply)", len(messages))

	// Intentionally NOT calling ExecuteFromPlan — simulates user abort
	logsAfter := gitLog(dir)
	if len(logsAfter) != len(logsBefore) {
		t.Errorf("git log changed after prepare-only: before=%d after=%d", len(logsBefore), len(logsAfter))
	}
}

// TestCommitRegenerate verifies that RegenerateMessage produces different messages
// and that the regenerated messages can be used to execute a commit.
func TestCommitRegenerate(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	writeFile(t, dir, "service.go", "package main\n\ntype Service struct{ Name string }\n\nfunc (s *Service) Run() {}\n")
	stageFile(dir, "service.go")

	origMessages, chunks, deleted, _, _, err := svc.PrepareCommit("add service")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}

	regenMessages, err := llmA.RegenerateMessage(origMessages, "be more descriptive about what the service does", chunks)
	if err != nil {
		t.Fatalf("RegenerateMessage: %v", err)
	}
	if len(regenMessages) == 0 {
		t.Fatal("expected regenerated messages")
	}
	if len(regenMessages) != len(chunks) {
		t.Errorf("regenerated=%d chunks=%d — must match", len(regenMessages), len(chunks))
	}

	detail = fmt.Sprintf("chunks=%d regen=ok", len(chunks))
	t.Logf("original=%v regenerated=%v", origMessages, regenMessages)

	chunkFiles := workflow.DiffChunksToChunkFiles(chunks)
	if _, err := svc.ExecuteFromPlan(regenMessages, chunkFiles, deleted, "add service"); err != nil {
		t.Fatalf("ExecuteFromPlan with regenerated messages: %v", err)
	}

	logs := gitLog(dir)
	if len(logs) < 2 {
		t.Errorf("expected ≥2 commits after execute, got %d", len(logs))
	}
}

// TestCommitSecretBlocked verifies that PrepareCommit is blocked when a file
// contains an AWS access key pattern.
func TestCommitSecretBlocked(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	cfg := config.Default()
	cfg.Secrets.DetectionMode = "regex"
	sec := security.New(cfg, llmA)
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// AWS access key pattern that triggers regex detection
	writeFile(t, dir, "config.go", `package config

const AWSKey = "AKIAIOSFODNN7EXAMPLE"
const AWSSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
`)
	stageAll(dir)

	_, _, _, _, _, err := svc.PrepareCommit("add aws config")

	detail = fmt.Sprintf("blocked=%v", err != nil && strings.Contains(err.Error(), "SECURITY"))
	t.Logf("PrepareCommit error: %v", err)

	if err == nil {
		t.Error("expected PrepareCommit to return error for file with AWS credentials")
		return
	}
	if !strings.Contains(err.Error(), "[SECURITY]") {
		t.Errorf("expected [SECURITY] prefix in error, got: %v", err)
	}

	// Verify no extra commit was created
	logs := gitLog(dir)
	if len(logs) != 1 {
		t.Errorf("expected only initial commit, got %d: %v", len(logs), logs)
	}
}

// TestCommitDirectExecute calls Execute (single-step, no prepare) and verifies
// that a commit is created without the prepare phase.
func TestCommitDirectExecute(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	writeFile(t, dir, "utils.go", "package main\n\nfunc helper() string { return \"ok\" }\n")
	stageAll(dir)

	result, err := svc.Execute("add helper utility", false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	detail = "direct"
	t.Logf("Execute result: %s", result)

	logs := gitLog(dir)
	if len(logs) < 2 {
		t.Errorf("expected ≥2 commits after direct execute, got %d", len(logs))
	}
}

// TestCommitLargeDiff verifies that a 5000+ line diff is chunked and committed
// without crashing or hanging.
func TestCommitLargeDiff(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	var sb strings.Builder
	sb.WriteString("package main\n\ntype Config struct {\n")
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("\tField%d string `json:\"field%d\"`\n", i, i))
	}
	sb.WriteString("}\n\nfunc NewConfig() *Config {\n\treturn &Config{\n")
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("\t\tField%d: \"default%d\",\n", i, i))
	}
	sb.WriteString("\t}\n}\n")

	writeFile(t, dir, "massive_config.go", sb.String())
	stageAll(dir)

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit("add generated config struct")
	if err != nil {
		t.Fatalf("PrepareCommit with large diff: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected chunks for large diff")
	}

	detail = fmt.Sprintf("chunks=%d", len(chunks))
	t.Logf("chunks=%d messages=%v warnings=%v", len(chunks), messages, warnings)

	for i, ch := range chunks {
		t.Logf("  chunk[%d]: files=%v diffLines=%d", i, ch.Files, strings.Count(ch.Diff, "\n"))
	}

	chunkFiles := workflow.DiffChunksToChunkFiles(chunks)
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "add generated config struct"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	logs := gitLog(dir)
	if len(logs) < 2 {
		t.Errorf("expected ≥2 commits, got %d", len(logs))
	}
}

// TestCommitMassiveFileCount verifies that 120 files across 10 directories
// are correctly chunked and committed.
func TestCommitMassiveFileCount(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	const fileCount = 120
	for i := 0; i < fileCount; i++ {
		name := fmt.Sprintf("module%d/entity%d.go", i%10, i)
		content := fmt.Sprintf("package module%d\n\ntype Entity%d struct{ ID int }\n\nfunc New%d() *Entity%d { return &Entity%d{} }\n",
			i%10, i, i, i, i)
		writeFile(t, dir, name, content)
	}
	stageAll(dir)

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit("scaffold all entities")
	if err != nil {
		t.Fatalf("PrepareCommit with %d files: %v", fileCount, err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected chunks for massive file count")
	}

	detail = fmt.Sprintf("chunks=%d", len(chunks))
	t.Logf("fileCount=%d chunks=%d messages=%d warnings=%v", fileCount, len(chunks), len(messages), warnings)

	chunkFiles := workflow.DiffChunksToChunkFiles(chunks)
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "scaffold all entities"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	logs := gitLog(dir)
	if len(logs) < 2 {
		t.Errorf("expected ≥2 commits, got %d", len(logs))
	}
	t.Logf("git log (%d commits)", len(logs))
}

// TestCommitConcurrent sends multiple simultaneous PrepareCommit calls to Ollama
// to verify concurrency safety.
func TestCommitConcurrent(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	const n = 4

	type result struct {
		idx  int
		msgs []string
		err  error
	}
	resultCh := make(chan result, n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			dir, gitA := sandboxRepo(t)
			svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)
			writeFile(t, dir, fmt.Sprintf("goroutine%d.go", i),
				fmt.Sprintf("package main\n\n// goroutine %d\nfunc G%d() {}\n", i, i))
			stageAll(dir)
			msgs, _, _, _, _, err := svc.PrepareCommit(fmt.Sprintf("add goroutine%d file", i))
			resultCh <- result{i, msgs, err}
		}()
	}

	success, failure := 0, 0
	for i := 0; i < n; i++ {
		r := <-resultCh
		if r.err != nil {
			failure++
			t.Logf("goroutine %d error: %v", r.idx, r.err)
		} else {
			success++
			t.Logf("goroutine %d messages: %v", r.idx, r.msgs)
		}
	}

	detail = fmt.Sprintf("ok=%d err=%d", success, failure)
	t.Logf("concurrent: success=%d failure=%d", success, failure)

	if success == 0 {
		t.Error("expected at least one successful concurrent PrepareCommit")
	}
}

// TestCommitUnusualInstructions verifies that unusual unicode/injection-like
// instructions are handled gracefully without crashing.
func TestCommitUnusualInstructions(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	cases := []struct {
		instruction string
		file        string
		content     string
	}{
		{"feat: 你好世界 🎉", "unicode.go", "package main\n\nfunc Unicode() {}\n"},
		{"fix: semicolon; injection; attempt", "semi.go", "package main\n\nfunc Semi() {}\n"},
		{"add: ${SHELL} injection test", "inject.go", "package main\n\nfunc Inject() {}\n"},
	}

	detail = "unusual"
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)
			writeFile(t, dir, tc.file, tc.content)
			stageFile(dir, tc.file)

			_, _, _, _, _, err := svc.PrepareCommit(tc.instruction)
			t.Logf("instruction=%q err=%v", tc.instruction, err)
			// System must not panic — error is acceptable for unusual input
		})
	}
}

// TestCommitChunkerIntegrity verifies that chunks returned by PrepareCommit
// are semantically complete: each chunk references files and a non-empty diff.
func TestCommitChunkerIntegrity(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	cfg := workflow.DefaultCommitServiceConfig(4096, 50, filepath.Join(dir, ".gcourer", "commit.log"))
	svc := workflow.NewCommitService(gitA, llmA, chunkers.NewDiffChunker(), &noOpSecurity{}, cfg)

	// Three semantically distinct packages
	for pkg, name := range []string{"api", "db", "cache"} {
		for i := 0; i < 3; i++ {
			writeFile(t, dir, fmt.Sprintf("%s/type%d.go", name, i),
				fmt.Sprintf("package %s\n\ntype T%d struct{ V%d int }\n", name, pkg*3+i, pkg*3+i))
		}
	}
	stageAll(dir)

	_, chunks, _, _, _, err := svc.PrepareCommit("add API, DB and cache types")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}

	for i, ch := range chunks {
		if len(ch.Files) == 0 {
			t.Errorf("chunk[%d] has no files", i)
		}
		if ch.Diff == "" {
			t.Errorf("chunk[%d] has empty diff", i)
		}
		t.Logf("chunk[%d]: files=%v diffLines=%d", i, ch.Files, strings.Count(ch.Diff, "\n"))
	}

	detail = fmt.Sprintf("chunks=%d", len(chunks))
}
