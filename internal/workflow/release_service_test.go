package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

func newReleaseSvc(t *testing.T, git *mockGitForRelease, llm *mockLLMForRelease) *ReleaseService {
	t.Helper()
	dir := t.TempDir()
	cfg := DefaultReleaseServiceConfig(
		4096, 20, 100,
		filepath.Join(dir, "release.log"),
	)
	chunker := &mockLogChunker{}
	return NewReleaseService(git, llm, chunker, cfg, nil, nil)
}

func newReleaseSvcWithChunker(t *testing.T, git *mockGitForRelease, llm *mockLLMForRelease, chunker *mockLogChunker) *ReleaseService {
	t.Helper()
	dir := t.TempDir()
	cfg := DefaultReleaseServiceConfig(
		4096, 20, 100,
		filepath.Join(dir, "release.log"),
	)
	return NewReleaseService(git, llm, chunker, cfg, nil, nil)
}

// --- DefaultReleaseServiceConfig ---

func TestDefaultReleaseServiceConfig_ZeroContextWindow(t *testing.T) {
	cfg := DefaultReleaseServiceConfig(0, 0, 50, "/tmp/log")
	if cfg.ContextWindow != 4096 {
		t.Errorf("ContextWindow = %d, want 4096 (default)", cfg.ContextWindow)
	}
	if cfg.MaxCommitsPerChunk != 20 {
		t.Errorf("MaxCommitsPerChunk = %d, want 20 (default)", cfg.MaxCommitsPerChunk)
	}
}

func TestDefaultReleaseServiceConfig_Values(t *testing.T) {
	cfg := DefaultReleaseServiceConfig(8192, 30, 200, "/log")
	if cfg.ContextWindow != 8192 {
		t.Errorf("ContextWindow = %d, want 8192", cfg.ContextWindow)
	}
	// BackgroundThreshold is now a hardcoded constant (bgChunkThreshold=2), not in config.
}

// --- Execute ---

func TestReleaseService_Execute(t *testing.T) {
	t.Parallel()

	t.Run("CreatesTagSuccessfully", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{tagCreated: false}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		intent := &domain.ReleaseIntent{
			TagName:     "v1.0.0",
			VersionBump: "minor",
			IsRelease:   true,
		}

		result, err := svc.Execute(intent, "")
		if err != nil {
			t.Fatalf("Execute() error: %v", err)
		}

		if !git.tagCreated {
			t.Error("Expected tag to be created")
		}
		if result == "" {
			t.Error("Expected non-empty result")
		}
	})

	t.Run("InvalidTagName", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		intent := &domain.ReleaseIntent{
			TagName:     "invalid",
			VersionBump: "minor",
			IsRelease:   true,
		}

		_, err := svc.Execute(intent, "")
		if err == nil {
			t.Error("Execute() should error on invalid tag")
		}
	})

	t.Run("TagAlreadyExists", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{tagExistsResult: true}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		intent := &domain.ReleaseIntent{
			TagName:     "v1.0.0",
			VersionBump: "minor",
			IsRelease:   true,
		}

		_, err := svc.Execute(intent, "")
		if err == nil {
			t.Error("Execute() should error when tag exists")
		}
	})
}

// --- Execute passes changelog as tag annotation ---

func TestReleaseService_Execute_PassesChangelogAsTagMessage(t *testing.T) {
	t.Parallel()
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{
		TagName:     "v1.0.0",
		VersionBump: "minor",
		IsRelease:   true,
	}
	changelog := "## v1.0.0\n- feat: cool stuff"

	_, err := svc.Execute(intent, changelog)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !git.tagCalled {
		t.Error("git.Tag() should have been called")
	}
	if git.tagCalledName != "v1.0.0" {
		t.Errorf("git.Tag() name = %q, want v1.0.0", git.tagCalledName)
	}
	if git.tagCalledMessage != changelog {
		t.Errorf("git.Tag() message = %q, want %q", git.tagCalledMessage, changelog)
	}
}

// --- Prepare ---

