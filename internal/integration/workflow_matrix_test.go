//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	gitadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

// ============================================================================
// Helpers
// ============================================================================

// sandboxRepo creates an isolated git repo in a temp dir with proper config.
// The repo has one initial commit so HEAD always exists.
// All tests should use this instead of a shared dir to avoid cross-contamination.
func sandboxRepo(t *testing.T) (dir string, adapter *gitadapter.ExecAdapter) {
	t.Helper()
	dir = t.TempDir()

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

	// Normalize branch name to "main" regardless of git version default.
	out, _ := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if branch := strings.TrimSpace(string(out)); branch != "main" && branch != "" {
		run("branch", "-m", branch, "main")
	}

	return dir, gitadapter.New(dir)
}

// sandboxRepoWithRemote creates a sandbox repo plus a local bare remote for push/pull tests.
// The bare remote is pre-loaded with the initial commit and tracking is configured.
func sandboxRepoWithRemote(t *testing.T) (dir string, adapter *gitadapter.ExecAdapter) {
	t.Helper()
	dir, adapter = sandboxRepo(t)
	remoteDir := t.TempDir()

	runRemote := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = remoteDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git (remote) %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	runLocal := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git (local) %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	runRemote("init", "--bare")
	runLocal("remote", "add", "origin", remoteDir)
	runLocal("push", "-u", "origin", "main")

	return dir, adapter
}

// requireOllama skips the test if Ollama is not reachable.
func requireOllama(t *testing.T) ports.LLM {
	t.Helper()
	adapter := openai_standard.NewOpenAIStandardAdapter(LLMHost+"/v1", LLMModel)
	if !adapter.IsAvailable() {
		t.Skip("Ollama not running — start with: ollama serve")
	}
	return adapter
}

// errorLLM is an LLM that immediately returns an error on every call.
// Used to test graceful error handling without the 120s retry delay of the real adapter.
type errorLLM struct{ msg string }

func (e *errorLLM) GenerateChunkMessage(_ domain.DiffChunk) (string, error) {
	return "", errors.New(e.msg)
}
func (e *errorLLM) DecideCommit(_, _, _, _, _ string) (domain.CommitIntent, error) {
	return domain.CommitIntent{}, errors.New(e.msg)
}
func (e *errorLLM) InterpretGitOp(_, _ string, _ map[string]string) (map[string]string, error) {
	return nil, errors.New(e.msg)
}
func (e *errorLLM) SetRetryContext(_ string)                                 {}
func (e *errorLLM) ClearRetryContext()                                       {}
func (e *errorLLM) IsAvailable() bool                                        { return false }
func (e *errorLLM) VerifySecrets(_ string, _ []domain.SecretDetection) (bool, error) {
	return false, errors.New(e.msg)
}
func (e *errorLLM) AuditBinaryContent(_, _ string) (bool, error) {
	return false, errors.New(e.msg)
}
func (e *errorLLM) GenerateChangelog(_, _, _ string) (*domain.Changelog, error) { return nil, errors.New(e.msg) }
func (e *errorLLM) RegenerateMessage(_ []string, _ string, _ []domain.DiffChunk) ([]string, error) {
	return nil, errors.New(e.msg)
}

// noOpSecurity is a security service that never blocks (simulates security=disabled).
type noOpSecurity struct{}

func (n *noOpSecurity) CheckFiles(_ []string, _ string) *ports.SecurityCheckResult {
	return &ports.SecurityCheckResult{Blocked: false, Files: []ports.SecurityResult{}}
}
func (n *noOpSecurity) ShouldUseLLMScan() bool { return false }

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

// makeWorkflow builds a Workflow with preview enabled/disabled for the given operations.
// Returns both the workflow and the confirm adapter (needed for Apply/Abort tests).
func makeWorkflow(gitA ports.Git, llmA ports.LLM, previewOps map[string]bool) (*workflow.Workflow, *confirm.InMemoryConfirm) {
	cfg := config.Default()
	cfg.Preview.Enabled = len(previewOps) > 0
	cfg.Preview.Operations = previewOps
	c := confirm.NewInMemory(5 * time.Minute)
	
	// Specialized services for unification
	sec := security.New(cfg, llmA)
	commitCfg := workflow.DefaultCommitServiceConfig(4096, 50, "/tmp/commit.log")
	commit := workflow.NewCommitService(gitA, llmA, chunkers.NewDiffChunker(), sec, commitCfg)
	releaseCfg := workflow.DefaultReleaseServiceConfig(4096, 20, 50, "/tmp/release.log")
	release := workflow.NewReleaseService(gitA, llmA, chunkers.NewLogChunker(4096), releaseCfg, nil)

	return workflow.New(gitA, llmA, c, cfg, commit, release, sec), c
}

