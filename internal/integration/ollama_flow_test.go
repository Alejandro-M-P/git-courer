//go:build integration

package integration

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

var testWorkDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "git-courer-integration-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	testWorkDir = dir

	code := m.Run()

	if collector := testutil.GetTelemetryCollector(); collector != nil {
		if c, ok := collector.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}

	os.RemoveAll(dir)
	os.Exit(code)
}

func skipIfNoOllama(t *testing.T) {
	testutil.RequireOllama(t)
}

// --- Commit Flow Integration Tests ---

// initGitRepo initializes a git repo with user config and an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@git-courer.test")
	run("config", "user.name", "Git Courer Test")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".gitkeep")
	run("commit", "-m", "chore: initial commit")
	out, _ := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if branch := strings.TrimSpace(string(out)); branch != "main" && branch != "" {
		run("branch", "-m", branch, "main")
	}
}

func TestCommitService_PrepareCommit_FullFlow(t *testing.T) {
	skipIfNoOllama(t)

	dir := filepath.Join(testWorkDir, "commit-test")
	os.MkdirAll(dir, 0755)
	initGitRepo(t, dir)

	gitAdapter := git.New(dir)
	llmAdapter := testutil.RequireOllama(t)
	chunker := chunkers.NewDiffChunker()
	cfg := config.Default()
	securitySvc := security.New(cfg, llmAdapter)

	commitCfg := workflow.DefaultCommitServiceConfig(
		4096,
		50,
		filepath.Join(testWorkDir, "commit.log"),
	)
	svc := workflow.NewCommitService(gitAdapter, llmAdapter, chunker, securitySvc, commitCfg)

	// Test with REAL project files instead of fake files
	// This uses actual code from the project to get realistic commit messages
	realFiles := map[string]string{
		"test.go": `package test

func TestAdd() bool {
	return true
}`,
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
		"config.go": `package config

var Version = "1.0.0"

func Load() error {
	return nil
}`,
		"service.go": `package service

type Service struct {
	Name string
}

func New() *Service {
	return &Service{Name: "default"}
}`,
	}

	for name, content := range realFiles {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	}
	// Pre-stage so the file appears as tracked (not untracked) — qwen is non-deterministic
	// about whether to include untracked files; tracked files are reliably included.
	// Add test.go to make it tracked
	cmd := exec.Command("git", "-C", dir, "add", "test.go")
	if err := cmd.Run(); err != nil {
		t.Logf("git add test.go err: %v", err)
	}
	// Add all files to ensure something is tracked
	cmd = exec.Command("git", "-C", dir, "add", ".")
	if err := cmd.Run(); err != nil {
		t.Logf("git add . err: %v", err)
	}

	log.Println("=== TestCommitService_PrepareCommit_FullFlow ===")
	log.Printf("WorkDir: %s", dir)

	messages, chunks, deleted, _, _, err := svc.PrepareCommit("commit all files")
	if err != nil {
		t.Fatalf("PrepareCommit() error: %v", err)
	}

	log.Println("=== PREVIEW (no commit executed) ===")
	log.Printf("Messages generated: %d", len(messages))
	for i, msg := range messages {
		log.Printf("  [%d] %s", i+1, msg)
	}
	log.Printf("Chunks: %d", len(chunks))
	log.Printf("Deleted files: %d", len(deleted))
	log.Println("=== END PREVIEW ===")

	if len(messages) == 0 {
		t.Error("Expected at least one message, got none")
	}
}

func TestCommitService_Execute_DryRun(t *testing.T) {
	skipIfNoOllama(t)

	dir := filepath.Join(testWorkDir, "commit-dryrun")
	os.MkdirAll(dir, 0755)
	initGitRepo(t, dir)

	gitAdapter := git.New(dir)
	llmAdapter := testutil.RequireOllama(t)
	chunker := chunkers.NewDiffChunker()
	cfg := config.Default()
	securitySvc := security.New(cfg, llmAdapter)

	commitCfg := workflow.DefaultCommitServiceConfig(
		4096,
		50,
		filepath.Join(testWorkDir, "commit-dryrun.log"),
	)
	svc := workflow.NewCommitService(gitAdapter, llmAdapter, chunker, securitySvc, commitCfg)

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	exec.Command("git", "-C", dir, "add", "main.go").Run()

	log.Println("=== TestCommitService_Execute_DryRun ===")

	result, err := svc.Execute("add main file", false)
	if err != nil {
		log.Printf("Execute result (expected to fail if no changes to commit): %v", err)
	}

	log.Printf("Result: %s", result)
	log.Println("=== NO COMMIT EXECUTED ===")

	status, _ := gitAdapter.Status()
	if !status.IsClean {
		t.Error("Repo should be clean (no commit executed)")
	}
}

// --- Release Flow Integration Tests ---

