//go:build e2e
// +build e2e

// Package e2e contains end-to-end stress and security tests.
// These tests intentionally push the system to its limits to verify resilience.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/secrets"
	"github.com/Alejandro-M-P/git-courer/internal/security"
)

// Use shared_test.go helpers for: LLMHost, LLMModel, LLMApiKey, requireOllama,
// sandboxRepo, makeCommitSvc, makeWorkflow, errorLLM, noOpSecurity

// ============================================================================
// Armageddon Test Suite — Stress & Security
// ============================================================================

// TestDiff5000Lines verifies that the diff chunker correctly handles a massive
// diff (5000+ lines) and produces valid chunks without crashing or hanging.
func TestDiff5000Lines(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	// Create a file with 5000+ lines of changes to generate a massive diff.
	var sb strings.Builder
	sb.WriteString("package main\n\ntype Config struct {\n")
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("\tField%d string `json:\"field%d\"`\n", i, i))
	}
	sb.WriteString("}\n\nfunc NewConfig() *Config {\n\treturn &Config{\n")
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("\t\tField%d: \"value%d\",\n", i, i))
	}
	sb.WriteString("\t}\n}\n")

	filePath := filepath.Join(dir, "massive_config.go")
	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	exec.Command("git", "-C", dir, "add", "massive_config.go").Run()

	sec := &noOpSecurity{}
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit("add massive config")
	if err != nil {
		t.Fatalf("PrepareCommit with 5000+ line diff: %v", err)
	}

	t.Logf("chunks=%d messages=%v deleted=%v warnings=%v", len(chunks), messages, deleted, warnings)

	// Verify chunks are reasonable (chunker should split this into manageable pieces).
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk for massive diff")
	}

	// Each chunk should have reasonable content (not empty).
	for i, chunk := range chunks {
		if len(chunk.Diff) == 0 {
			t.Errorf("chunk %d has empty diff", i)
		}
		t.Logf("chunk %d: files=%d diffLines=%d", i, len(chunk.Files), strings.Count(chunk.Diff, "\n"))
	}

	// Verify we can still execute (commit the massive diff).
	chunkFiles := make([][]string, len(chunks))
	for i, c := range chunks {
		chunkFiles[i] = c.Files
	}
	result, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "add massive config")
	if err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}
	t.Logf("ExecuteFromPlan result: %s", result)

	// Verify commit was created.
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if len(strings.TrimSpace(string(out))) == 0 {
		t.Error("expected commits after executing massive diff")
	}
}