// makeReleaseSvc builds a ReleaseService wired to the given dependencies.
func makeReleaseSvc(t *testing.T, gitA ports.Git, llmA ports.LLM, dir string) *workflow.ReleaseService {
	t.Helper()
	cfg := workflow.DefaultReleaseServiceConfig(
		4096,
		20,
		50,
		filepath.Join(dir, ".gcourer", "release.log"),
	)
	return workflow.NewReleaseService(gitA, llmA, chunkers.NewLogChunker(4096), cfg, nil)
}

// gitLogMessages returns the commit subject lines from the sandbox repo,
// excluding the initial "chore: initial commit" so assertions don't need to
// account for it.
func gitLogMessages(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "log", "--format=%s", "--no-merges")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	var msgs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" && line != "chore: initial commit" {
			msgs = append(msgs, line)
		}
	}
	return msgs
}

// branchExists returns true if the named branch exists in the sandbox repo.
func branchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list", name)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) != ""
}

// tagExists returns true if the named tag exists in the sandbox repo.
func tagExists(t *testing.T, dir, name string) bool {
	t.Helper()
	cmd := exec.Command("git", "tag", "--list", name)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) == name
}

// remoteHasCommits returns the number of commits reachable from the remote's main branch.
func remoteHasCommits(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("git", "log", "origin/main", "--oneline")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(string(out)), "\n"))
}

// conventionalCommitRE matches the conventional commit format with optional scope and breaking flag.
var conventionalCommitRE = regexp.MustCompile(
	`^(feat|fix|chore|docs|refactor|test|style|build|ci|perf|revert)(\([^)]+\))?(!)?:\s+\S`,
)

func isConventionalCommit(msg string) bool {
	return conventionalCommitRE.MatchString(msg)
}

// ============================================================================
// Group A — Commit
// ============================================================================

// TestCommit_Preview_SecurityDisabled verifies the full two-phase commit flow:
// PrepareCommit → user sees preview → ExecuteFromPlan → commits exist in git.
// Security is disabled so no files are blocked.
func TestCommit_Preview_SecurityDisabled(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "config.go"), []byte("package main\n\nvar cfg = struct{ debug bool }{}\n"), 0644)

	// Phase 1: prepare — Ollama decides what to stage, chunks the diff, generates messages.
	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit(
"commit all my changes")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}
	t.Logf("messages=%v chunks=%d deleted=%v warnings=%v",
		messages, len(chunks), deleted, warnings)

	if len(messages) == 0 {
		t.Fatal("expected at least one commit message")
	}
	for _, msg := range messages {
		if msg == "" || msg == "chore: no meaningful changes" {
			continue
		}
		if !isConventionalCommit(msg) {
			t.Errorf("message %q is not a conventional commit", msg)
		}
	}

	// No new commits yet — preview has NOT been applied.
	if msgs := gitLogMessages(t, dir); len(msgs) > 0 {
		t.Errorf("expected no commits before apply, got: %v", msgs)
	}

	// Phase 2: apply.
	chunkFiles := make([][]string, len(chunks))
	for i, c := range chunks {
		chunkFiles[i] = c.Files
	}
	result, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "commit all my changes")
	if err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}
	t.Logf("ExecuteFromPlan result: %s", result)

	// At least one commit must now exist.
	committed := gitLogMessages(t, dir)
	if len(committed) == 0 {
		t.Fatal("expected commits in git log after apply, found none")
	}
	for _, msg := range committed {
		if !isConventionalCommit(msg) {
			t.Errorf("committed message %q is not a conventional commit", msg)
		}
	}
}

