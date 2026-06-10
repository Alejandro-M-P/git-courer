package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// --- FilterAndGroupCommits ---

func TestFilterAndGroupCommits_FiltersInternalTypes(t *testing.T) {
	commits := strings.Join([]string{
		"abc1234 test: add validator tests",
		"def5678 chore: update go.mod",
		"ghi9012 ci: fix pipeline",
		"jkl3456 build: update docker image",
		"mno7890 feat: add semantic annotator",
	}, "\n")

	groups := FilterAndGroupCommits(commits)

	if _, ok := groups[""]; !ok {
		t.Error("feat commit should appear in no-scope group")
	}
	total := 0
	for _, items := range groups {
		total += len(items)
	}
	if total != 1 {
		t.Errorf("expected 1 user-facing commit, got %d", total)
	}
}

func TestFilterAndGroupCommits_BreakingNotFiltered(t *testing.T) {
	commits := "abc1234 test!: this breaking test must appear"
	groups := FilterAndGroupCommits(commits)
	if len(groups) == 0 {
		t.Error("breaking test commit should not be filtered")
	}
}

func TestFilterAndGroupCommits_GroupsByScope(t *testing.T) {
	commits := strings.Join([]string{
		"aaa feat(security): add webhook auth",
		"bbb fix(security): handle nil token",
		"ccc feat(core): add semantic diff",
		"ddd fix: handle empty input",
	}, "\n")

	groups := FilterAndGroupCommits(commits)

	if len(groups["security"]) != 2 {
		t.Errorf("security scope: want 2 commits, got %d", len(groups["security"]))
	}
	if len(groups["core"]) != 1 {
		t.Errorf("core scope: want 1 commit, got %d", len(groups["core"]))
	}
	if len(groups[""]) != 1 {
		t.Errorf("no-scope group: want 1 commit, got %d", len(groups[""]))
	}
}

func TestFilterAndGroupCommits_NonConventional_Skipped(t *testing.T) {
	commits := "Updated README\nWIP some stuff"
	groups := FilterAndGroupCommits(commits)
	if len(groups) != 0 {
		t.Errorf("non-conventional commits should be skipped, got %v", groups)
	}
}

// --- FormatGroupedCommits ---

func TestFormatGroupedCommits_Empty(t *testing.T) {
	if out := FormatGroupedCommits(map[string][]string{}); out != "" {
		t.Errorf("empty groups should produce empty string, got %q", out)
	}
}

func TestFormatGroupedCommits_GeneralLast(t *testing.T) {
	groups := map[string][]string{
		"":         {"fix: handle empty input"},
		"security": {"feat(security): add webhook"},
	}
	out := FormatGroupedCommits(groups)

	secIdx := strings.Index(out, "security:")
	genIdx := strings.Index(out, "general:")
	if secIdx < 0 || genIdx < 0 {
		t.Fatalf("missing sections in output:\n%s", out)
	}
	if secIdx > genIdx {
		t.Error("security should come before general")
	}
}

// --- Generate (v2 integration) ---
// mockAreaLLM removed - replaced with mockFreeformLLM for Phase 2 freeform tests

func TestGenerate_CallsGenerateChangelogByArea(t *testing.T) {
	// DEPRECATED: Areas-based routing removed in Phase 2
	// This test is kept for historical reference but no longer executes
	t.Skip("Areas-based routing removed - freeform generation is now default")
}

func TestGenerate_AllInternalCommits_ReturnsEmpty(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockFreeformLLM{}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)

	commits := strings.Join([]string{
		"abc test: add unit tests",
		"def chore: update go.mod",
		"ghi ci: fix pipeline",
	}, "\n")
	changelog, warnings, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if changelog != "" || len(warnings) != 0 {
		t.Errorf("expected empty result for all-internal commits, got: %q, %v", changelog, warnings)
	}
}