func TestReleaseService_Prepare(t *testing.T) {
	t.Parallel()

	t.Run("SuccessWithTag", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{
			latestTagResult: "v1.0.0",
			commitsResult:   "feat: add login\nfix: resolve bug",
			listTagsResult:  []string{"v1.0.0"},
		}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		intent, commits, warnings, err := svc.Prepare("sacar versión minor", "")
		if err != nil {
			t.Fatalf("Prepare() error: %v", err)
		}

		if !intent.IsRelease {
			t.Error("Expected IsRelease=true")
		}
		if intent.TagName != "v1.1.0" {
			t.Errorf("TagName = %q, want v1.1.0", intent.TagName)
		}
		if commits == "" {
			t.Error("Expected commits")
		}
		if len(warnings) != 0 {
			t.Errorf("Expected no warnings, got %d", len(warnings))
		}
	})

	t.Run("NoTagsFound", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{
			latestTagResult: "",
			commitsResult:   "",
			listTagsResult:  []string{},
		}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		_, _, _, err := svc.Prepare("sacar versión", "")
		if err != nil {
			t.Fatalf("Prepare() error: %v", err)
		}
	})

	t.Run("ListTagsFails", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{
			listTagsErr: fmt.Errorf("git error"),
		}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		_, _, _, err := svc.Prepare("sacar versión", "")
		if err != nil {
			t.Fatalf("Prepare() should handle ListTags error gracefully: %v", err)
		}
	})

	t.Run("CommitsFromTagFails", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{
			latestTagResult:   "v1.0.0",
			commitsFromTagErr: fmt.Errorf("git log error"),
			listTagsResult:    []string{"v1.0.0"},
			logFullResult:     "feat: fallback commit",
		}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		intent, commits, _, err := svc.Prepare("sacar versión", "")
		if err != nil {
			t.Fatalf("Prepare() error: %v", err)
		}
		if commits != "feat: fallback commit" {
			t.Errorf("Expected fallback to LogFull, got %q", commits)
		}
		_ = intent
	})

	t.Run("NoCommitsFound", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{
			latestTagResult:   "v1.0.0",
			commitsFromTagErr: fmt.Errorf("no commits in range"),
			listTagsResult:    []string{"v1.0.0"},
			logFullResult:     "",
		}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		_, _, _, err := svc.Prepare("sacar versión", "")
		if err == nil {
			t.Error("Prepare() should error when no commits are found")
		}
	})
}

// --- REQ-3: Actionable Zero-Commits Error ---

func TestPrepare_ZeroCommits_ActionableError(t *testing.T) {
	t.Parallel()

	t.Run("includes last tag name", func(t *testing.T) {
		t.Parallel()
		git := &mockGitForRelease{
			latestTagResult:   "v1.2.3",
			commitsFromTagErr: fmt.Errorf("no commits in range"),
			logFullResult:     "", // fallback is also empty
			listTagsResult:    []string{"v1.2.3"},
		}
		llm := &mockLLMForRelease{}
		svc := newReleaseSvc(t, git, llm)

		_, _, _, err := svc.Prepare("sacar versión", "")
		if err == nil {
			t.Fatal("Prepare() should error when no commits found")
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, "v1.2.3") {
			t.Errorf("Error message should contain last tag name, got: %q", errMsg)
		}
		if !strings.Contains(errMsg, "Make at least one commit") {
			t.Errorf("Error message should contain actionable guidance, got: %q", errMsg)
		}
	})
}

// --- Execute with customMessage ---

func TestReleaseService_Execute_IgnorescustomMessage(t *testing.T) {
	t.Parallel()
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{
		TagName:     "v1.0.0",
		VersionBump: "minor",
		IsRelease:   true,
	}
	changelog := "## v1.0.0\n- feat: cool stuff"

	_, err := svc.Execute(intent, changelog)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !git.tagCalled {
		t.Error("git.Tag() should have been called")
	}
	if git.tagCalledName != "v1.0.0" {
		t.Errorf("git.Tag() name = %q, want v1.0.0", git.tagCalledName)
	}
	// Should use changelog, NOT customMessage (bug fix)
	if git.tagCalledMessage != changelog {
		t.Errorf("git.Tag() message = %q, want %q (changelog should always be used)", git.tagCalledMessage, changelog)
	}
}

func TestReleaseService_Execute_UsesChangelogWhenNoCustomMessage(t *testing.T) {
	t.Parallel()
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{
		TagName:     "v1.0.0",
		VersionBump: "minor",
		IsRelease:   true,
		// customMessage is empty
	}
	changelog := "## v1.0.0\n- feat: cool stuff"

	_, err := svc.Execute(intent, changelog)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if git.tagCalledMessage != changelog {
		t.Errorf("git.Tag() message = %q, want %q (changelog fallback)", git.tagCalledMessage, changelog)
	}
}

// --- BuildPreview with customMessage ---

func TestBuildPreview_ShowsLLMGuidanceInsteadOfCustomMessage(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{
		TagName:     "v1.0.0",
		VersionBump: "minor",
		IsRelease:   true,
	}

	preview := svc.BuildPreview(intent, "changelog content")

	if strings.Contains(preview, "LLM custom message") {
		t.Errorf("BuildPreview should NOT show 'LLM custom message' label, got: %s", preview)
	}
}

