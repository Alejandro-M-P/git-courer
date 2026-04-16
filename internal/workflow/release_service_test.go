package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func newReleaseSvc(t *testing.T, git *mockGitForRelease, llm *mockLLMForRelease) *ReleaseService {
	t.Helper()
	dir := t.TempDir()
	cfg := DefaultReleaseServiceConfig(
		4096, 20, 100,
		filepath.Join(dir, "release.log"),
		filepath.Join(dir, "CHANGELOG.md"),
		filepath.Join(dir, "intent.json"),
	)
	chunker := &mockLogChunker{}
	return NewReleaseService(git, llm, chunker, cfg)
}

// --- DefaultReleaseServiceConfig ---

func TestDefaultReleaseServiceConfig_ZeroContextWindow(t *testing.T) {
	cfg := DefaultReleaseServiceConfig(0, 0, 50, "/tmp/log", "/tmp/cl", "/tmp/intent")
	if cfg.ContextWindow != 4096 {
		t.Errorf("ContextWindow = %d, want 4096 (default)", cfg.ContextWindow)
	}
	if cfg.MaxCommitsPerChunk != 20 {
		t.Errorf("MaxCommitsPerChunk = %d, want 20 (default)", cfg.MaxCommitsPerChunk)
	}
}

func TestDefaultReleaseServiceConfig_Values(t *testing.T) {
	cfg := DefaultReleaseServiceConfig(8192, 30, 200, "/log", "/cl", "/intent")
	if cfg.ContextWindow != 8192 {
		t.Errorf("ContextWindow = %d, want 8192", cfg.ContextWindow)
	}
	if cfg.BackgroundThreshold != 3 {
		t.Errorf("BackgroundThreshold = %d, want 3", cfg.BackgroundThreshold)
	}
}

// --- Execute ---

func TestReleaseService_Execute_CreatesTag(t *testing.T) {
	git := &mockGitForRelease{tagExistsResult: false}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{TagName: "v1.0.0", VersionBump: "minor"}
	result, err := svc.Execute(intent, "## Changelog", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("Execute() returned empty result")
	}
	if !strings.Contains(result, "v1.0.0") {
		t.Errorf("result %q should contain tag name", result)
	}
}

func TestReleaseService_Execute_InvalidTagName(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{TagName: "not-semver"}
	_, err := svc.Execute(intent, "changelog", false)
	if err == nil {
		t.Error("Execute() should error on invalid tag name")
	}
	if !strings.Contains(err.Error(), "invalid tag name") {
		t.Errorf("error %q should mention 'invalid tag name'", err.Error())
	}
}

func TestReleaseService_Execute_TagAlreadyExists(t *testing.T) {
	git := &mockGitForRelease{tagExistsResult: true}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{TagName: "v1.0.0"}
	_, err := svc.Execute(intent, "changelog", false)
	if err == nil {
		t.Error("Execute() should error when tag already exists")
	}
	if !strings.Contains(err.Error(), "ya existe") {
		t.Errorf("error %q should mention tag already exists", err.Error())
	}
}

// --- Generate ---

func TestReleaseService_Generate_ReturnsChangelog(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	commits := "commit abc\nfeat: add feature\ncommit def\nfix: fix bug"
	changelog, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if changelog == "" {
		t.Error("Generate() returned empty changelog")
	}
	_ = warnings
}

func TestReleaseService_Generate_EmptyCommits_ChunkerError(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	dir := t.TempDir()
	cfg := DefaultReleaseServiceConfig(
		4096, 20, 100,
		filepath.Join(dir, "release.log"),
		filepath.Join(dir, "CHANGELOG.md"),
		filepath.Join(dir, "intent.json"),
	)
	// Use a chunker that errors
	errChunker := &mockLogChunker{err: fmt.Errorf("log input is empty")}
	svc := NewReleaseService(git, llm, errChunker, cfg)

	_, _, err := svc.Generate("anything")
	if err == nil {
		t.Error("Generate() should propagate chunker error")
	}
}

