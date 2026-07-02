//go:build e2e

package release

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/adapters/commitstore"
	gitadapter "github.com/blak0p/git-courer/internal/adapters/git"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/shared/testutil"
	"github.com/blak0p/git-courer/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCmd is a test helper that runs a command in a directory.
func runCmd(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cmd %s %v failed in %s:\n%s\n%v", name, args, dir, string(out), err)
	}
	return strings.TrimSpace(string(out))
}

// commitInfo holds a parsed commit SHA and message.
type commitInfo struct {
	sha     string
	message string
}

// setupTestRepo creates a temp git repo with an initial tag and some commits,
// populates a branch-scoped CommitStore, and writes a dedicated config.json
// with areas so the area-based changelog path is exercised.
func setupTestRepo(t *testing.T) (repoDir string, store *commitstore.FilesystemCommitStore) {
	t.Helper()
	dir := t.TempDir()

	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "e2e@test.com")
	runCmd(t, dir, "git", "config", "user.name", "E2E Test")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test repo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runCmd(t, dir, "git", "add", ".")
	runCmd(t, dir, "git", "commit", "-m", "chore: initial commit")
	runCmd(t, dir, "git", "tag", "v1.0.0")

	commits := []struct {
		file    string
		content string
		msg     string
	}{
		{"main.go", "package main\n\nfunc main() {}\n", "feat(core): add main function"},
		{"utils.go", "package main\n\nfunc util() {}\n", "feat(core): add utility function"},
		{"fix.go", "package main\n\nfunc fix() {}\n", "fix(core): resolve edge case in parser"},
	}
	for _, c := range commits {
		if err := os.WriteFile(filepath.Join(dir, c.file), []byte(c.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", c.file, err)
		}
		runCmd(t, dir, "git", "add", ".")
		runCmd(t, dir, "git", "commit", "-m", c.msg)
	}

	raw := runCmd(t, dir, "git", "log", "--oneline", "--format=%H||%s", "v1.0.0..HEAD")
	var entries []commitInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "||", 2)
		if len(parts) == 2 {
			entries = append(entries, commitInfo{sha: parts[0], message: parts[1]})
		}
	}

	store = commitstore.NewFilesystemCommitStore(dir, nil)
	if err := store.SetWorkspace("e2e/test"); err != nil {
		t.Fatalf("SetWorkspace: %v", err)
	}
	for _, e := range entries {
		entry, err := domain.NewCommitEntry(e.sha, e.message)
		if err != nil {
			t.Fatalf("NewCommitEntry(%q, %q): %v", e.sha, e.message, err)
		}
		if err := store.Append(entry); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	// Write dedicated config.json with areas matching commit scopes
	cfgDir := filepath.Join(dir, ".git-courer")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir .git-courer: %v", err)
	}
	projectCfg := domain.ProjectConfig{
		Description: "E2E release test",
		Areas: map[string][]string{
			"core": {""},
		},
	}
	cfgData, _ := json.MarshalIndent(projectCfg, "", "  ")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), cfgData, 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	return dir, store
}

