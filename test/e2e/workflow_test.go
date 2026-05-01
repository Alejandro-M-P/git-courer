//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
)

type TestContext struct {
	t      *testing.T
	dir    string
	gitA   ports.Git
	llmA   ports.LLM
	chunks []domain.DiffChunk
	msgs   []string
}

func NewTestContext(t *testing.T, dir string, gitA ports.Git) *TestContext {
	llmA := requireOllama(t)
	return &TestContext{
		t:    t,
		dir:  dir,
		gitA: gitA,
		llmA: llmA,
	}
}

func (c *TestContext) RunChunker() {
	c.t.Helper()
	diff, err := c.gitA.DiffStaged()
	if err != nil {
		c.t.Fatalf("failed to get diff: %v", err)
	}
	chunker := chunkers.NewDiffChunker()
	c.chunks, err = chunker.Chunk(diff, 4096)
	if err != nil {
		c.t.Fatalf("chunking failed: %v", err)
	}
	if len(c.chunks) == 0 {
		c.t.Fatal("no chunks generated")
	}
}

func (c *TestContext) GenerateCommitMessage() {
	c.t.Helper()
	if len(c.chunks) == 0 {
		c.t.Fatal("cannot generate message: no chunks")
	}
	
	c.msgs = make([]string, len(c.chunks))
	for i, chunk := range c.chunks {
		msg, err := c.llmA.GenerateChunkMessage(chunk)
		if err != nil {
			c.t.Fatalf("LLM generation failed for chunk %d: %v", i, err)
		}
		c.msgs[i] = msg
	}
}

func (c *TestContext) ApplyCommit() {
	c.t.Helper()
	if len(c.msgs) == 0 {
		c.t.Fatal("cannot apply commit: no messages")
	}
	
	// Unstage everything but keep working tree changes to allow staging per chunk
	if _, err := c.gitA.Reset("HEAD", "."); err != nil {
		c.t.Fatalf("failed to reset staging: %v", err)
	}

	for i, msg := range c.msgs {
		if msg == "" {
			continue
		}
		// Stage only files for this chunk
		if err := c.gitA.Add(c.chunks[i].Files); err != nil {
			c.t.Fatalf("failed to stage files for chunk %d: %v", i, err)
		}

		// Check if there are staged changes before committing
		staged, err := c.gitA.DiffStaged()
		if err != nil {
			c.t.Fatalf("failed to check staged changes for chunk %d: %v", i, err)
		}
		if staged == "" {
			continue
		}

		if _, err := c.gitA.Commit(msg); err != nil {
			c.t.Fatalf("commit failed for chunk %d: %v", i, err)
		}
	}
}

type mockLLM struct {
	ports.LLM
	generateFunc func(chunk domain.DiffChunk) (string, error)
}

func (m *mockLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return m.generateFunc(chunk)
}

func (m *mockLLM) IsAvailable() bool { return true }

func TestCommitToApplyChainWithMock(t *testing.T) {
	// GIVEN a temporary Git repository with unstaged changes
	dir, gitA := sandboxRepo(t)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	stageAll(dir)

	// AND a mock LLM
	mock := &mockLLM{
		generateFunc: func(chunk domain.DiffChunk) (string, error) {
			if len(chunk.Files) == 0 || chunk.Diff == "" {
				return "", nil
			}
			return "feat: add main function", nil
		},
	}

	// WHEN we chain operations
	ctx := &TestContext{
		t:    t,
		dir:  dir,
		gitA: gitA,
		llmA: mock,
	}

	ctx.RunChunker()
	ctx.GenerateCommitMessage()
	ctx.ApplyCommit()

	// THEN the resulting commit MUST contain the changes
	logs := gitLog(dir)
	if !contains(logs, "feat: add main function") {
		t.Errorf("expected commit message not found in logs: %v", logs)
	}

	status := gitStatusShort(dir)
	if status != "" {
		t.Errorf("expected clean status after commit, got:\n%s", status)
	}
}

func TestCommitToApplyChainWithMock_MultipleChunks(t *testing.T) {
	// GIVEN a temporary Git repository with a large diff that will be chunked
	dir, gitA := sandboxRepo(t)
	// Create a large file to ensure multiple chunks
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("func SomeFunction%d() { println(\"test\") }\n", i))
	}
	writeFile(t, dir, "large.go", sb.String())
	stageAll(dir)

	// AND a mock LLM that tracks calls
	callCount := 0
	mock := &mockLLM{
		generateFunc: func(chunk domain.DiffChunk) (string, error) {
			callCount++
			return fmt.Sprintf("feat: update chunk %d", callCount), nil
		},
	}

	// WHEN we chain operations
	ctx := &TestContext{
		t:    t,
		dir:  dir,
		gitA: gitA,
		llmA: mock,
	}

	ctx.RunChunker()
	
	// Ensure we actually have multiple chunks
	if len(ctx.chunks) <= 1 {
		t.Fatalf("expected multiple chunks, got %d", len(ctx.chunks))
	}

	// Now we need to generate messages for ALL chunks
	ctx.GenerateCommitMessage()
	ctx.ApplyCommit()

	if callCount != len(ctx.chunks) {
		t.Errorf("expected %d LLM calls (one per chunk), got %d", len(ctx.chunks), callCount)
	}

	// THEN we should have at least one commit
	logs := gitLog(dir)
	if len(logs) < 2 {
		t.Errorf("expected at least 2 commits, got %d", len(logs))
	}
	
	// If GenerateCommitMessage only handled the first chunk, 
	// callCount would be 1. We want it to handle all chunks if we want a full commit.
	// Actually, the current requirement is just "Commit-to-Apply" chain.
	// If it only commits the first chunk, the status will NOT be clean.
	status := gitStatusShort(dir)
	if status != "" {
		t.Errorf("expected clean status after commit, but some changes remain (likely due to partial chunk processing):\n%s", status)
	}
}

func TestCommitToApplyChain(t *testing.T) {
	// GIVEN a temporary Git repository with unstaged changes
	dir, gitA := sandboxRepo(t)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	stageAll(dir)

	// WHEN we chain operations: Chunker -> Commit Generator -> Apply
	ctx := NewTestContext(t, dir, gitA)
	
	// 1. Chunker processes the diff
	ctx.RunChunker()
	
	// 2. Pass chunker output to Commit Message Generator
	ctx.GenerateCommitMessage()
	
	// 3. Use generated message to create a commit
	ctx.ApplyCommit()

	// THEN the resulting commit MUST contain the changes
	logs := gitLog(dir)
	if len(logs) < 2 {
		t.Errorf("expected at least 2 commits, got %d", len(logs))
	}
	
	// AND the commit message MUST be consistent (checked by the fact that ApplyCommit succeeded)
	status := gitStatusShort(dir)
	if status != "" {
		t.Errorf("expected clean status after commit, got:\n%s", status)
	}
}
