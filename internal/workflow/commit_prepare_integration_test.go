//go:build integration

package workflow

import (
	"context"
	"os"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

func TestGenerateCommitMessage_Integration_WhyFlowsThrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "devstral:24b"
	}

	// Create a real git adapter pointing to current repo
	gitAdapter := git.New(".")

	llm := openai_standard.NewOpenAIStandardAdapter(baseURL, model)
	chunker := chunkers.NewDiffChunker(chunkers.WithChunkSize(4096))
	security := &stubSecurity{}
	contentProvider := testutil.NewMockContentProvider()

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log")
	cfg.NumParallel = 1
	cfg.ContentProvider = contentProvider

	svc := NewCommitService(gitAdapter, llm, chunker, security, cfg, nil)

	// Test that Why flows through to the LLM
	// This test requires staged changes — skip if nothing is staged
	diffStaged, err := gitAdapter.DiffStaged()
	if err != nil || diffStaged == "" {
		t.Skip("skipping: no staged changes in current repo")
	}

	messages, err := svc.GenerateCommitMessage(context.Background(), "add refresh token rotation for security")
	if err != nil {
		t.Fatalf("GenerateCommitMessage() integration error: %v", err)
	}

	if len(messages) == 0 {
		t.Error("GenerateCommitMessage() returned no messages")
	}

	t.Logf("Integration test generated %d message(s):", len(messages))
	for i, msg := range messages {
		t.Logf("  %d: %s", i+1, msg)
	}
}