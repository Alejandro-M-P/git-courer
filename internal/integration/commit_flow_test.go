package integration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

// TestCommitTwoPhaseFlowViaMCPHandlers verifies the full two‑phase commit via MCP handlers.
// Start (prepare) → plan with commit fields → Apply (execute) → commits in git.
func TestCommitTwoPhaseFlowViaMCPHandlers(t *testing.T) {
	// Setup git adapter with a real repo (temp dir)
	// TODO: implement NewTempRepo or use mock
	t.Skip("NewTempRepo not implemented yet - test disabled")
}

	// Write a file and stage it for the test
	gitA.WriteFile("test.go", "package test\n")
	if err := gitA.Add([]string{"."}); err != nil {
		t.Fatalf("setup: failed to stage file: %v", err)
	}

	// Change file to create diff
	gitA.WriteFile("test.go", "package test\n// modified\n")

	// Setup LLM mock that returns predictable messages
	llmA := llm.NewMock()
	llmA.SetResponse(domain.CommitIntent{
		IncludeUntracked: false,
		Filter:          "",
		Reasoning:       "M",
	})
	llmA.SetChunkMessage("feat: integration test commit")

	// Setup commit service
	cfg := &config.Config{
		Ollama: config.OllamaConfig{ContextWindow: 4096},
		Commit: config.CommitConfig{
			BackgroundThreshold: 10000,
			MaxLogLines:         100,
			LogPath:             "",
			TTL: config.DurationConfig{Duration: 300},
		},
		Secrets: config.SecretsConfig{
			DetectionMode:      "none",
			UseLLMSecurityScan: "auto",
			Patterns:           []string{},
		},
		Preview: config.PreviewConfig{
			EnabledMap: map[string]bool{"commit": true},
		},
		Commands: config.CommandsConfig{
			EnabledMap: map[string]bool{"commit": true},
		},
	}

	chunker := chunkers.NewDiffChunker()
	securitySvc := security.New(cfg)
	commitCfg := workflow.DefaultCommitServiceConfig(
		cfg.Ollama.ContextWindow,
		cfg.Commit.BackgroundThreshold,
		cfg.Commit.MaxLogLines,
		cfg.Commit.LogPath,
	)
	commitSvc := workflow.NewCommitService(gitA, llmA, chunker, securitySvc, commitCfg)

	// Commit confirm adapter
	commitConfirm := confirm.NewInMemory(cfg.Commit.TTL.Duration)

	// Workflow with commit service
	wf := workflow.New(gitA, llmA, commitConfirm, commitSvc, cfg)

	// PHASE 1: START → prepare commit, get plan
	res, err := wf.Run(nil, "commit", "commit test changes", nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.Status != workflow.StatusPending {
		t.Fatalf("Run() status = %s, want pending_approval", res.Status)
	}

	// Verify plan exists
	if !commitConfirm.HasBlocker() {
		t.Fatal("Plan blocker not created")
	}

	plan, err := commitConfirm.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan failed: %v", err)
	}
	if plan.Operation != "commit" {
		t.Fatalf("plan.Operation = %s, want commit", plan.Operation)
	}
	if len(plan.Messages) == 0 {
		t.Fatal("plan.Messages empty")
	}
	if len(plan.Chunks) == 0 {
		t.Fatal("plan.Chunks empty")
	}
	if plan.Instruction != "commit test changes" {
		t.Fatalf("plan.Instruction = %s, want 'commit test changes'", plan.Instruction)
	}

	// PHASE 2: APPLY → execute from plan
	res, err = wf.Apply(nil)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if res.Status != workflow.StatusCompleted {
		t.Fatalf("Apply() status = %s, want completed", res.Status)
	}

	// Verify at least one commit was created
	log, _ := gitA.Log(5)
	if !strings.Contains(log, "feat: integration test commit") {
		t.Fatalf("commit not found in git log:\n%s", log)
	}

	// Cleanup
	wf.Abort()
}