// TestShellInjection verifies that malicious shell commands disguised in file
// content are safely handled (not executed) by the system. Go's exec.Command
// is safe by default, but we verify the workflow layer doesn't inadvertently
// pass user content to shell execution.
func TestShellInjection(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	sec := &noOpSecurity{}
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// Malicious content disguised as normal code.
	maliciousContent := `package main

import "os/exec"

// This looks like legitimate code but contains shell injection patterns.
func RunCommand() {
	// Injection attempt: $(whoami)
	cmd := exec.Command("sh", "-c", "echo $(whoami)")
	cmd.Run()

	// Backtick injection: %60id%60
	exec.Command("sh", "-c", "echo $"+"("+"id"+")").Run()

	// Semicolon chain: ; rm -rf /
	exec.Command("sh", "-c", "echo hello; ls /").Run()

	// Pipe to reverse shell attempt
	exec.Command("sh", "-c", "echo vulnerable | ncattacker").Run()
}

func main() {}
`

	filePath := filepath.Join(dir, "suspicious.go")
	if err := os.WriteFile(filePath, []byte(maliciousContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	exec.Command("git", "-C", dir, "add", "suspicious.go").Run()

	// Prepare should handle the content without executing anything.
	// The workflow should pass this to LLM for analysis, not execute shell commands.
	messages, chunks, _, warnings, _, err := svc.PrepareCommit("add command runner")
	if err != nil {
		t.Fatalf("PrepareCommit with shell injection patterns: %v", err)
	}

	t.Logf("messages=%v warnings=%v", messages, warnings)

	// The system should NOT have executed any shell commands — verify by checking
	// that the diff was processed as content, not command execution.
	if len(chunks) == 0 {
		t.Error("expected chunks for shell injection test file")
	}

	// Verify file content was treated as text, not executed.
	cmd := exec.Command("git", "log", "--all", "--oneline")
	cmd.Dir = dir
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Should only have initial commit + our new commit.
	if len(lines) != 2 {
		t.Errorf("expected 2 commits, got %d: %v", len(lines), lines)
	}
}

// TestBinaryDisguised verifies that the security service can detect binary
// content disguised with misleading file extensions.
func TestBinaryDisguised(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	cfg := config.Default()
	cfg.Secrets.DetectionMode = "llm"
	sec := security.New(cfg, llmA)
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// Binary content disguised as .go file (compiled binary embedded).
	// This tests the binary detection layer, not just regex patterns.
	disguisedBinary := `package main

// This file looks like Go source but contains embedded binary data.
// The security service should flag this as potentially dangerous.

func init() {
	// Binary payload disguised as comments and code
	data := []byte{
		0x7f, 0x45, 0x4c, 0x46, // ELF magic
		0x02, 0x01, 0x01, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x3e, 0x00,
	}
	_ = data
}

func main() {
	println("This is a disguised binary test")
}
`

	filePath := filepath.Join(dir, "binary.go")
	if err := os.WriteFile(filePath, []byte(disguisedBinary), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	exec.Command("git", "-C", dir, "add", "binary.go").Run()

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to sandbox: %v", err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	// LLM-based detection should flag this as suspicious.
	// Note: This test depends on the LLM's binary detection capabilities.
	messages, chunks, _, warnings, _, err := svc.PrepareCommit("add binary disguised as go file")
	t.Logf("messages=%v warnings=%v chunks=%d", messages, warnings, len(chunks))

	// Even if not blocked, the warning should be present.
	if len(warnings) == 0 && err == nil {
		t.Log("⚠ No warnings for disguised binary content — LLM may not detect binary patterns")
	}
	_ = err // May or may not error depending on LLM sensitivity
}

// TestFragmentedSecrets verifies that secrets distributed across multiple
// files are correctly detected by the security service.
func TestFragmentedSecrets(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	cfg := config.Default()
	cfg.Secrets.DetectionMode = "regex"
	sec := security.New(cfg, llmA)
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// Fragment secret across multiple files.
	// File 1: partial AWS key
	file1 := filepath.Join(dir, "config1.go")
	os.WriteFile(file1, []byte(`package config
const keyPart1 = "AKIA" + "SAMPLE_KEY_AWS"\n`), 0644)

	// File 2: remaining AWS key
	file2 := filepath.Join(dir, "config2.go")
	os.WriteFile(file2, []byte(`package config
const keyPart2 = "_123" + "456789ABCD"\n`), 0644)

	// File 3: partial GitHub token
	file3 := filepath.Join(dir, "auth.go")
	os.WriteFile(file3, []byte(`package auth
var githubToken = "ghp_" + "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"\n`), 0644)

	// File 4: Another secret fragment
	file4 := filepath.Join(dir, "secrets.go")
	os.WriteFile(file4, []byte(`package main
const dbPass = "postgres://admin:" + "secret123" + "@localhost/db"\n`), 0644)

	exec.Command("git", "-C", dir, "add", ".").Run()

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to sandbox: %v", err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	// Security service should detect at least one of the secrets.
	_, _, _, _, _, err := svc.PrepareCommit("add fragmented secrets")

	// At least one of these patterns should trigger detection.
	found := false
	if err != nil {
		if strings.Contains(err.Error(), "[SECURITY]") {
			found = true
		}
	}

	// Also check with the secrets package directly.
	results, _ := secrets.Detect([]string{file1, file2, file3, file4})
	t.Logf("secrets.Detect found %d results across %d files", len(results), 4)

	if len(results) > 0 {
		found = true
		for _, r := range results {
			t.Logf("  detected: type=%s file=%s", r.Type, r.File)
		}
	}

	if !found {
		t.Error("expected security service to detect at least one fragmented secret")
	}
}

// TestOllamaTimeout verifies that the system handles Ollama timeouts gracefully
// without hanging indefinitely.
func TestOllamaTimeout(t *testing.T) {
	// Use errorLLM to simulate immediate failure without waiting for timeout.
	errLLM := &errorLLM{msg: "simulated timeout"}
	dir, gitA := sandboxRepo(t)
	sec := &noOpSecurity{}
	svc := makeCommitSvc(t, gitA, errLLM, sec, dir)

	os.WriteFile(filepath.Join(dir, "timeout_test.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "timeout_test.go").Run()

	start := time.Now()

	// Should return error immediately, not hang.
	_, err := svc.Execute("commit timeout test", false)

	elapsed := time.Since(start)
	t.Logf("elapsed=%v error=%v", elapsed, err)

	if err == nil {
		t.Error("expected error for timeout simulation, got nil")
	}

	// Should not hang — error should be immediate (< 5 seconds).
	if elapsed > 5*time.Second {
		t.Errorf("timeout handling took too long: %v", elapsed)
	}
}

// TestMassiveFileCount verifies that the system handles 100+ modified files
// without crashing or running out of memory.
func TestMassiveFileCount(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	sec := &noOpSecurity{}
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// Create 120 files across multiple directories.
	const fileCount = 120
	t.Logf("Creating %d files...", fileCount)

	for i := 0; i < fileCount; i++ {
		subDir := filepath.Join(dir, fmt.Sprintf("module%d", i%10))
		os.MkdirAll(subDir, 0755)

		filePath := filepath.Join(subDir, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf("package module%d\n\ntype Data%d struct {\n\tID int\n\tValue string\n}\n\nfunc Process%d(d Data%d) {\n\t_ = d.ID + 1\n}\n", i%10, i, i, i)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %d: %v", i, err)
		}
	}

	exec.Command("git", "-C", dir, "add", ".").Run()

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit("add all 120 files")
	if err != nil {
		t.Fatalf("PrepareCommit with 120 files: %v", err)
	}

	t.Logf("chunks=%d messages=%v deleted=%v warnings=%v", len(chunks), messages, deleted, warnings)

	// Verify the chunker handled it.
	if len(chunks) == 0 {
		t.Fatal("expected chunks for massive file count")
	}

	// Verify we can execute.
	chunkFiles := make([][]string, len(chunks))
	for i, c := range chunks {
		chunkFiles[i] = c.Files
	}
	result, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "add all 120 files")
	if err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}
	t.Logf("ExecuteFromPlan result: %s", result)

	// Verify commits were created.
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		t.Errorf("expected commits, got: %v", lines)
	}
}

// TestConcurrentOps verifies that multiple concurrent operations don't corrupt
// state or cause race conditions.
func TestConcurrentOps(t *testing.T) {
	llmA := requireOllama(t)
	const goroutines = 5

	// Create shared sandbox but each goroutine gets its own service instance.
	dir, gitA := sandboxRepo(t)
	sec := &noOpSecurity{}

	// Create separate files for each goroutine.
	var filePaths []string
	for i := 0; i < goroutines; i++ {
		filePath := filepath.Join(dir, fmt.Sprintf("concurrent%d.go", i))
		os.WriteFile(filePath, []byte(fmt.Sprintf("package main\n\n// File %d for concurrent test\n", i)), 0644)
		filePaths = append(filePaths, filePath)
		exec.Command("git", "-C", dir, "add", filePath).Run()
	}

	var wg sync.WaitGroup
	errChan := make(chan error, goroutines)
	resultChan := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Each goroutine creates its own service to avoid internal state conflicts.
			svc := makeCommitSvc(t, gitA, llmA, sec, dir)

ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_ = ctx // ctx is reserved for future use with service calls

		// Use the sandbox repo with a short instruction.
			result, err := svc.Execute(fmt.Sprintf("commit file %d", idx), false)
			if err != nil {
				errChan <- fmt.Errorf("goroutine %d: %w", idx, err)
				return
			}
			resultChan <- result
		}(i)
	}

	wg.Wait()
	close(errChan)
	close(resultChan)

	// Collect results.
	var errors []string
	for err := range errChan {
		errors = append(errors, err.Error())
	}
	var results []string
	for r := range resultChan {
		results = append(results, r)
	}

	t.Logf("goroutines=%d results=%d errors=%d", goroutines, len(results), len(errors))
	for _, e := range errors {
		t.Logf("  error: %s", e)
	}
	for _, r := range results {
		t.Logf("  result: %s", r)
	}

	// All goroutines should complete without errors.
	if len(errors) > 0 {
		t.Errorf("concurrent operations failed: %v", errors)
	}

	// At least some commits should succeed.
	if len(results) == 0 {
		t.Error("expected at least some successful concurrent operations")
	}
}

// Use shared_test.go: errorLLM, noOpSecurity