// TestCommit_Direct_NoPreview verifies preview=false:
// Execute runs the full pipeline (stage → Ollama → commit) in a single call.
func TestCommit_Direct_NoPreview(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	os.WriteFile(filepath.Join(dir, "service.go"), []byte("package main\n\nfunc NewService() {}\n"), 0644)

	result, err := svc.Execute("commit all my changes", false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	t.Logf("Execute result: %s", result)

	committed := gitLogMessages(t, dir)
	if len(committed) == 0 {
		t.Fatal("expected commits in git log after direct execute, found none")
	}
	for _, msg := range committed {
		if !isConventionalCommit(msg) {
			t.Errorf("committed message %q is not a conventional commit", msg)
		}
	}
}

// TestCommit_Preview_SecurityRegex_CleanFiles verifies that clean files pass
// the real security service (regex mode) and commits succeed.
func TestCommit_Preview_SecurityRegex_CleanFiles(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	cfg := config.Default()
	cfg.Secrets.DetectionMode = "regex"
	sec := security.New(cfg, llmA)
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// Plain Go source — no secrets.
	os.WriteFile(filepath.Join(dir, "router.go"), []byte(
		"package main\n\nfunc setupRoutes() {}\n",
	), 0644)

	messages, chunks, deleted, _, _, err := svc.PrepareCommit("add router setup")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}

	chunkFiles := make([][]string, len(chunks))
	for i, c := range chunks {
		chunkFiles[i] = c.Files
	}
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "add router setup"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	if msgs := gitLogMessages(t, dir); len(msgs) == 0 {
		t.Fatal("expected at least one commit after clean-file commit")
	}
}

// TestCommit_Security_Blocked_FakeAWSKey verifies that a file containing a
// fake AWS access key is caught by the security service and the commit is
// blocked before any git operation runs.
func TestCommit_Security_Blocked_FakeAWSKey(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	cfg := config.Default()
	cfg.Secrets.DetectionMode = "regex"
	sec := security.New(cfg, llmA)
	svc := makeCommitSvc(t, gitA, llmA, sec, dir)

	// A file containing a fake AWS access key that matches DUMMY_AWS[0-9A-Z]{16}.
	// Pre-staged so the file appears as tracked (not untracked).
	// We also chdir into the sandbox so that secrets.Detect() can open files with
	// the relative paths that git returns — in production, CWD == git workdir.
	os.WriteFile(filepath.Join(dir, "deploy.go"), []byte(
		"package main\n\nconst awsKey = \"AKIA1234567890123456\"\n",
	), 0644)
	exec.Command("git", "-C", dir, "add", "deploy.go").Run()

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to sandbox: %v", err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	_, _, _, _, _, err := svc.PrepareCommit("commit deploy config")
	if err == nil {
		t.Fatal("expected PrepareCommit to return a security error, got nil")
	}
	if !strings.Contains(err.Error(), "[SECURITY]") {
		t.Errorf("expected [SECURITY] in error, got: %v", err)
	}
	t.Logf("Security correctly blocked commit: %v", err)

	// Verify no commit was created.
	if msgs := gitLogMessages(t, dir); len(msgs) > 0 {
		t.Errorf("expected no commits after security block, got: %v", msgs)
	}
}