// --- SaveIntent / LoadIntent ---

func TestReleaseService_SaveLoadIntent(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{
		TagName:     "v2.0.0",
		VersionBump: "major",
		IsRelease:   true,
	}

	if err := svc.SaveIntent(intent); err != nil {
		t.Fatalf("SaveIntent() error: %v", err)
	}

	loaded, err := svc.LoadIntent()
	if err != nil {
		t.Fatalf("LoadIntent() error: %v", err)
	}
	if loaded.TagName != "v2.0.0" {
		t.Errorf("LoadIntent() TagName = %q, want v2.0.0", loaded.TagName)
	}
	if loaded.VersionBump != "major" {
		t.Errorf("LoadIntent() VersionBump = %q, want major", loaded.VersionBump)
	}
}

func TestReleaseService_LoadIntent_NonExistent(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	// Remove the intent file
	os.Remove(svc.cfg.IntentPath)

	_, err := svc.LoadIntent()
	if err == nil {
		t.Error("LoadIntent() should error when file does not exist")
	}
}

func TestReleaseService_SaveIntent_NoPath(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, "/tmp/log", "", "")
	// IntentPath is empty
	svc := NewReleaseService(git, llm, &mockLogChunker{}, cfg)

	err := svc.SaveIntent(&domain.ReleaseIntent{TagName: "v1.0.0"})
	if err != nil {
		t.Errorf("SaveIntent() with no path should return nil error, got: %v", err)
	}
}

// --- LoadChangelog ---

func TestReleaseService_LoadChangelog_NonExistent(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	os.Remove(svc.cfg.ChangelogPath)

	content, err := svc.LoadChangelog()
	if err != nil {
		t.Fatalf("LoadChangelog() error for non-existent file: %v", err)
	}
	if content != "" {
		t.Errorf("LoadChangelog() = %q, want empty for non-existent file", content)
	}
}

func TestReleaseService_LoadChangelog_Existing(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	os.MkdirAll(filepath.Dir(svc.cfg.ChangelogPath), 0755)
	os.WriteFile(svc.cfg.ChangelogPath, []byte("## v1.0.0\n- Feature"), 0644)

	content, err := svc.LoadChangelog()
	if err != nil {
		t.Fatalf("LoadChangelog() error: %v", err)
	}
	if !strings.Contains(content, "v1.0.0") {
		t.Errorf("LoadChangelog() = %q, should contain 'v1.0.0'", content)
	}
}

func TestReleaseService_LoadChangelog_NoPath(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, "/tmp/log", "", "")
	svc := NewReleaseService(git, llm, &mockLogChunker{}, cfg)

	content, err := svc.LoadChangelog()
	if err != nil {
		t.Fatalf("LoadChangelog() with no path error: %v", err)
	}
	if content != "" {
		t.Errorf("LoadChangelog() with no path = %q, want empty", content)
	}
}

// --- SaveState / LoadState / DeleteState ---

func TestReleaseService_SaveLoadState(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	svc.SaveState("processing")
	state := svc.LoadState()
	if state != "processing" {
		t.Errorf("LoadState() = %q, want 'processing'", state)
	}
}

func TestReleaseService_DeleteState(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	svc.SaveState("running")
	svc.DeleteState()
	state := svc.LoadState()
	if state != "" {
		t.Errorf("LoadState() after DeleteState() = %q, want empty", state)
	}
}

func TestReleaseService_LoadState_NoPath(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, "/tmp/log", "", "")
	svc := NewReleaseService(git, llm, &mockLogChunker{}, cfg)

	state := svc.LoadState()
	if state != "" {
		t.Errorf("LoadState() with no path = %q, want empty", state)
	}
}

// --- DeletePendingFiles ---