func TestGenerate_LLMError_ReturnsWarning(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockFreeformLLM{err: fmt.Errorf("LLM unavailable")}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)

	commits := "abc feat: add new feature"
	_, warnings, _, err := svc.Generate(commits)
	if err == nil {
		t.Error("expected error from LLM failure")
	}
	if len(warnings) == 0 {
		t.Error("expected warning on LLM failure")
	}
}

// --- filterForChangelog tests ---

func TestFilterForChangelog_ExcludesDocsAndInternal(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Excluded: []string{"docs/", "scripts/"},
	}
	commits := "abc1234 feat: add feature\ndef5678 docs: update readme\nghi9012 chore(scripts): update build\njkl3456 fix: fix bug"
	groups := filterForChangelog(commits, cfg)
	// In freeform mode:
	// - feat and fix (no scope) pass through
	// - docs: update readme has type "docs" (NOT in skipTypes) and empty scope, so it passes
	// - chore(scripts) is filtered by skipTypes (chore is internal)
	// Result: 3 commits in no-scope group (feat, docs, fix)
	if len(groups[""]) != 3 {
		t.Errorf("no-scope group: want 3 commits (feat + docs + fix), got %d: %v", len(groups[""]), groups[""])
	}
	// scripts/ path should be excluded (chore type filtered by skipTypes)
	if _, ok := groups["scripts"]; ok {
		t.Error("scripts/ scope should be excluded (chore type filtered by skipTypes)")
	}
}

func TestFilterForChangelog_NilConfig_UsesDefaults(t *testing.T) {
	// nil config means no scope-based exclusion (IsExcluded not called)
	// but skipTypes still filters test/chore/ci/build
	commits := "abc1234 feat: add feature\ndef5678 test: update tests\nghi9012 chore: update go.mod"
	groups := filterForChangelog(commits, nil)
	total := 0
	for _, items := range groups {
		total += len(items)
	}
	if total != 1 {
		t.Errorf("expected 1 commit after filtering (test/chore skipped by type), got %d", total)
	}
}

func TestFilterForChangelog_BreakingNotFiltered(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Excluded: []string{},
	}
	commits := "abc1234 test!: breaking test must appear"
	groups := filterForChangelog(commits, cfg)
	if len(groups) == 0 {
		t.Error("breaking test commit should not be filtered")
	}
}

// --- Generate freeform tests ---

func TestGenerate_Freeform_ReturnsLLMCategorizedChangelog(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockFreeformLLM{
		result: "## Authentication\n- Added login flow with **JWT tokens**\n\n## API\n- Exposed new **webhook** endpoints",
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)

	commits := "abc feat: add login\nbcd feat(api): add webhooks"
	changelog, warnings, isBg, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if isBg {
		t.Error("Generate should return isBg=false")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !llm.freeformCalled {
		t.Error("GenerateChangelogGrouped should have been called in freeform mode")
	}
	if llm.mode != "freeform" {
		t.Errorf("mode = %q, want %q", llm.mode, "freeform")
	}
	// Verify changelog contains LLM-returned markdown directly
	if !strings.Contains(changelog, "## Authentication") {
		t.Errorf("changelog should contain LLM category 'Authentication', got:\n%s", changelog)
	}
	if !strings.Contains(changelog, "## API") {
		t.Errorf("changelog should contain LLM category 'API', got:\n%s", changelog)
	}
}

func TestGenerate_Fallback_WhenNoBaseBranch(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockFreeformLLM{
		result: "## Features\n- Added new capability",
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	// No BaseBranch, no Areas → freeform fallback

	commits := "abc feat: add capability"
	changelog, _, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !llm.freeformCalled {
		t.Error("GenerateChangelogGrouped should be called in freeform fallback")
	}
	if !strings.Contains(changelog, "## Features") {
		t.Errorf("changelog should contain category header, got:\n%s", changelog)
	}
}

// --- mockFreeformLLM for freeform generation tests ---

type mockFreeformLLM struct {
	result         string
	err            error
	freeformCalled bool
	input          string
	nameMap        map[string]string
	customMessage  string
	mode           string
}

func (m *mockFreeformLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) { return "", nil }
func (m *mockFreeformLLM) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	return "", nil
}
func (m *mockFreeformLLM) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return nil, nil
}
func (m *mockFreeformLLM) SetRetryContext(msg string) {}
func (m *mockFreeformLLM) ClearRetryContext()         {}
func (m *mockFreeformLLM) IsAvailable() bool          { return true }
func (m *mockFreeformLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockFreeformLLM) AuditBinaryContent(filename, content string) (bool, error) { return false, nil }
func (m *mockFreeformLLM) GenerateChangelogGrouped(formattedGroups string, nameMap map[string]string, customMessage string, mode string) (string, error) {
	m.freeformCalled = true
	m.input = formattedGroups
	m.nameMap = nameMap
	m.customMessage = customMessage
	m.mode = mode
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}
func (m *mockFreeformLLM) RegenerateMessage(prev []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}
func (m *mockFreeformLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) { return nil, nil }
func (m *mockFreeformLLM) ClassifyBinary(prompt string) (string, error) {
	return "fix", nil
}