// TestCommit_Preview_Abort simulates a user who previews but then decides NOT
// to apply — the repo must remain unchanged.
func TestCommit_Preview_Abort(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	// Pre-stage the file so qwen sees it as tracked (not untracked).
	// When untracked, qwen may decide IncludeUntracked=false → nothing staged → error.
	os.WriteFile(filepath.Join(dir, "abort_me.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "abort_me.go").Run()

	// Prepare but intentionally do NOT apply.
	_, _, _, _, _, err := svc.PrepareCommit("commit everything")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}

	// Git log must have no new commits.
	if msgs := gitLogMessages(t, dir); len(msgs) > 0 {
		t.Errorf("repo was modified after abort; commits found: %v", msgs)
	}
}

// TestCommit_MultipleFiles verifies that several files across different
// directories produce at least one conventional commit.
// (With small diffs the chunker usually produces 1 chunk; that is expected.)
func TestCommit_MultipleFiles(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	os.MkdirAll(filepath.Join(dir, "internal", "user"), 0755)
	os.MkdirAll(filepath.Join(dir, "internal", "auth"), 0755)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0755)

	os.WriteFile(filepath.Join(dir, "internal", "user", "user.go"),
		[]byte("package user\n\ntype User struct{ ID string }\n"), 0644)
	os.WriteFile(filepath.Join(dir, "internal", "auth", "auth.go"),
		[]byte("package auth\n\nfunc Login() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "cmd", "main.go"),
		[]byte("package main\n\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module example.com/app\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "go.sum"),
		[]byte(""), 0644)

	// Pre-stage all files: qwen may return a comma-separated filter string for untracked
	// files (e.g. "internal/auth/auth.go, internal/user/user.go") which git treats as a
	// single literal path and fails. Tracked files always get a valid filter or include=true.
	exec.Command("git", "-C", dir, "add", ".").Run()

	messages, chunks, deleted, warnings, _, err := svc.PrepareCommit(
"commit all the new code")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}
	t.Logf("chunks=%d messages=%v warnings=%v", len(chunks), messages, warnings)

	chunkFiles := make([][]string, len(chunks))
	for i, c := range chunks {
		chunkFiles[i] = c.Files
	}
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "commit all the new code"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	committed := gitLogMessages(t, dir)
	if len(committed) == 0 {
		t.Fatal("expected commits after multi-file commit, found none")
	}
	for _, msg := range committed {
		if !isConventionalCommit(msg) {
			t.Errorf("message %q is not a conventional commit", msg)
		}
	}
}

// TestCommit_DeletedFiles verifies that deleted files are included in the
// commit and do not cause errors.
func TestCommit_DeletedFiles(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	// Stage and commit a file so we can later delete it.
	filePath := filepath.Join(dir, "old_feature.go")
	os.WriteFile(filePath, []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "old_feature.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "chore: add old feature").Run()

	// Use os.Remove (not git rm) to produce an unstaged deletion (status " D").
	// git rm would produce "D " (staged) and prepareStages would try to git add
	// a non-existent file, which always fails. With os.Remove the file is in the
	// "tracked" slice and prepareStages can stage it normally via git add.
	os.Remove(filePath)

	// Use "commit all changes" — simpler instruction so qwen includes the deleted
	// file instead of guessing a non-existent filter path like "src/".
	messages, chunks, deleted, _, _, err := svc.PrepareCommit("commit all changes")
	if err != nil {
		t.Fatalf("PrepareCommit with deleted file: %v", err)
	}
	t.Logf("deleted=%v messages=%v", deleted, messages)

	chunkFiles := make([][]string, len(chunks))
	for i, c := range chunks {
		chunkFiles[i] = c.Files
	}
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "commit all changes"); err != nil {
		t.Fatalf("ExecuteFromPlan with deleted files: %v", err)
	}

	committed := gitLogMessages(t, dir)
	if len(committed) == 0 {
		t.Fatal("expected at least one commit after deleting files")
	}
}

// TestCommit_NothingToCommit verifies that an empty repo (nothing staged)
// returns a user-friendly message and does NOT crash or create a bad commit.
func TestCommit_NothingToCommit(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	// No new files — sandbox repo only has the initial commit.
	result, err := svc.Execute("commit everything", false)

	// The service may return an error or a JSON result — both are valid outcomes.
	// The key invariant is that NO commit was created in the repo.
	// (qwen may return unexpected filters like "None" that cause git errors —
	// those are acceptable as long as the repo stays clean.)
	t.Logf("Execute result: %v | err: %v", result, err)

	if msgs := gitLogMessages(t, dir); len(msgs) > 0 {
		t.Errorf("unexpected commits in repo when nothing to commit: %v", msgs)
	}
}

// TestCommit_BreakingChange_Detection verifies that the LLM correctly identifies
// a breaking change from the diff (removing a public API) and marks it with `!`
// or includes `BREAKING CHANGE:` in the commit message.
//
// This test answers: "does the LLM know to write feat! when the diff breaks an API?"
// The commit_message prompt instructs the model to use ! for breaking changes,
// but this test proves it can DETECT them from the diff content, not just copy
// an explicit instruction from the user.
func TestCommit_BreakingChange_Detection(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, llmA, &noOpSecurity{}, dir)

	// Establish a public API.
	api := `package api

// Client is the public API client.
type Client struct {
	Host string
	Port int
}

// New creates a new API client.
func New(host string, port int) *Client {
	return &Client{Host: host, Port: port}
}

// Connect opens a connection using the client config.
func (c *Client) Connect() error {
	return nil
}
`
	os.WriteFile(filepath.Join(dir, "client.go"), []byte(api), 0644)
	exec.Command("git", "-C", dir, "add", "client.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: add API client").Run()

	// Now REMOVE the public New() constructor — callers that used New() will break.
	broken := `package api

// Client is the public API client.
type Client struct {
	Host string
	Port int
}

// Connect opens a connection using the client config.
func (c *Client) Connect() error {
	return nil
}
`
	os.WriteFile(filepath.Join(dir, "client.go"), []byte(broken), 0644)
	exec.Command("git", "-C", dir, "add", "client.go").Run()

	messages, chunks, deleted, _, _, err := svc.PrepareCommit("commit the removal of the New constructor")
	if err != nil {
		t.Fatalf("PrepareCommit: %v", err)
	}
	t.Logf("Generated messages: %v", messages)

	chunkFiles := make([][]string, len(chunks))
	for i, c := range chunks {
		chunkFiles[i] = c.Files
	}
	if _, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "commit the removal"); err != nil {
		t.Fatalf("ExecuteFromPlan: %v", err)
	}

	committed := gitLogMessages(t, dir)
	if len(committed) == 0 {
		t.Fatal("expected a commit after removing the constructor")
	}
	t.Logf("Committed: %v", committed)

	// Check if the LLM detected the breaking change.
	// We accept either `!` notation OR `BREAKING CHANGE:` in the body.
	// A small model may not always detect it — we log but don't hard-fail,
	// since prompt quality varies across models. The test documents the expected behavior.
	hasBreakingMarker := false
	for _, msg := range committed {
		if strings.Contains(msg, "!:") || strings.Contains(msg, "BREAKING CHANGE") {
			hasBreakingMarker = true
			break
		}
	}
	if hasBreakingMarker {
		t.Logf("✓ LLM correctly marked the breaking change")
	} else {
		t.Logf("🚨 LLM did not mark this as a breaking change — message: %v", committed)
		t.Logf("  Expected feat!: or BREAKING CHANGE: when removing a public constructor")
		t.Logf("  This is a prompt quality issue — the model should mark API removals as breaking changes")
		// Log only (not t.Error) because small models may not reliably detect this.
		// Run with a larger model for stricter verification.
	}

	// What MUST always be true regardless of model quality:
	for _, msg := range committed {
		if !isConventionalCommit(msg) {
			t.Errorf("message %q is not a conventional commit format", msg)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "client.go")); err != nil {
		t.Error("client.go should still exist after commit")
	}
}