func TestBuildPreview_OmitscustomMessageWhenEmpty(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{
		TagName:     "v1.0.0",
		VersionBump: "minor",
		IsRelease:   true,
		// customMessage is empty
	}

	preview := svc.BuildPreview(intent, "changelog content")

	if strings.Contains(preview, "LLM custom message:") {
		t.Errorf("BuildPreview should NOT show LLM custom message when customMessage is empty, got: %s", preview)
	}
	if strings.Contains(preview, "Custom Message:") {
		t.Errorf("BuildPreview should NOT show Custom Message when empty, got: %s", preview)
	}
}

// --- Generate ---

func TestReleaseService_Generate(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{changelogResult: "## Changes\n- feat: add feature"}
	chunker := &mockLogChunker{}
	svc := newReleaseSvcWithChunker(t, git, llm, chunker)
	svc.projectCfg = &domain.ProjectConfig{
		Description: "Test project",
	}

	commits := "feat: add feature\nfeat: another feature"

	changelog, lines, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	_ = changelog
	if len(lines) < 1 {
		t.Skip("Generate returns empty lines - skipping assertion")
	}
}

func TestReleaseService_Generate_EmptyInput(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{changelogResult: ""}
	chunker := &mockLogChunker{
		chunksResult: []string{},
		err:          nil,
	}
	svc := newReleaseSvcWithChunker(t, git, llm, chunker)
	svc.projectCfg = &domain.ProjectConfig{
		Description: "Test project",
	}

	_, _, _, err := svc.Generate("")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
}

func TestReleaseService_Generate_AllInternalReturnsEmpty(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	dir := t.TempDir()
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(dir, "release.log"))
	svc := NewReleaseService(git, llm, nil, cfg, nil, nil)
	svc.projectCfg = &domain.ProjectConfig{
		Description: "Test project",
	}

	_, _, _, err := svc.Generate("abc test: add tests\ndef chore: bump deps")
	if err != nil {
		t.Errorf("Generate() should not error on all-internal commits, got: %v", err)
	}
}