func TestReleaseService_DeletePendingFiles(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	os.MkdirAll(filepath.Dir(svc.cfg.IntentPath), 0755)
	os.WriteFile(svc.cfg.IntentPath, []byte("{}"), 0644)
	os.MkdirAll(filepath.Dir(svc.cfg.ChangelogPath), 0755)
	os.WriteFile(svc.cfg.ChangelogPath, []byte("# changelog"), 0644)

	svc.DeletePendingFiles()

	if _, err := os.Stat(svc.cfg.IntentPath); !os.IsNotExist(err) {
		t.Error("DeletePendingFiles() should remove intent file")
	}
	if _, err := os.Stat(svc.cfg.ChangelogPath); !os.IsNotExist(err) {
		t.Error("DeletePendingFiles() should remove changelog file")
	}
}

// --- parseSemver ---

func TestParseSemver(t *testing.T) {
	cases := []struct {
		tag                    string
		major, minor, patch int
	}{
		{"v1.2.3", 1, 2, 3},
		{"1.0.0", 1, 0, 0},
		{"v2.10.5", 2, 10, 5},
		{"v1.2.3-beta", 1, 2, 3},
		{"v0.0.1", 0, 0, 1},
		{"", 0, 0, 0},
	}
	for _, tc := range cases {
		maj, min, pat := parseSemver(tc.tag)
		if maj != tc.major || min != tc.minor || pat != tc.patch {
			t.Errorf("parseSemver(%q) = (%d,%d,%d), want (%d,%d,%d)",
				tc.tag, maj, min, pat, tc.major, tc.minor, tc.patch)
		}
	}
}

// --- previousTag ---

func TestPreviousTag(t *testing.T) {
	tags := []string{"v1.0.0", "v1.1.0", "v2.0.0"}

	if got := previousTag(tags, "v1.1.0"); got != "v1.0.0" {
		t.Errorf("previousTag(v1.1.0) = %q, want v1.0.0", got)
	}
	if got := previousTag(tags, "v2.0.0"); got != "v1.1.0" {
		t.Errorf("previousTag(v2.0.0) = %q, want v1.1.0", got)
	}
	// v1.0.0 is first — function falls through to "last sorted tag as reference"
	if got := previousTag(tags, "v1.0.0"); got != "v2.0.0" {
		t.Errorf("previousTag(first tag) = %q, want v2.0.0 (last sorted fallback)", got)
	}
}

func TestPreviousTag_TargetNotInList(t *testing.T) {
	tags := []string{"v1.0.0", "v1.1.0"}
	// Target not in list — returns last sorted tag as reference
	got := previousTag(tags, "v2.0.0")
	if got != "v1.1.0" {
		t.Errorf("previousTag(not in list) = %q, want v1.1.0", got)
	}
}

func TestPreviousTag_EmptyList(t *testing.T) {
	got := previousTag([]string{}, "v1.0.0")
	if got != "" {
		t.Errorf("previousTag(empty list) = %q, want empty", got)
	}
}

// --- PrepareAndGenerateAsync (smoke test — does not hang) ---

func TestReleaseService_PrepareAndGenerateAsync_Smoke(t *testing.T) {
	git := &mockGitForRelease{
		latestTagResult: "v1.0.0",
		commitsResult:   "commit abc\nfeat: test\n",
	}
	llm := &mockLLMForRelease{
		intentResult: &domain.ReleaseIntent{
			TagName:     "v1.1.0",
			VersionBump: "minor",
			IsRelease:   true,
		},
	}
	svc := newReleaseSvc(t, git, llm)

	// Should not panic or block
	svc.PrepareAndGenerateAsync("release minor version")
}

// --- countLines ---

func TestReleaseService_CountLines(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	if got := svc.countLines(""); got != 0 {
		t.Errorf("countLines(empty) = %d, want 0", got)
	}
	if got := svc.countLines("one"); got != 1 {
		t.Errorf("countLines(one line) = %d, want 1", got)
	}
	if got := svc.countLines("a\nb\nc"); got != 3 {
		t.Errorf("countLines(3 lines) = %d, want 3", got)
	}
}