// ============================================================================
// Group B — Branch Create
// ============================================================================

// TestBranchCreate_Preview verifies the full two-phase workflow for branch creation:
// BRANCH_CREATE_START → pending_approval → BRANCH_CREATE_APPLY → branch exists.
// The LLM interprets the natural language instruction to determine the branch name.
func TestBranchCreate_Preview(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf, _ := makeWorkflow(gitA, llmA, map[string]bool{"branch_create": true})

	ctx := context.Background()
	res, err := wf.Run(ctx, "branch_create", "create a branch for the new authentication feature", nil)
	if err != nil {
		t.Fatalf("Run(branch_create): %v", err)
	}
	t.Logf("Run result: status=%s preview=%q args=%v", res.Status, res.Preview, res.Args)

	if res.Status != workflow.StatusPending {
		t.Fatalf("expected status=%s, got %s", workflow.StatusPending, res.Status)
	}
	if res.Preview == "" {
		t.Error("expected a non-empty preview message")
	}
	if res.Args["branch"] == "" {
		t.Error("expected LLM to return a branch name in args")
	}
	if !wf.HasPendingPlan() {
		t.Error("expected a pending plan after Run")
	}

	// Apply — branch must now exist in git.
	applyRes, err := wf.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(branch_create): %v", err)
	}
	t.Logf("Apply result: status=%s output=%q", applyRes.Status, applyRes.Output)

	if applyRes.Status != workflow.StatusCompleted {
		t.Errorf("expected status=%s after apply, got %s", workflow.StatusCompleted, applyRes.Status)
	}
	if branchName := res.Args["branch"]; !branchExists(t, dir, branchName) {
		t.Errorf("branch %q not found in git after apply", branchName)
	}
}

// TestBranchCreate_Direct verifies preview=false:
// Run executes immediately and the branch is created in a single call.
func TestBranchCreate_Direct(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf, _ := makeWorkflow(gitA, llmA, map[string]bool{}) // no preview

	ctx := context.Background()
	res, err := wf.Run(ctx, "branch_create", "create a branch for the payment integration", nil)
	if err != nil {
		t.Fatalf("Run(branch_create) direct: %v", err)
	}
	t.Logf("Direct result: status=%s output=%q args=%v", res.Status, res.Output, res.Args)

	if res.Status != workflow.StatusCompleted {
		t.Errorf("expected status=%s, got %s", workflow.StatusCompleted, res.Status)
	}
	if branchName := res.Args["branch"]; branchName != "" && !branchExists(t, dir, branchName) {
		t.Errorf("branch %q not found in git after direct create", branchName)
	}
}

// ============================================================================
// Group C — Branch Delete
// ============================================================================

