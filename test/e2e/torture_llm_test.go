//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/security"
)

// TestTorture_ShellInjection verifies that diffs or responses containing 
// shell-sensitive characters don't break the system logic or leak into shell commands.
func TestTorture_ShellInjection(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)
	llmA := requireOllama(t)
	sec := security.New(config.Default(), llmA)
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// 1. Malicious diff content
	maliciousContent := "func main() { }\n// ; rm -rf / ; $(id) | wall > /tmp/hacked"
	writeFile(t, dir, "malicious.go", maliciousContent)
	stageAll(dir)

	_, _, _, _, _, err := svc.PrepareCommit("commit malicious")
	if err != nil {
		t.Fatalf("Prepare failed with malicious content: %v", err)
	}

	// 2. Malicious response simulation (if we could, but svc.Execute uses LLM output)
	// We'll just execute the flow. The commit message generated will be used in 'git commit -m'.
	if _, err := svc.Execute("commit malicious", false); err != nil {
		if !strings.Contains(err.Error(), "no commits were generated") {
			t.Fatalf("Execute failed unexpectedly: %v", err)
		} else {
			t.Logf("Execute returned 'no commits were generated', which is acceptable for a small model processing malicious content.")
		}
	}

	// Verify no files were created outside the repo (very basic check)
	if _, err := os.Stat("/tmp/hacked"); err == nil {
		t.Error("CRITICAL: Shell injection leaked! /tmp/hacked exists")
		_ = os.Remove("/tmp/hacked")
	}

	detail = "injection safe"
}

// TestTorture_ConcurrentOps uses 50 goroutines to call GenerateChunkMessage 
// in parallel on the same LLM adapter.
func TestTorture_ConcurrentOps(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	const concurrency = 50
	
	chunk := domain.DiffChunk{
		Files: []string{"concurrency.go"},
		Diff:  "diff --git a/concurrency.go b/concurrency.go\n--- a/concurrency.go\n+++ b/concurrency.go\n@@ -1,1 +1,2 @@\n+func Concurrent() { }",
	}

	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	t.Logf("Firing %d concurrent requests...", concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := llmA.GenerateChunkMessage(chunk)
			if err != nil {
				errs <- fmt.Errorf("request %d failed: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	failCount := 0
	for err := range errs {
		t.Log(err)
		failCount++
	}

	detail = fmt.Sprintf("success=%d fail=%d", concurrency-failCount, failCount)
	if failCount > concurrency/2 {
		t.Errorf("Too many failures: %d/%d", failCount, concurrency)
	}
}

// TestTortureLLM_ExtremeConcurrency stresses the LLM adapter (and the provider)
// by sending many concurrent requests.
func TestTortureLLM_ExtremeConcurrency(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	// We use a relatively high number for local stress, but not so high it 
	// always crashes a standard machine. 15-20 is usually a good 'torture' level.
	const concurrency = 15
	
	chunk := domain.DiffChunk{
		Files: []string{"torture.go"},
		Diff:  "diff --git a/torture.go b/torture.go\n--- a/torture.go\n+++ b/torture.go\n@@ -1,1 +1,2 @@\n+func StressTest() { println(\"more pressure\") }",
	}

	var wg sync.WaitGroup
	results := make(chan error, concurrency)

	t.Logf("Launching %d concurrent LLM requests...", concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := llmA.GenerateChunkMessage(chunk)
			results <- err
		}(i)
	}

	// Monitor progress
	go func() {
		wg.Wait()
		close(results)
	}()

	failCount := 0
	successCount := 0
	for err := range results {
		if err != nil {
			failCount++
			t.Logf("LLM Request failed: %v", err)
		} else {
			successCount++
		}
	}

	detail = fmt.Sprintf("ok=%d fail=%d total=%d", successCount, failCount, concurrency)
	t.Logf("LLM Torture: %s", detail)

	// In a torture test, we expect success, but we log failures.
	// If more than 50% fail, it's a structural failure.
	if failCount > concurrency/2 {
		t.Errorf("Excessive LLM failures under load: %d/%d", failCount, concurrency)
	}
}

// TestTortureLLM_LongContext stresses the LLM with a very large chunk.
func TestTortureLLM_LongContext(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)

	// Generate a 2000-line diff to test context window handling
	var sb strings.Builder
	sb.WriteString("diff --git a/massive.go b/massive.go\n--- a/massive.go\n+++ b/massive.go\n@@ -1,1 +1,2000 @@\n")
	for i := 0; i < 2000; i++ {
		sb.WriteString(fmt.Sprintf("+ // Line %d of massive diff to torture LLM context\n", i))
	}
	
	chunk := domain.DiffChunk{
		Files: []string{"massive.go"},
		Diff:  sb.String(),
	}

	t.Log("Sending massive context request to LLM...")
	msg, err := llmA.GenerateChunkMessage(chunk)
	if err != nil {
		t.Fatalf("LLM failed on massive context: %v", err)
	}

	if msg == "" {
		t.Error("LLM returned empty message for massive context")
	}

	detail = fmt.Sprintf("len=%d", len(chunk.Diff))
	t.Logf("LLM response for massive context: %q", msg)
}
