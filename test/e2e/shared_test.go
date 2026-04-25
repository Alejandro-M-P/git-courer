//go:build e2e
// +build e2e

// Package e2e contains end-to-end stress and security tests.
// Shared helpers for the e2e test suite.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

// LLMConfig holds the Ollama connection configuration.
var LLMHost   = os.Getenv("OLLAMA_HOST")
var LLMModel  = os.Getenv("OLLAMA_MODEL")
var LLMApiKey = os.Getenv("OLLAMA_API_KEY")

func init() {
	if LLMHost == "" {
		LLMHost = "http://localhost:11434"
	}
	if LLMModel == "" {
		LLMModel = "qwen3.5:0.8b"
	}
}

// requireOllama skips the test if Ollama is not reachable, pre-warms model.
func requireOllama(t *testing.T) ports.LLM {
	t.Helper()
	adapter := llm.New(LLMHost, LLMModel, LLMApiKey)
	if !adapter.IsAvailable() {
		t.Skip("Ollama not running — start with: ollama serve")
	}
	// Pre-warm: load model into memory so subsequent tests run fast
	if err := adapter.PreWarm(); err != nil {
		t.Logf("PreWarm warning: %v", err)
	}
	return adapter
}

// sandboxRepo creates an isolated git repo in a temp dir with proper config.
// The repo has one initial commit so HEAD always exists.
func sandboxRepo(t *testing.T) (string, *git.ExecAdapter) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@git-courer.test")
	run("config", "user.name", "Git Courer Test")
	run("config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "chore: initial commit")

	return dir, git.New(dir)
}

// makeCommitSvc builds a CommitService wired to the given dependencies.
func makeCommitSvc(t *testing.T, gitA ports.Git, llmA ports.LLM, sec ports.SecurityService, dir string) *workflow.CommitService {
	t.Helper()
	cfg := workflow.DefaultCommitServiceConfig(
		4096,
		50,
		filepath.Join(dir, ".gcourer", "commit.log"),
	)
	return workflow.NewCommitService(gitA, llmA, chunkers.NewDiffChunker(), sec, cfg)
}

// makeWorkflow builds a Workflow with all services configured.
func makeWorkflow(t *testing.T, gitA ports.Git, llmA ports.LLM) (*workflow.Workflow, *confirm.InMemoryConfirm) {
	t.Helper()
	cfg := config.Default()
	c := confirm.NewInMemory(5 * time.Minute)
	sec := security.New(cfg, llmA)
	commitCfg := workflow.DefaultCommitServiceConfig(4096, 50, filepath.Join(t.TempDir(), "commit.log"))
	commit := workflow.NewCommitService(gitA, llmA, chunkers.NewDiffChunker(), sec, commitCfg)
	releaseCfg := workflow.DefaultReleaseServiceConfig(4096, 20, 50, filepath.Join(t.TempDir(), "release.log"))
	release := workflow.NewReleaseService(gitA, llmA, chunkers.NewLogChunker(4096), releaseCfg)
	return workflow.New(gitA, llmA, c, cfg, commit, release, sec), c
}

// ============================================================================
// Shared mock types
// ============================================================================

// errorLLM is an LLM that immediately returns an error on every call.
type errorLLM struct{ msg string }

func (e *errorLLM) GenerateChunkMessage(_ domain.DiffChunk) (string, error) {
	return "", fmt.Errorf("%s", e.msg)
}
func (e *errorLLM) DecideCommit(_, _, _, _, _ string) (domain.CommitIntent, error) {
	return domain.CommitIntent{}, fmt.Errorf("%s", e.msg)
}
func (e *errorLLM) InterpretGitOp(_, _ string, _ map[string]string) (map[string]string, error) {
	return nil, fmt.Errorf("%s", e.msg)
}
func (e *errorLLM) SetRetryContext(_ string)                             {}
func (e *errorLLM) ClearRetryContext()                                   {}
func (e *errorLLM) IsAvailable() bool                                     { return false }
func (e *errorLLM) VerifySecrets(_ string, _ []domain.SecretDetection) (bool, error) {
	return false, fmt.Errorf("%s", e.msg)
}
func (e *errorLLM) AuditBinaryContent(_, _ string) (bool, error) {
	return false, fmt.Errorf("%s", e.msg)
}
func (e *errorLLM) GenerateChangelog(_, _, _ string) (string, error) { return "", fmt.Errorf("%s", e.msg) }


// noOpSecurity is a security service that never blocks.
type noOpSecurity struct{}

func (n *noOpSecurity) CheckFiles(_ []string, _ string) *ports.SecurityCheckResult {
	return &ports.SecurityCheckResult{Blocked: false, Files: []ports.SecurityResult{}}
}
func (n *noOpSecurity) ShouldUseLLMScan() bool { return false }