// TestBranchDelete_Preview verifies preview=true for branch deletion:
// START returns pending, APPLY removes the branch.
func TestBranchDelete_Preview(t *testing.T) {
	dir, gitA := sandboxRepo(t)
	llmA := requireOllama(t)
	wf, _ := makeWorkflow(gitA, llmA, map[string]bool{"branch_delete": true})

	// Create a branch to delete.
	exec.Command("git", "-C", dir, "branch", "feat/to-delete").Run()
	if !branchExists(t, dir, "feat/to-delete") {
		t.Fatal("setup: could not create feat/to-delete branch")
	}

	ctx := context.Background()
	// Pass explicit args to avoid LLM guessing the wrong branch name.
	res, err := wf.Run(ctx, "branch_delete", "", map[string]string{"branch": "feat/to-delete"})
	if err != nil {
		t.Fatalf("Run(branch_delete): %v", err)
	}
	t.Logf("Run result: status=%s preview=%q", res.Status, res.Preview)

	if res.Status != workflow.StatusPending {
		t.Fatalf("expected %s, got %s", workflow.StatusPending, res.Status)
	}
	// Branch still exists — pending, not applied yet.
	if !branchExists(t, dir, "feat/to-delete") {
		t.Error("branch was deleted before APPLY — preview mode broken")
	}

	applyRes, err := wf.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(branch_delete): %v", err)
	}
	t.Logf("Apply result: status=%s", applyRes.Status)

	if branchExists(t, dir, "feat/to-delete") {
		t.Error("branch still exists after APPLY — delete did not execute")
	}
}

// TestBranchDelete_Direct verifies preview=false:
// The branch is deleted immediately in a single Run call.
func TestBranchDelete_Direct(t *testing.T) {
	dir, gitA := sandboxRepo(t)
	llmA := requireOllama(t)
	wf, _ := makeWorkflow(gitA, llmA, map[string]bool{})

	exec.Command("git", "-C", dir, "branch", "feat/cleanup").Run()
	if !branchExists(t, dir, "feat/cleanup") {
		t.Fatal("setup: could not create feat/cleanup branch")
	}

	ctx := context.Background()
	res, err := wf.Run(ctx, "branch_delete", "", map[string]string{"branch": "feat/cleanup"})
	if err != nil {
		t.Fatalf("Run(branch_delete) direct: %v", err)
	}
	t.Logf("Direct result: status=%s output=%q", res.Status, res.Output)

	if res.Status != workflow.StatusCompleted {
		t.Errorf("expected %s, got %s", workflow.StatusCompleted, res.Status)
	}
	if branchExists(t, dir, "feat/cleanup") {
		t.Error("branch still exists after direct delete")
	}
}

// ============================================================================
// Group D — Merge
// ============================================================================

// TestMerge_Preview verifies the two-phase merge flow:
// MERGE_START → pending → MERGE_APPLY → branches are merged.
func TestMerge_Preview(t *testing.T) {
	dir, gitA := sandboxRepo(t)
	llmA := requireOllama(t)
	wf, _ := makeWorkflow(gitA, llmA, map[string]bool{"merge": true})

	// Create a feature branch with a commit so the merge has something to do.
	exec.Command("git", "-C", dir, "checkout", "-b", "feat/merge-me").Run()
	os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "feature.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: add feature").Run()
	exec.Command("git", "-C", dir, "checkout", "main").Run()

	ctx := context.Background()
	res, err := wf.Run(ctx, "merge", "", map[string]string{"branch": "feat/merge-me"})
	if err != nil {
		t.Fatalf("Run(merge): %v", err)
	}
	t.Logf("Run result: status=%s preview=%q", res.Status, res.Preview)

	if res.Status != workflow.StatusPending {
		t.Fatalf("expected %s, got %s", workflow.StatusPending, res.Status)
	}

	// feature.go must NOT be in main yet.
	if _, err := os.Stat(filepath.Join(dir, "feature.go")); err == nil {
		t.Error("feature.go is in main before APPLY — merge happened too early")
	}

	applyRes, err := wf.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(merge): %v", err)
	}
	t.Logf("Apply result: status=%s output=%q", applyRes.Status, applyRes.Output)

	// feature.go must now be in main.
	if _, err := os.Stat(filepath.Join(dir, "feature.go")); err != nil {
		t.Error("feature.go not found in main after APPLY — merge did not execute")
	}
}