func TestReleaseService_Prepare_FullFlow(t *testing.T) {
	skipIfNoOllama(t)

	dir := filepath.Join(testWorkDir, "release-test")
	os.MkdirAll(dir, 0755)
	initGitRepo(t, dir)

	gitAdapter := git.New(dir)
	llmAdapter := testutil.RequireOllama(t)

	gitAdapter.Tag("v1.0.0", "")

	os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\nfunc newFeature() {}\n"), 0644)
	gitAdapter.Add([]string{"."})
	gitAdapter.Commit("feat: add new feature")

	logChunker := chunkers.NewLogChunker(4096)
	releaseCfg := workflow.DefaultReleaseServiceConfig(
		4096,
		20,
		50,
		filepath.Join(testWorkDir, "release.log"),
	)
	svc := workflow.NewReleaseService(gitAdapter, llmAdapter, logChunker, releaseCfg, nil)

	log.Println("=== TestReleaseService_Prepare_FullFlow ===")

	intent, commits, warnings, err := svc.Prepare("sacar versión minor", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	log.Println("=== PREVIEW (no tag created) ===")
	log.Printf("Tag name: %s", intent.TagName)
	log.Printf("Version bump: %s", intent.VersionBump)
	log.Printf("Is release: %v", intent.IsRelease)
	log.Printf("Commits found: %d", len(strings.Split(commits, "\n")))
	log.Printf("Warnings: %v", warnings)
	log.Println("=== END PREVIEW ===")

	if !intent.IsRelease {
		t.Error("Expected IsRelease=true")
	}
	if intent.TagName == "" {
		t.Error("Expected TagName to be set")
	}
}

func TestReleaseService_Generate_Changelog(t *testing.T) {
	skipIfNoOllama(t)

	dir := filepath.Join(testWorkDir, "release-generate")
	os.MkdirAll(dir, 0755)
	initGitRepo(t, dir)

	gitAdapter := git.New(dir)
	llmAdapter := testutil.RequireOllama(t)

	gitAdapter.Tag("v1.0.0", "")

	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644)
	gitAdapter.Add([]string{"."})
	gitAdapter.Commit("feat: add feature A")

	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644)
	gitAdapter.Add([]string{"."})
	gitAdapter.Commit("fix: resolve bug B")

	logChunker := chunkers.NewLogChunker(4096)
	releaseCfg := workflow.DefaultReleaseServiceConfig(
		4096,
		20,
		50,
		filepath.Join(testWorkDir, "release-generate.log"),
	)
	svc := workflow.NewReleaseService(gitAdapter, llmAdapter, logChunker, releaseCfg, nil)

	commits := `feat: add feature A
fix: resolve bug B`

	log.Println("=== TestReleaseService_Generate_Changelog ===")

	changelog, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	log.Println("=== CHANGELOG PREVIEW (no tag created) ===")
	log.Printf("%s", changelog)
	log.Println("=== END CHANGELOG ===")

	if len(changelog) < 20 {
		t.Errorf("Expected changelog > 20 chars, got %d", len(changelog))
	}
}

func TestReleaseService_PrepareAndGenerate_EndToEnd(t *testing.T) {
	skipIfNoOllama(t)

	dir := filepath.Join(testWorkDir, "release-e2e")
	os.MkdirAll(dir, 0755)
	initGitRepo(t, dir)

	gitAdapter := git.New(dir)
	llmAdapter := testutil.RequireOllama(t)

	gitAdapter.Tag("v1.0.0", "")

	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\nfunc new() {}\n"), 0644)
	gitAdapter.Add([]string{"."})
	gitAdapter.Commit("feat: awesome new feature")
	gitAdapter.Commit("docs: update readme")

	logChunker := chunkers.NewLogChunker(4096)
	releaseCfg := workflow.DefaultReleaseServiceConfig(
		4096,
		20,
		50,
		filepath.Join(testWorkDir, "release-e2e.log"),
	)
	svc := workflow.NewReleaseService(gitAdapter, llmAdapter, logChunker, releaseCfg, nil)

	log.Println("=== TestReleaseService_PrepareAndGenerate_EndToEnd ===")

	intent, commits, _, err := svc.Prepare("sacar versión", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	changelog, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	log.Println("=== FULL RELEASE PREVIEW (no tag created) ===")
	log.Printf("Tag: %s (%s)", intent.TagName, intent.VersionBump)
	log.Printf("Commits:\n%s", commits)
	log.Printf("\nChangelog:\n%s", changelog)
	log.Println("=== END PREVIEW ===")

	tags, _ := gitAdapter.ListTags()
	log.Printf("Tags in repo (should only have v1.0.0): %v", tags)

	if len(tags) > 1 || (len(tags) == 1 && tags[0] != "v1.0.0") {
		t.Error("New tag was created! Test should not modify repo")
	}
}

// --- Branch Flow Integration Tests ---

func TestWorkflow_BranchCreate_Interpret(t *testing.T) {
	skipIfNoOllama(t)

	dir := filepath.Join(testWorkDir, "branch-test")
	os.MkdirAll(dir, 0755)
	initGitRepo(t, dir)

	gitAdapter := git.New(dir)
	llmAdapter := testutil.RequireOllama(t)

	log.Println("=== TestWorkflow_BranchCreate_Interpret ===")

	result, err := llmAdapter.InterpretGitOp(
		"branch",
		"create a new branch called feat/login",
		map[string]string{"current_branch": "main"},
	)
	if err != nil {
		t.Fatalf("InterpretGitOp() error: %v", err)
	}

	log.Println("=== BRANCH PLAN (no branch created) ===")
	log.Printf("Result: %v", result)
	log.Println("=== END PLAN ===")

	if result["error"] != "" {
		t.Errorf("Expected no error, got: %s", result["error"])
	}

	branches, _ := gitAdapter.ListBranches()
	log.Printf("Branches in repo (should only have main): %s", branches)

	if !strings.Contains(branches, "main") || strings.Contains(branches, "feat/login") {
		t.Error("Branch was created! Test should not modify repo")
	}
}