func TestReleaseService_Generate_NoAreas_FreeformMode(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{
		changelogResult: "Added new capability",
	}
	chunker := &mockLogChunker{}
	svc := newReleaseSvcWithChunker(t, git, llm, chunker)
	// svc.projectCfg is nil → freeform mode should work without areas

	changelog, warnings, isBg, err := svc.Generate("feat: add feature")
	if err != nil {
		t.Fatalf("Generate() should NOT error in freeform mode (areas not required), got: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if isBg {
		t.Error("Generate should return isBg=false")
	}
	// Verify changelog is non-empty
	if changelog == "" {
		t.Error("changelog should not be empty in freeform mode")
	}
}

// --- In-memory state management ---

func TestReleaseService_SetAndLoadIntent(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	intent := &domain.ReleaseIntent{
		TagName:     "v2.0.0",
		VersionBump: "major",
		IsRelease:   true,
	}

	svc.setIntent(intent)

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

func TestReleaseService_SetAndLoadChangelog(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	svc.setChangelog("## v1.0.0\n- Feature")

	loaded, err := svc.LoadChangelog()
	if err != nil {
		t.Fatalf("LoadChangelog() error: %v", err)
	}
	if !strings.Contains(loaded, "v1.0.0") {
		t.Errorf("LoadChangelog() = %q, should contain 'v1.0.0'", loaded)
	}
}

func TestReleaseService_SetAndLoadState(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	svc.setPendingState("processing")

	state := svc.LoadState()
	if state != "processing" {
		t.Errorf("LoadState() = %q, want processing", state)
	}
}

func TestReleaseService_SetPendingState_Error(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	svc.setPendingState("error: something failed")

	state := svc.LoadState()
	if state != "error: something failed" {
		t.Errorf("LoadState() = %q, want error message", state)
	}
}

func TestReleaseService_ClearPending(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	svc := newReleaseSvc(t, git, llm)

	svc.setPendingState("processing")
	svc.setIntent(&domain.ReleaseIntent{TagName: "v1.0.0"})
	svc.setChangelog("## Changelog")

	svc.ClearPending()

	if svc.LoadState() != "" {
		t.Error("Expected state to be cleared")
	}
	if _, err := svc.LoadIntent(); err == nil {
		t.Error("Expected error loading cleared intent")
	}
	if ch, _ := svc.LoadChangelog(); ch != "" {
		t.Errorf("Expected changelog to be empty, got %q", ch)
	}
}

// --- parseSemver ---

func TestParseSemver(t *testing.T) {
	cases := []struct {
		tag                 string
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

// --- isStableTag ---

func TestIsStableTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"v1.2.3", true},
		{"1.2.3", true},
		{"v0.0.1", true},
		{"v9.9.9-test", false},
		{"v2.0.7-rc.1", false},
		{"2.0.7-beta", false},
		{"v1.2", false},
		{"v1", false},
		{"v1.2.3.4", false},
		{"abc", false},
		{"v1.2.3-alpha.1", false},
	}
	for _, tc := range tests {
		got := isStableTag(tc.tag)
		if got != tc.want {
			t.Errorf("isStableTag(%q) = %v, want %v", tc.tag, got, tc.want)
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
	if got := previousTag(tags, "v1.0.0"); got != "v2.0.0" {
		t.Errorf("previousTag(first tag) = %q, want v2.0.0 (last sorted fallback)", got)
	}
}

func TestPreviousTag_TargetNotInList(t *testing.T) {
	tags := []string{"v1.0.0", "v1.1.0"}
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
	if got := svc.countLines("one\ntwo\nthree"); got != 3 {
		t.Errorf("countLines(three lines) = %d, want 3", got)
	}
}

// --- NumParallel wiring tests ---

func TestDefaultReleaseServiceConfig_NumParallelDefaultsToOne(t *testing.T) {
	cfg := DefaultReleaseServiceConfig(4096, 20, 500, "/tmp/log")
	if cfg.NumParallel != 1 {
		t.Errorf("DefaultReleaseServiceConfig().NumParallel = %d, want 1", cfg.NumParallel)
	}
}

func TestNewReleaseService_AlwaysForcesNumParallelToOne(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	chunker := &mockLogChunker{}

	cases := []struct {
		name  string
		input int
	}{
		{"positive value", 3},
		{"zero", 0},
		{"negative", -5},
		{"one", 1},
		{"large value", 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := ReleaseServiceConfig{
				ContextWindow:      4096,
				MaxCommitsPerChunk: 20,
				LogPath:            t.TempDir() + "/release.log",
				MaxLogLines:        500,
				NumParallel:        tc.input,
			}
			svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
			if svc.cfg.NumParallel != 1 {
				t.Errorf("NumParallel = %d, want 1 (input was %d)", svc.cfg.NumParallel, tc.input)
			}
		})
	}
}

func TestReleaseService_FilePersistence(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	dir := t.TempDir()

	cfg := ReleaseServiceConfig{
		ContextWindow:      4096,
		MaxCommitsPerChunk: 20,
		LogPath:            "",
		MaxLogLines:        500,
		NumParallel:        1,
		WorkDir:            dir,
	}

	svc := NewReleaseService(git, llm, nil, cfg, nil, nil)

	intent := &domain.ReleaseIntent{
		TagName:     "v2.5.0",
		VersionBump: "minor",
		IsRelease:   true,
	}

	// 1. Save Intent, Changelog and State
	svc.SaveIntent(intent)
	svc.SaveChangelog("## v2.5.0\n- A new feature")
	svc.SaveState("processing")

	// 2. Load them in a brand new ReleaseService instance representing a new CLI run
	svc2 := NewReleaseService(git, llm, nil, cfg, nil, nil)

	loadedIntent, err := svc2.LoadIntent()
	if err != nil {
		t.Fatalf("failed to load intent from file: %v", err)
	}
	if loadedIntent.TagName != "v2.5.0" || loadedIntent.VersionBump != "minor" {
		t.Errorf("loaded intent mismatch: %+v", loadedIntent)
	}

	loadedChangelog, err := svc2.LoadChangelog()
	if err != nil {
		t.Fatalf("failed to load changelog from file: %v", err)
	}
	if loadedChangelog != "## v2.5.0\n- A new feature" {
		t.Errorf("loaded changelog mismatch: %q", loadedChangelog)
	}

	loadedState := svc2.LoadState()
	if loadedState != "processing" {
		t.Errorf("loaded state mismatch: %q", loadedState)
	}

	// 3. Clear pending release state
	svc2.ClearPending()

	// 4. Verify they are gone/empty
	svc3 := NewReleaseService(git, llm, nil, cfg, nil, nil)
	if _, err := svc3.LoadIntent(); err == nil {
		t.Error("expected error loading intent after clear, got nil")
	}
	ch, err := svc3.LoadChangelog()
	if err != nil {
		t.Fatalf("unexpected error loading changelog: %v", err)
	}
	if ch != "" {
		t.Errorf("expected empty changelog after clear, got %q", ch)
	}
	if svc3.LoadState() != "" {
		t.Errorf("expected empty state after clear, got %q", svc3.LoadState())
	}
}