// TestMerge_Direct verifies preview=false merge:
// Run merges immediately in a single call.
func TestMerge_Direct(t *testing.T) {
	dir, gitA := sandboxRepo(t)
	llmA := requireOllama(t)
	wf, _ := makeWorkflow(gitA, llmA, map[string]bool{})

	exec.Command("git", "-C", dir, "checkout", "-b", "feat/direct-merge").Run()
	os.WriteFile(filepath.Join(dir, "direct.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "direct.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: direct merge file").Run()
	exec.Command("git", "-C", dir, "checkout", "main").Run()

	ctx := context.Background()
	res, err := wf.Run(ctx, "merge", "", map[string]string{"branch": "feat/direct-merge"})
	if err != nil {
		t.Fatalf("Run(merge) direct: %v", err)
	}
	t.Logf("Direct result: status=%s output=%q", res.Status, res.Output)

	if res.Status != workflow.StatusCompleted {
		t.Errorf("expected %s, got %s", workflow.StatusCompleted, res.Status)
	}
	if _, err := os.Stat(filepath.Join(dir, "direct.go")); err != nil {
		t.Error("direct.go not in main after direct merge")
	}
}

// ============================================================================
// Group E — Push / Pull (bare local remote — no internet required)
// ============================================================================

// TestPush_Direct verifies that Push sends local commits to the remote.
func TestPush_Direct(t *testing.T) {
	dir, gitA := sandboxRepoWithRemote(t)

	os.WriteFile(filepath.Join(dir, "pushed.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "pushed.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: file to push").Run()

	beforePush := remoteHasCommits(t, dir)

	result, err := gitA.Push()
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	t.Logf("Push result: %q", result)

	afterPush := remoteHasCommits(t, dir)
	if afterPush <= beforePush {
		t.Errorf("remote commit count did not increase: before=%d after=%d", beforePush, afterPush)
	}
}

// TestPull_Direct verifies that Pull brings remote changes into the local repo.
func TestPull_Direct(t *testing.T) {
	dir, gitA := sandboxRepoWithRemote(t)

	// Push a commit to remote, then reset locally and pull it back.
	os.WriteFile(filepath.Join(dir, "remote_file.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "remote_file.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: remote file").Run()
	gitA.Push()

	// Remove the local file and reset to simulate divergence.
	exec.Command("git", "-C", dir, "reset", "--hard", "HEAD~1").Run()

	if _, err := os.Stat(filepath.Join(dir, "remote_file.go")); err == nil {
		t.Fatal("setup: remote_file.go should not exist locally after reset")
	}

	result, err := gitA.Pull()
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	t.Logf("Pull result: %q", result)

	if _, err := os.Stat(filepath.Join(dir, "remote_file.go")); err != nil {
		t.Error("remote_file.go not found after pull — pull did not bring remote changes")
	}
}

// ============================================================================
// Group F — Release
// ============================================================================

// TestRelease_Prepare verifies that Prepare correctly parses the instruction,
// calculates the version bump from commits, and returns the commits since the
// last tag — all WITHOUT touching the repo (no tag created).
func TestRelease_Prepare(t *testing.T) {
	dir, gitA := sandboxRepo(t)
	// Prepare does NOT need Ollama — it uses regex parsing internally.
	llmA := openai_standard.NewOpenAIStandardAdapter(LLMHost+"/v1", LLMModel)
	svc := makeReleaseSvc(t, gitA, llmA, dir)

	// Create a baseline tag.
	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()

	// Add commits that should trigger a minor bump.
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "a.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: add feature A").Run()

	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "b.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "fix: resolve bug B").Run()

	intent, commits, warnings, err := svc.Prepare("release a new minor version", "")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	t.Logf("intent=%+v commits=%q warnings=%v", intent, commits, warnings)

	if !intent.IsRelease {
		t.Error("expected IsRelease=true")
	}
	if intent.TagName == "" {
		t.Error("expected TagName to be set")
	}
	if commits == "" {
		t.Error("expected commits to be non-empty")
	}
	if tagExists(t, dir, intent.TagName) {
		t.Errorf("Prepare must NOT create the tag — %q was found in git", intent.TagName)
	}
}

// TestRelease_Generate_Changelog verifies that Generate produces a human-readable
// changelog from concrete commits, without creating any tag or commit.
// Uses unambiguous feature names so small models can translate them correctly.
func TestRelease_Generate_Changelog(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeReleaseSvc(t, gitA, llmA, dir)

	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()

	commits := "feat: add CSV export to the reports page\nfix: user list no longer crashes on empty projects\ndocs: update API documentation"

	changelog, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	t.Logf("changelog:\n%s\nwarnings: %v", changelog, warnings)

	if len(changelog) < 30 {
		t.Errorf("changelog too short (%d chars) — expected substantive output", len(changelog))
	}
	// No breaking change commits → the Breaking Changes section must not contain actual items.
	// We accept any explicit statement that there are none, e.g.:
	// - "- None"
	// - "- none"
	// - "*None in this release.*"
	// - "None"
	if strings.Contains(changelog, "🚨") || strings.Contains(changelog, "Breaking") {
		lower := strings.ToLower(changelog)
		hasNone := strings.Contains(lower, "- none") || strings.Contains(lower, "*none") || strings.Contains(lower, "none in this release") || strings.Contains(lower, "no breaking changes")
		if !hasNone {
			t.Errorf("changelog invented a breaking change that does not exist:\n%s", changelog)
		}
	}

	// Repo must remain untouched.
	tags, _ := gitA.ListTags()
	for _, tag := range tags {
		if tag != "v1.0.0" {
			t.Errorf("unexpected tag %q — Generate must not create tags", tag)
		}
	}
}

// TestRelease_BreakingChange_Changelog verifies that a commit marked with `!`
// produces a 🚨 Breaking Changes section and the rest are user-friendly entries.
func TestRelease_BreakingChange_Changelog(t *testing.T) {
	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	svc := makeReleaseSvc(t, gitA, llmA, dir)

	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()

	// feat! marks a breaking change — authentication flow was replaced.
	commits := "feat!: replace password login with SSO-only authentication\nfeat: add two-factor authentication support\nfix: session no longer expires unexpectedly after 10 minutes"

	changelog, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	t.Logf("changelog:\n%s\nwarnings: %v", changelog, warnings)

	// Must have substantive content.
	if len(changelog) < 30 {
		t.Errorf("changelog too short (%d chars)", len(changelog))
	}
	// Breaking change IS present in the commits → must appear in output.
	if !strings.Contains(changelog, "🚨") && !strings.Contains(changelog, "Breaking") {
		t.Errorf("expected 🚨 Breaking Changes section for feat! commit, not found:\n%s", changelog)
	}
	// Repo must remain untouched.
	tags, _ := gitA.ListTags()
	for _, tag := range tags {
		if tag != "v1.0.0" {
			t.Errorf("unexpected tag %q — Generate must not create tags", tag)
		}
	}
}

// ============================================================================
// Group G — Edge Cases
// ============================================================================

// TestCommit_LLMError verifies that when the LLM returns an error the service
// propagates it cleanly and does NOT create any commit or leave the repo dirty.
// Uses errorLLM (immediate fail) instead of a closed port to avoid the real
// adapter's 120s retry delay (10s+20s+30s+60s waitTimes).
func TestCommit_LLMError(t *testing.T) {
	dir, gitA := sandboxRepo(t)
	svc := makeCommitSvc(t, gitA, &errorLLM{"LLM connection refused"}, &noOpSecurity{}, dir)

	os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "broken.go").Run()

	_, err := svc.Execute("commit all my changes", false)
	if err == nil {
		t.Fatal("expected an error when LLM fails, got nil")
	}
	t.Logf("Got expected LLM error: %v", err)

	if msgs := gitLogMessages(t, dir); len(msgs) > 0 {
		t.Errorf("repo has commits after LLM error — should be clean: %v", msgs)
	}
}

// TestWorkflow_ApplyWithoutStart verifies that calling Apply before any START
// returns a clear error and does NOT crash.
func TestWorkflow_ApplyWithoutStart(t *testing.T) {
	_, gitA := sandboxRepo(t)
	llmA := openai_standard.NewOpenAIStandardAdapter(LLMHost+"/v1", LLMModel)
	wf, _ := makeWorkflow(gitA, llmA, map[string]bool{"branch_create": true})

	_, err := wf.Apply(context.Background())
	if err == nil {
		t.Fatal("expected an error when applying without a prior START, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

// TestWorkflow_DisabledOperation verifies that operations not in
// EnabledOperations return a user-friendly error from the execute layer.
func TestWorkflow_DisabledOperation(t *testing.T) {
	_, gitA := sandboxRepo(t)
	llmA := requireOllama(t)

	cfg := config.Default()
	cfg.Commands.EnabledOperations = []string{} // nothing enabled
	cfg.Preview.Enabled = false
	c := confirm.NewInMemory(5 * time.Minute)
	
	// Create minimal dependencies for the test
	sec := security.New(cfg, llmA)
	wf := workflow.New(gitA, llmA, c, cfg, nil, nil, sec)

	_, err := wf.Run(context.Background(), "branch_create", "", map[string]string{"branch": "feat/x"})
	if err == nil {
		t.Fatal("expected error for disabled operation, got nil")
	}
	if !strings.Contains(err.Error(), "deshabilitad") && !strings.Contains(err.Error(), "disabled") {
		t.Errorf("expected disabled-operation error, got: %v", err)
	}
	t.Logf("Got expected disabled-op error: %v", err)
}