// writeAuditFile writes data to the audit directory.
func writeAuditFile(t *testing.T, auditDir, name string, data []byte) {
	t.Helper()
	path := filepath.Join(auditDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Logf("warning: could not create audit dir: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Logf("warning: could not write audit file %s: %v", path, err)
	}
}

func TestE2ERelease(t *testing.T) {
	// Resolve audit dir BEFORE Chdir so it stays relative to the project root
	auditDir := filepath.Join("audit", fmt.Sprintf("release_%d", time.Now().Unix()))
	absAudit, _ := filepath.Abs(auditDir)
	os.RemoveAll(absAudit)
	os.MkdirAll(absAudit, 0o755)

	// Stage 0: Setup — create real git repo + CommitStore + config.json with areas
	stage0Start := time.Now()
	repoDir, commitStore := setupTestRepo(t)

	// Chdir so LoadProjectConfig(".") reads from the test repo's config.json
	t.Chdir(repoDir)

	stage0Dur := time.Since(stage0Start)
	t.Logf("stage 00 (setup): repo=%s latency=%s", repoDir, stage0Dur)

	// Dump CommitStore raw content + config to audit
	branchStore := filepath.Join(repoDir, ".git-courer", "branches", "e2e-test", "commits.json")
	if storeData, err := os.ReadFile(branchStore); err == nil {
		writeAuditFile(t, absAudit, "00_commitstore.jsonl", storeData)
	}
	if cfgData, err := os.ReadFile(filepath.Join(repoDir, ".git-courer", "config.json")); err == nil {
		writeAuditFile(t, absAudit, "00_config.json", cfgData)
	}

	gitAdapter := gitadapter.New(repoDir)
	llm := testutil.RequireLLM(t)

	cfg := workflow.ReleaseServiceConfig{
		ContextWindow:      4096,
		MaxCommitsPerChunk: 20,
		LogPath:            filepath.Join(repoDir, ".git-courer", "release.log"),
		MaxLogLines:        500,
		NumParallel:        1,
		WorkDir:            repoDir,
	}

	svc := workflow.NewReleaseService(gitAdapter, llm, nil, cfg, nil, commitStore)

	// ---- Stage 1: Prepare ----
	t.Run("Prepare", func(t *testing.T) {
		stage1Start := time.Now()
		intent, commits, warnings, err := svc.Prepare("sacar versión minor", "")
		stage1Dur := time.Since(stage1Start)

		input := []byte("sacar versión minor")
		output := []byte(fmt.Sprintf("TagName=%s VersionBump=%s IsRelease=%v\n\nCommits (%d chars):\n%s",
			intent.TagName, intent.VersionBump, intent.IsRelease, len(commits), commits))
		writeAuditFile(t, absAudit, "01_prepare.txt", output)

		intentJSON, _ := json.MarshalIndent(intent, "", "  ")
		writeAuditFile(t, absAudit, "01_intent.json", intentJSON)

		require.NoError(t, err, "Prepare should succeed")
		require.NotNil(t, intent, "intent should not be nil")
		assert.True(t, intent.IsRelease, "IsRelease should be true")
		assert.Equal(t, "minor", intent.VersionBump, "VersionBump should be minor")
		assert.Contains(t, intent.TagName, "v1.1", "TagName should be v1.1.x")
		assert.NotEmpty(t, commits, "commits should not be empty")
		assert.Contains(t, commits, "feat(core): add main function", "commits should contain our feat commits")
		assert.Contains(t, commits, "fix(core): resolve edge case in parser", "commits should contain our fix commit")

		t.Logf("  Intent: TagName=%s VersionBump=%s IsRelease=%v", intent.TagName, intent.VersionBump, intent.IsRelease)
		t.Logf("  Commits (%d chars): %s", len(commits), strings.ReplaceAll(commits, "\n", " | "))
		for _, w := range warnings {
			t.Logf("  Warning: %s", w)
		}
		t.Logf("stage 01 (prepare):           in=%dB  out=%dB  latency=%s", len(input), len(output), stage1Dur)
		t.Logf("  CommitStore: %s (%d bytes)", branchStore, len(intentJSON))

		// Stage 2: Generate (LLM) — area-based changelog
		t.Run("Generate", func(t *testing.T) {
			stage2Start := time.Now()
			changelog, lines, background, err := svc.Generate(commits)
			stage2Dur := time.Since(stage2Start)

			writeAuditFile(t, absAudit, "02_changelog.md", []byte(changelog))

			require.NoError(t, err, "Generate should succeed")
			assert.NotEmpty(t, changelog, "changelog should not be empty")
			t.Logf("  Changelog (%d chars):\n%s", len(changelog), changelog)
			t.Logf("  Changelog lines: %d", len(lines))
			t.Logf("  Is background: %v", background)
			t.Logf("stage 02 (generate):         in=%dB  out=%dB  latency=%s", len([]byte(commits)), len([]byte(changelog)), stage2Dur)

			// Stage 3: Persistence — Save/Load round-trip
			t.Run("Persistence", func(t *testing.T) {
				stage3Start := time.Now()
				svc.SaveIntent(intent)
				svc.SaveChangelog(changelog)

				intentFile := filepath.Join(repoDir, ".git-courer", "release_intent.json")
				changelogFile := filepath.Join(repoDir, ".git-courer", "release_changelog.md")
				if d, err := os.ReadFile(intentFile); err == nil {
					writeAuditFile(t, absAudit, "03_intent_persisted.json", d)
				}
				if d, err := os.ReadFile(changelogFile); err == nil {
					writeAuditFile(t, absAudit, "03_changelog_persisted.md", d)
				}

				svc2 := workflow.NewReleaseService(gitAdapter, llm, nil, cfg, nil, commitStore)
				loadedIntent, err := svc2.LoadIntent()
				require.NoError(t, err, "LoadIntent should succeed after SaveIntent")
				assert.Equal(t, intent.TagName, loadedIntent.TagName, "loaded intent TagName should match")
				assert.Equal(t, intent.VersionBump, loadedIntent.VersionBump, "loaded intent VersionBump should match")

				loadedChangelog, err := svc2.LoadChangelog()
				require.NoError(t, err, "LoadChangelog should succeed after SaveChangelog")
				assert.Equal(t, changelog, loadedChangelog, "loaded changelog should match")
				stage3Dur := time.Since(stage3Start)
				t.Logf("stage 03 (persistence):      in=%dB  out=%dB  latency=%s", len([]byte(changelog)), len([]byte(loadedChangelog)), stage3Dur)

				// Stage 4: ClearPending
				t.Run("Clear", func(t *testing.T) {
					stage4Start := time.Now()
					svc2.ClearPending()

					svc3 := workflow.NewReleaseService(gitAdapter, llm, nil, cfg, nil, commitStore)
					_, err := svc3.LoadIntent()
					assert.Error(t, err, "LoadIntent should error after ClearPending")

					clearedChangelog, _ := svc3.LoadChangelog()
					assert.Empty(t, clearedChangelog, "changelog should be empty after ClearPending")
					stage4Dur := time.Since(stage4Start)
					t.Logf("stage 04 (clear):            latency=%s", stage4Dur)
				})
			})
		})
	})

	writeAuditREADME(t, absAudit)
}

func writeAuditREADME(t *testing.T, auditDir string) {
	t.Helper()
	content := `# Release E2E Audit

Output of the release e2e test with area-based changelog generation (via dedicated config.json).

## Files

| File | Format | Description |
|------|--------|-------------|
| 00_config.json | JSON | Project config with area mappings (triggers area-based path) |
| 00_commitstore.jsonl | JSONL | Raw CommitStore entries (input to release) |
| 01_prepare.txt | Plain text | Release intent summary + commits fed to LLM |
| 01_intent.json | JSON | Parsed ReleaseIntent from Prepare |
| 02_changelog.md | Markdown | LLM-generated changelog (area-based) |
| 03_intent_persisted.json | JSON | ReleaseIntent re-read from .git-courer/ |
| 03_changelog_persisted.md | Markdown | Changelog re-read from .git-courer/ |

## Data Flow

CommitStore → Prepare → LLM Generate (by area) → SaveIntent/SaveChangelog → LoadIntent/LoadChangelog → ClearPending
`
	writeAuditFile(t, auditDir, "README.md", []byte(content))
}
