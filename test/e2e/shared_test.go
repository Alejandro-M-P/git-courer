//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

// ─── Result collector ─────────────────────────────────────────────────────────

type testResult struct {
	name     string
	passed   bool
	duration time.Duration
	detail   string
}

var (
	resMu   sync.Mutex
	results []testResult
)

func recordResult(name string, passed bool, dur time.Duration, detail string) {
	resMu.Lock()
	results = append(results, testResult{name, passed, dur, detail})
	resMu.Unlock()
}

func TestMain(m *testing.M) {
	code := m.Run()
	if collector := testutil.GetTelemetryCollector(); collector != nil {
		if c, ok := collector.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}
	printSummary()
	os.Exit(code)
}

func printSummary() {
	const width = 72
	line := strings.Repeat("═", width-2)
	fmt.Printf("\n╔%s╗\n", line)
	fmt.Printf("║%s║\n", center("E2E TEST SUITE — SUMMARY", width-2))
	fmt.Printf("╠%s╣\n", line)

	passed, failed := 0, 0
	for _, r := range results {
		icon := "✓"
		if !r.passed {
			icon = "✗"
			failed++
		} else {
			passed++
		}
		name := r.name
		if len(name) > 44 {
			name = name[:41] + "..."
		}
		detail := r.detail
		if len(detail) > 12 {
			detail = detail[:12]
		}
		fmt.Printf("║ %s  %-44s  %5.1fs  %-12s ║\n",
			icon, name, r.duration.Seconds(), detail)
	}

	fmt.Printf("╠%s╣\n", line)
	summary := fmt.Sprintf("  PASSED: %d    FAILED: %d    TOTAL: %d", passed, failed, passed+failed)
	fmt.Printf("║%-*s║\n", width-2, summary)
	fmt.Printf("╚%s╝\n\n", line)
}

func center(s string, w int) string {
	if len(s) >= w {
		return s
	}
	pad := (w - len(s)) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", w-pad-len(s))
}

// ─── Ollama helper ────────────────────────────────────────────────────────────

func requireOllama(t *testing.T) ports.LLM {
	return testutil.RequireOllama(t)
}

// ─── Git sandbox ──────────────────────────────────────────────────────────────

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
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0644)
	run("add", "README.md")
	run("commit", "-m", "chore: initial commit")
	return dir, git.New(dir)
}

// ─── Service builders ─────────────────────────────────────────────────────────

func makeCommitSvc(t *testing.T, gitA ports.Git, llmA ports.LLM, sec ports.SecurityService, dir string) *workflow.CommitService {
	t.Helper()
	cfg := workflow.DefaultCommitServiceConfig(4096, 50, filepath.Join(dir, ".gcourer", "commit.log"))
	return workflow.NewCommitService(gitA, llmA, chunkers.NewDiffChunker(), sec, cfg)
}

// makeWorkflow returns a workflow with preview enabled (Start→Apply flow).
func makeWorkflow(t *testing.T, gitA ports.Git, llmA ports.LLM) (*workflow.Workflow, *confirm.InMemoryConfirm) {
	t.Helper()
	cfg := config.Default()
	c := confirm.NewInMemory(5 * time.Minute)
	sec := security.New(cfg, llmA)
	commitCfg := workflow.DefaultCommitServiceConfig(4096, 50, filepath.Join(t.TempDir(), "commit.log"))
	commit := workflow.NewCommitService(gitA, llmA, chunkers.NewDiffChunker(), sec, commitCfg)
	releaseCfg := workflow.DefaultReleaseServiceConfig(4096, 20, 50, filepath.Join(t.TempDir(), "release.log"))
	release := workflow.NewReleaseService(gitA, llmA, chunkers.NewLogChunker(4096), releaseCfg, nil)
	return workflow.New(gitA, llmA, c, cfg, commit, release, sec), c
}

// makeDirectWorkflow returns a workflow with preview disabled (executes immediately).
func makeDirectWorkflow(t *testing.T, gitA ports.Git, llmA ports.LLM) *workflow.Workflow {
	t.Helper()
	cfg := config.Default()
	cfg.Preview.Enabled = false
	c := confirm.NewInMemory(5 * time.Minute)
	sec := security.New(cfg, llmA)
	commitCfg := workflow.DefaultCommitServiceConfig(4096, 50, filepath.Join(t.TempDir(), "commit.log"))
	commit := workflow.NewCommitService(gitA, llmA, chunkers.NewDiffChunker(), sec, commitCfg)
	releaseCfg := workflow.DefaultReleaseServiceConfig(4096, 20, 50, filepath.Join(t.TempDir(), "release.log"))
	release := workflow.NewReleaseService(gitA, llmA, chunkers.NewLogChunker(4096), releaseCfg, nil)
	return workflow.New(gitA, llmA, c, cfg, commit, release, sec)
}

// ─── Security helpers ─────────────────────────────────────────────────────────

type noOpSecurity struct{}

func (n *noOpSecurity) CheckFiles(_ []string, _ string) *ports.SecurityCheckResult {
	return &ports.SecurityCheckResult{Blocked: false, Files: []ports.SecurityResult{}}
}
func (n *noOpSecurity) ShouldUseLLMScan() bool { return false }

// ─── Git state helpers ────────────────────────────────────────────────────────

func gitLog(dir string) []string {
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, _ := cmd.Output()
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func gitBranches(dir string) []string {
	cmd := exec.Command("git", "branch")
	cmd.Dir = dir
	out, _ := cmd.Output()
	var branches []string
	for _, b := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		b = strings.TrimPrefix(b, "* ")
		b = strings.TrimSpace(b)
		if b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

func gitCurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func gitTags(dir string) []string {
	cmd := exec.Command("git", "tag")
	cmd.Dir = dir
	out, _ := cmd.Output()
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func gitStatusShort(dir string) string {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// ─── File helpers ─────────────────────────────────────────────────────────────

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755); err != nil {
		t.Fatalf("mkdirAll %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

func stageAll(dir string) {
	exec.Command("git", "-C", dir, "add", ".").Run() //nolint:errcheck
}

func stageFile(dir, path string) {
	exec.Command("git", "-C", dir, "add", path).Run() //nolint:errcheck
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if strings.Contains(v, s) {
			return true
		}
	}
	return false
}