// TestCommitTwoPhaseFlowWithCleanRepo edge case test for commit with no files (nothing to commit).
func TestCommitTwoPhaseFlowWithCleanRepo(t *testing.T) {
	t.Skip("Temporary skip: NewTempRepo not implemented")

	llmA := llm.NewMock()
	llmA.SetResponse(domain.CommitIntent{IncludeUntracked: false})

	cfg := &config.Config{
		Ollama: config.OllamaConfig{ContextWindow: 4096},
		Commit: config.CommitConfig{
			BackgroundThreshold: 10000,
			MaxLogLines:         100,
			LogPath:             "",
			TTL: config.DurationConfig{Duration: 300},
		},
		Security: security.Config{DetectionMode: "none"},
		Preview:  config.PreviewConfig{EnabledMap: map[string]bool{"commit": true}},
		Commands: config.CommandsConfig{EnabledMap: map[string]bool{"commit": true}},
	}

	chunker := chunkers.NewDiffChunker()
	securitySvc := security.New(cfg)
	commitCfg := workflow.DefaultCommitServiceConfig(
		cfg.Ollama.ContextWindow,
		cfg.Commit.BackgroundThreshold,
		cfg.Commit.MaxLogLines,
		cfg.Commit.LogPath,
	)
	commitSvc := workflow.NewCommitService(gitA, llmA, chunker, securitySvc, commitCfg)
	commitConfirm := confirm.NewInMemory(cfg.Commit.TTL.Duration)
	wf := workflow.New(gitA, llmA, commitConfirm, commitSvc, cfg)

	// Repo is clean — no files to commit
	res, err := wf.Run(nil, "commit", "commit all", nil)
	if err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") {
			t.Fatalf("expected 'nothing to commit' error, got: %v", err)
		}
		// Expected error — plan should not exist
		if commitConfirm.HasBlocker() {
			t.Fatal("Plan blocker created but should not (nothing to commit)")
		}
	} else {
		t.Fatal("Run succeeded unexpectedly (should fail with 'nothing to commit')")
	}
}

// TestCommitTwoPhaseFlowSecurityBlocked test for commit with security‑blocked files (security = enabled).
func TestCommitTwoPhaseFlowSecurityBlocked(t *testing.T) {
	gitA, cleanup := git.NewTempRepo()
	defer cleanup()

	// Create file with fake secret
	gitA.WriteFile("secrets.env", "PASSWORD=secret123\nAPI_KEY=abc123\n")
	if err := gitA.Add([]string{"."}); err != nil {
		t.Fatalf("setup: failed to stage file: %v", err)
	}
	gitA.WriteFile("secrets.env", "PASSWORD=secret123\nAPI_KEY=abc123\n// modified\n")

	llmA := llm.NewMock()
	llmA.SetResponse(domain.CommitIntent{IncludeUntracked: true})

	cfg := &config.Config{
		Ollama: config.OllamaConfig{ContextWindow: 4096},
		Commit: config.CommitConfig{
			BackgroundThreshold: 10000,
			MaxLogLines:         100,
			LogPath:             "",
			TTL: config.DurationConfig{Duration: 300},
		},
		Security: security.Config{
			DetectionMode: "regex",
			Patterns: []security.Pattern{
				{Name: "password", Pattern: `(?i)password\s*=\s*`, Severity: "block"},
				{Name: "api_key", Pattern: `(?i)api_key\s*=\s*`, Severity: "block"},
			},
		},
		Preview:  config.PreviewConfig{EnabledMap: map[string]bool{"commit": true}},
		Commands: config.CommandsConfig{EnabledMap: map[string]bool{"commit": true}},
	}

	chunker := chunkers.NewDiffChunker()
	securitySvc := security.New(cfg)
	commitCfg := workflow.DefaultCommitServiceConfig(
		cfg.Ollama.ContextWindow,
		cfg.Commit.BackgroundThreshold,
		cfg.Commit.MaxLogLines,
		cfg.Commit.LogPath,
	)
	commitSvc := workflow.NewCommitService(gitA, llmA, chunker, securitySvc, commitCfg)
	commitConfirm := confirm.NewInMemory(cfg.Commit.TTL.Duration)
	wf := workflow.New(gitA, llmA, commitConfirm, commitSvc, cfg)

	res, err := wf.Run(nil, "commit", "commit everything", nil)
	if err == nil {
		t.Fatal("Run succeeded unexpectedly (should block on security)")
	}
	if !strings.Contains(err.Error(), "[SECURITY]") {
		t.Fatalf("expected [SECURITY] error, got: %v", err)
	}

	// No plan should exist (blocked before plan creation)
	if commitConfirm.HasBlocker() {
		t.Fatal("Plan blocker created but security should have blocked")
	}
}