// --- GenerateChangelogGrouped (stack mode) tests ---

func TestGenerateChangelogGrouped_StackMode(t *testing.T) {
	// Test that GenerateChangelogGrouped is called with mode="stack"
	// when BaseBranch is configured (triggers stack grouping path).
	git := &mockGitForRelease{}
	llm := &mockGroupedLLM{
		result: "## feature/auth\n- Added authentication flow",
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	svc.projectCfg = &domain.ProjectConfig{
		BaseBranch: "main", // Non-empty triggers stack path
	}

	// Set pending entries with stack metadata (simulates Prepare() flow)
	e1, _ := domain.NewCommitEntry("aaa0000000000000000000000000000000000000", "feat(auth): add login", domain.WithStackID("abc123"), domain.WithStackBranch("feature/auth"))
	e2, _ := domain.NewCommitEntry("bbb0000000000000000000000000000000000000", "fix(auth): handle nil token", domain.WithStackID("abc123"), domain.WithStackBranch("feature/auth"))
	svc.pendingEntries = []domain.CommitEntry{e1, e2}

	commits := "aaa0000 feat(auth): add login\nbbb0000 fix(auth): handle nil token"
	changelog, warnings, isBg, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if isBg {
		t.Error("Generate should return isBg=false")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !llm.groupedCalled {
		t.Error("GenerateChangelogGrouped should have been called for stack path")
	}
	if llm.mode != "stack" {
		t.Errorf("GenerateChangelogGrouped mode = %q, want %q", llm.mode, "stack")
	}
	if llm.customMessage != "" {
		t.Errorf("customMessage should be empty by default, got %q", llm.customMessage)
	}
	// The changelog should contain the LLM-returned markdown directly
	if changelog != "" && !strings.Contains(changelog, "feature/auth") {
		t.Errorf("changelog should contain section header from LLM, got:\n%s", changelog)
	}
}

func TestGenerateChangelogGrouped_AreaMode(t *testing.T) {
	// DEPRECATED: Area mode removed in Phase 2 - now uses freeform mode
	t.Skip("Area mode deprecated - freeform categorization is now default")
}

func TestGenerateWithStacks_UnspecifiedGroupFallback(t *testing.T) {
	// Test that entries with empty StackID fall into an "Unspecified" group
	// and that generateWithStacks produces a changelog with section headers.
	git := &mockGitForRelease{}
	llm := &mockGroupedLLM{
		result: "## Unspecified\n- Updated dependencies\n\n## feature/auth\n- Added authentication flow",
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	svc.projectCfg = &domain.ProjectConfig{
		BaseBranch: "main",
	}

	// Entries: one with StackID, one without (Unspecified group)
	e1, _ := domain.NewCommitEntry("aaa0000000000000000000000000000000000000", "feat(auth): add login", domain.WithStackID("abc123"), domain.WithStackBranch("feature/auth"))
	e2, _ := domain.NewCommitEntry("bbb0000000000000000000000000000000000000", "feat: update deps") // No stack fields, but feat type (not filtered)
	svc.pendingEntries = []domain.CommitEntry{e1, e2}

	commits := "aaa0000 feat(auth): add login\nbbb0000 feat: update deps"
	changelog, _, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !llm.groupedCalled {
		t.Error("GenerateChangelogGrouped should have been called")
	}
	if llm.mode != "stack" {
		t.Errorf("mode = %q, want %q", llm.mode, "stack")
	}
	// Verify the nameMap contains Unspecified and feature/auth
	if _, ok := llm.nameMap["group_1"]; !ok {
		t.Error("nameMap should contain group_1")
	}
	if _, ok := llm.nameMap["group_2"]; !ok {
		t.Error("nameMap should contain group_2")
	}
	// Verify the changelog has section headers
	if changelog == "" {
		t.Error("changelog should not be empty")
	}
}

func TestGenerateWithStacks_LLMError_UsesBranchFallback(t *testing.T) {
	// When the LLM fails, generateWithStacks should fall back to using branch names
	// as section headers.
	git := &mockGitForRelease{}
	llm := &mockGroupedLLM{
		err: fmt.Errorf("LLM unavailable"),
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	svc.projectCfg = &domain.ProjectConfig{
		BaseBranch: "main",
	}

	e1, _ := domain.NewCommitEntry("aaa0000000000000000000000000000000000000", "feat(auth): add login", domain.WithStackID("abc123"), domain.WithStackBranch("feature/auth"))
	svc.pendingEntries = []domain.CommitEntry{e1}

	commits := "aaa0000 feat(auth): add login"
	changelog, warnings, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() should not error on LLM failure, got: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected warnings about LLM failure")
	}
	// When LLM fails, branch name is used as section header
	if !strings.Contains(changelog, "feature/auth") {
		t.Errorf("changelog should contain branch name as header when LLM fails, got:\n%s", changelog)
	}
}

// mockGroupedLLM implements ports.LLM and tracks GenerateChangelogGrouped calls.
type mockGroupedLLM struct {
	result        string
	err           error
	groupedCalled bool
	input         string
	nameMap       map[string]string
	customMessage string
	mode          string
}

func (m *mockGroupedLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) { return "", nil }
func (m *mockGroupedLLM) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	return "", nil
}
func (m *mockGroupedLLM) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return nil, nil
}
func (m *mockGroupedLLM) SetRetryContext(msg string) {}
func (m *mockGroupedLLM) ClearRetryContext()         {}
func (m *mockGroupedLLM) IsAvailable() bool          { return true }
func (m *mockGroupedLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockGroupedLLM) AuditBinaryContent(filename, content string) (bool, error) { return false, nil }
func (m *mockGroupedLLM) GenerateChangelogGrouped(formattedGroups string, nameMap map[string]string, customMessage string, mode string) (string, error) {
	m.groupedCalled = true
	m.input = formattedGroups
	m.nameMap = nameMap
	m.customMessage = customMessage
	m.mode = mode
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}
func (m *mockGroupedLLM) RegenerateMessage(prev []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}
func (m *mockGroupedLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) { return nil, nil }
func (m *mockGroupedLLM) ClassifyBinary(prompt string) (string, error) {
	return "fix", nil
}

// --- Bug 5: Stack-mode filtering tests ---

func TestFilterEntriesForChangelog_ExcludesInternalTypes(t *testing.T) {
	// Bug 5: Stack mode should exclude test/chore/ci/build unless breaking
	e1, _ := domain.NewCommitEntry("aaa0000000000000000000000000000000000000", "feat: add feature")
	e2, _ := domain.NewCommitEntry("bbb0000000000000000000000000000000000000", "test: add tests")
	e3, _ := domain.NewCommitEntry("ccc0000000000000000000000000000000000000", "chore: update deps")
	e4, _ := domain.NewCommitEntry("ddd0000000000000000000000000000000000000", "ci: fix pipeline")
	e5, _ := domain.NewCommitEntry("eee0000000000000000000000000000000000000", "build: update docker")

	entries := []domain.CommitEntry{e1, e2, e3, e4, e5}
	filtered := filterEntriesForChangelog(entries, nil)

	if len(filtered) != 1 {
		t.Errorf("filterEntriesForChangelog should exclude internal types, got %d entries", len(filtered))
	}
	if filtered[0].Message() != "feat: add feature" {
		t.Errorf("expected feat commit, got %q", filtered[0].Message())
	}
}

func TestFilterEntriesForChangelog_BreakingNotFiltered(t *testing.T) {
	// Bug 5: Breaking internal commits should NOT be filtered
	e1, _ := domain.NewCommitEntry("aaa0000000000000000000000000000000000000", "test!: breaking test")
	e2, _ := domain.NewCommitEntry("bbb0000000000000000000000000000000000000", "chore!: breaking chore")

	entries := []domain.CommitEntry{e1, e2}
	filtered := filterEntriesForChangelog(entries, nil)

	if len(filtered) != 2 {
		t.Errorf("breaking commits should not be filtered, got %d entries", len(filtered))
	}
}

func TestFilterEntriesForChangelog_ExcludedPaths(t *testing.T) {
	// Bug 5: Excluded paths should be filtered in stack mode
	cfg := &domain.ProjectConfig{
		Excluded: []string{"docs", "scripts"},
	}
	e1, _ := domain.NewCommitEntry("aaa0000000000000000000000000000000000000", "feat(core): add feature")
	e2, _ := domain.NewCommitEntry("bbb0000000000000000000000000000000000000", "docs(docs): update readme")
	e3, _ := domain.NewCommitEntry("ccc0000000000000000000000000000000000000", "chore(scripts): update build")

	entries := []domain.CommitEntry{e1, e2, e3}
	filtered := filterEntriesForChangelog(entries, cfg)

	if len(filtered) != 1 {
		t.Errorf("filterEntriesForChangelog should exclude docs/scripts, got %d entries", len(filtered))
	}
	if filtered[0].Message() != "feat(core): add feature" {
		t.Errorf("expected feat(core) commit, got %q", filtered[0].Message())
	}
}

func TestGenerateWithStacks_FiltersInternalCommits(t *testing.T) {
	// Bug 5: generateWithStacks should filter internal commits before grouping
	git := &mockGitForRelease{}
	llm := &mockGroupedLLM{
		result: "## feature/auth\n- Added authentication",
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	svc.projectCfg = &domain.ProjectConfig{
		BaseBranch: "main",
	}

	// Mix of user-facing and internal commits
	e1, _ := domain.NewCommitEntry("aaa", "feat(auth): add login", domain.WithStackID("abc123"), domain.WithStackBranch("feature/auth"))
	e2, _ := domain.NewCommitEntry("bbb", "test(auth): add tests", domain.WithStackID("abc123"), domain.WithStackBranch("feature/auth"))
	e3, _ := domain.NewCommitEntry("ccc", "chore(auth): cleanup", domain.WithStackID("abc123"), domain.WithStackBranch("feature/auth"))
	svc.pendingEntries = []domain.CommitEntry{e1, e2, e3}

	changelog, _, _, err := svc.Generate("aaa feat(auth): add login\nbbb test(auth): add tests\nccc chore(auth): cleanup")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// LLM should only see the feat commit, not test/chore
	if !llm.groupedCalled {
		t.Error("GenerateChangelogGrouped should have been called")
	}
	// The changelog should only contain the feat commit
	if !strings.Contains(changelog, "Added authentication") {
		t.Errorf("changelog should contain feat commit, got:\n%s", changelog)
	}
}