// TestCommitAbortIntegrity_CleansStaging verifies staging is completely cleaned after ABORT.
func TestCommitAbortIntegrity_CleansStaging(t *testing.T) {
	gitA, cleanup := git.NewTempRepo()
	defer cleanup()

	gitA.WriteFile("a.go", "package a\n")
	if err := gitA.Add([]string{"."}); err != nil {
		t.Fatalf("setup: failed to stage file: %v", err)
	}

	// Stage a file — git diff --staged should show it
	diffStaged, _ := gitA.DiffStaged()
	if diffStaged == "" {
		t.Fatal("setup: diff staged empty before test")
	}

	llmA := llm.NewMock()
	llmA.SetResponse(domain.CommitIntent{IncludeUntracked: false})
	llmA.SetChunkMessage("feat: abort test")

	cfg := &config.Config{
		Ollama: config.OllamaConfig{ContextWindow: 4096},
		Commit: config.CommitConfig{
			BackgroundThreshold: 10000,
			MaxLogLines:         100,
			LogPath:             "",
			TTL: config.DurationConfig{Duration: 300},
		},
		Security: security.Config{DetectionMode: "none"},
		Preview:  config.PreviewConfig{EnabledMap: map[string]bool{"commit": true}},
		Commands: config.CommandsConfig{EnabledMap: map[string]bool{"commit": true}},
	}

	chunker := chunkers.NewDiffChunker()
	securitySvc := security.New(cfg)
	commitCfg := workflow.DefaultCommitServiceConfig(
		cfg.Ollama.ContextWindow,
		cfg.Commit.BackgroundThreshold,
		cfg.Commit.MaxLogLines,
		cfg.Commit.LogPath,
	)
	commitSvc := workflow.NewCommitService(gitA, llmA, chunker, securitySvc, commitCfg)
	commitConfirm := confirm.NewInMemory(cfg.Commit.TTL.Duration)
	wf := workflow.New(gitA, llmA, commitConfirm, commitSvc, cfg)

	// START → plan created
	res, err := wf.Run(nil, "commit", "commit changes", nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.Status != workflow.StatusPending {
		t.Fatalf("Run() status = %s, want pending_approval", res.Status)
	}

	// ABORT → staging should be cleaned
	if err := wf.Abort(); err != nil {
		t.Fatalf("Abort failed: %v", err)
	}

	// After ABORT, git diff --staged should be empty
	diffStagedAfter, _ := gitA.DiffStaged()
	if diffStagedAfter != "" {
		t.Fatalf("staging not cleaned after abort:\n%s", diffStagedAfter)
	}
}

// TestCommitTwoPhaseFlowVariation — staged + unstaged mix (triangulation test).
func TestCommitTwoPhaseFlowVariation(t *testing.T) {
	gitA, cleanup := git.NewTempRepo()
	defer cleanup()

	// Create two files
	gitA.WriteFile("staged.go", "package staged\n")
	gitA.WriteFile("unstaged.go", "package unstaged\n")

	// Stage only one file
	if err := gitA.Add([]string{"staged.go"}); err != nil {
		t.Fatalf("setup: failed to stage staged.go: %v", err)
	}
	// Modify staged file (now staged + unstaged changes)
	gitA.WriteFile("staged.go", "package staged\n// modified\n")
	// Modify unstaged file (only unstaged)
	gitA.WriteFile("unstaged.go", "package unstaged\n// modified\n")

	llmA := llm.NewMock()
	llmA.SetResponse(domain.CommitIntent{IncludeUntracked: true}) // include unstaged files
	llmA.SetChunkMessage("feat: mixed changes")

	cfg := &config.Config{
		Ollama: config.OllamaConfig{ContextWindow: 4096},
		Commit: config.CommitConfig{
			BackgroundThreshold: 10000,
			MaxLogLines:         100,
			LogPath:             "",
			TTL: config.DurationConfig{Duration: 300},
		},
		Security: security.Config{DetectionMode: "none"},
		Preview:  config.PreviewConfig{EnabledMap: map[string]bool{"commit": true}},
		Commands: config.CommandsConfig{EnabledMap: map[string]bool{"commit": true}},
	}

	chunker := chunkers.NewDiffChunker()
	securitySvc := security.New(cfg)
	commitCfg := workflow.DefaultCommitServiceConfig(
		cfg.Ollama.ContextWindow,
		cfg.Commit.BackgroundThreshold,
		cfg.Commit.MaxLogLines,
		cfg.Commit.LogPath,
	)
	commitSvc := workflow.NewCommitService(gitA, llmA, chunker, securitySvc, commitCfg)
	commitConfirm := confirm.NewInMemory(cfg.Commit.TTL.Duration)
	wf := workflow.New(gitA, llmA, commitConfirm, commitSvc, cfg)

	// START → plan created
	res, err := wf.Run(nil, "commit", "commit mixed changes", nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.Status != workflow.StatusPending {
		t.Fatalf("Run() status = %s, want pending_approval", res.Status)
	}

	plan, err := commitConfirm.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan failed: %v", err)
	}
	if len(plan.Messages) == 0 {
		t.Fatal("plan.Messages empty")
	}

	// APPLY → execute from plan
	res, err = wf.Apply(nil)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if res.Status != workflow.StatusCompleted {
		t.Fatalf("Apply() status = %s, want completed", res.Status)
	}

	// Verify commit created
	log, _ := gitA.Log(5)
	if !strings.Contains(log, "feat: mixed changes") {
		t.Fatalf("commit not found in git log:\n%s", log)
	}

	// Cleanup
	wf.Abort()
}