package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// --- formatChangelogMarkdown (legacy, kept for reference) ---

func TestFormatChangelogMarkdown_AllSections(t *testing.T) {
	ch := &domain.Changelog{
		Features: []string{"add login", "add logout"},
		Fixes:    []string{"fix crash"},
		Breaking: []string{"remove old API"},
		Docs:     []string{"update readme"},
		Perf:     []string{"faster queries"},
		Internal: []string{"refactor"},
	}

	md := formatChangelogMarkdown(ch)

	for _, want := range []string{"## Features", "## Fixes", "## Breaking Changes", "## Documentation", "## Performance", "## Internal"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing section header %q", want)
		}
	}
	for _, want := range []string{"- add login", "- fix crash", "- remove old API", "- update readme", "- faster queries", "- refactor"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing item %q", want)
		}
	}
}

func TestFormatChangelogMarkdown_EmptySections(t *testing.T) {
	ch := &domain.Changelog{Features: []string{"add feature"}}
	md := formatChangelogMarkdown(ch)
	if strings.Contains(md, "## Fixes") {
		t.Error("empty Fixes section should not appear")
	}
	if !strings.Contains(md, "## Features") {
		t.Error("non-empty Features section should appear")
	}
}

func TestFormatChangelogMarkdown_EmptyChangelog(t *testing.T) {
	ch := &domain.Changelog{}
	if md := formatChangelogMarkdown(ch); md != "" {
		t.Errorf("empty changelog should produce empty string, got %q", md)
	}
}

// --- formatChangelogByAreaMarkdown ---

func TestFormatChangelogByAreaMarkdown_AreasSortedGeneralLast(t *testing.T) {
	ch := domain.ChangelogByArea{
		"security": []string{"Fixed auth bypass"},
		"core":     []string{"Added semantic diff"},
		"general":  []string{"Updated dependencies"},
	}
	md := formatChangelogByAreaMarkdown(ch)

	coreIdx := strings.Index(md, "## Core")
	secIdx := strings.Index(md, "## Security")
	genIdx := strings.Index(md, "## General")

	if coreIdx < 0 || secIdx < 0 || genIdx < 0 {
		t.Fatalf("missing sections: %s", md)
	}
	if coreIdx > secIdx {
		t.Error("core should come before security (alphabetical)")
	}
	if genIdx < secIdx {
		t.Error("general should be last")
	}
}

func TestFormatChangelogByAreaMarkdown_EmptyAreas(t *testing.T) {
	ch := domain.ChangelogByArea{
		"security": []string{"Fixed auth bypass"},
		"core":     []string{},
	}
	md := formatChangelogByAreaMarkdown(ch)
	if strings.Contains(md, "## Core") {
		t.Error("empty core area should not appear")
	}
	if !strings.Contains(md, "## Security") {
		t.Error("non-empty security area should appear")
	}
}

func TestFormatChangelogByAreaMarkdown_Empty(t *testing.T) {
	if md := formatChangelogByAreaMarkdown(domain.ChangelogByArea{}); md != "" {
		t.Errorf("empty changelog should produce empty string, got %q", md)
	}
}

// --- FilterAndGroupCommits ---

func TestFilterAndGroupCommits_FiltersInternalTypes(t *testing.T) {
	commits := strings.Join([]string{
		"abc1234 test: add validator tests",
		"def5678 chore: update go.mod",
		"ghi9012 ci: fix pipeline",
		"jkl3456 build: update docker image",
		"mno7890 feat(core): add semantic annotator",
	}, "\n")

	groups := FilterAndGroupCommits(commits)

	if _, ok := groups["core"]; !ok {
		t.Error("feat commit should appear in core area")
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
		t.Errorf("security area: want 2 commits, got %d", len(groups["security"]))
	}
	if len(groups["core"]) != 1 {
		t.Errorf("core area: want 1 commit, got %d", len(groups["core"]))
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

type mockAreaLLM struct {
	result domain.ChangelogByArea
	err    error
	called string
}

func (m *mockAreaLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) { return "", nil }
func (m *mockAreaLLM) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	return "", nil
}
func (m *mockAreaLLM) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return nil, nil
}
func (m *mockAreaLLM) SetRetryContext(msg string)  {}
func (m *mockAreaLLM) ClearRetryContext()           {}
func (m *mockAreaLLM) IsAvailable() bool            { return true }
func (m *mockAreaLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockAreaLLM) AuditBinaryContent(filename, content string) (bool, error) { return false, nil }
func (m *mockAreaLLM) GenerateChangelogByArea(formattedGroups string, nameMap map[string]string) (domain.ChangelogByArea, error) {
	m.called = formattedGroups
	return m.result, m.err
}
func (m *mockAreaLLM) GenerateChangelogGeneric(commits, prev, out string) (*domain.Changelog, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Changelog{}, nil
}
func (m *mockAreaLLM) RegenerateMessage(prev []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}
func (m *mockAreaLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) { return nil, nil }

func (m *mockAreaLLM) ClassifyBinary(prompt string) (string, error) {
	return "fix", nil
}

func TestGenerate_CallsGenerateChangelogByArea(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockAreaLLM{result: domain.ChangelogByArea{"security": []string{"Fixed auth bypass"}}}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	// Set areas to trigger by-area routing
	svc.projectCfg = &domain.ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth"},
		},
	}

	commits := "abc1234 fix(security): handle nil token"
	changelog, warnings, isBg, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if isBg {
		t.Error("v2 should always be sync (isBg=false)")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if llm.called == "" {
		t.Error("GenerateChangelogByArea was not called")
	}
	if !strings.Contains(changelog, "## Security") {
		t.Errorf("expected Security section in changelog, got:\n%s", changelog)
	}
}

func TestGenerate_AllInternalCommits_ReturnsEmpty(t *testing.T) {
	git := &mockGitForRelease{}
	llm :=&mockAreaLLM{}
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
	if llm.called != "" {
		t.Error("LLM should not be called when all commits are internal")
	}
	if changelog != "" || len(warnings) != 0 {
		t.Errorf("expected empty result for all-internal commits, got: %q, %v", changelog, warnings)
	}
}

func TestGenerate_LLMError_ReturnsWarning(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockAreaLLM{err: fmt.Errorf("LLM unavailable")}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	// Test generic path (nil areas) — generateGeneric should also return error
	svc.projectCfg = nil

	commits := "abc feat(core): add new feature"
	_, warnings, _, err := svc.Generate(commits)
	if err == nil {
		t.Error("expected error from LLM failure")
	}
	if len(warnings) == 0 {
		t.Error("expected warning on LLM failure")
	}
}

// --- mockGenericLLM tracks both generic and by-area paths ---

type mockGenericLLM struct {
	genericResult *domain.Changelog
	genericErr    error
	genericCalled bool

	byAreaResult domain.ChangelogByArea
	byAreaErr     error
	byAreaCalled  bool
}

func (m *mockGenericLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) { return "", nil }
func (m *mockGenericLLM) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	return "", nil
}
func (m *mockGenericLLM) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return nil, nil
}
func (m *mockGenericLLM) SetRetryContext(msg string)  {}
func (m *mockGenericLLM) ClearRetryContext()           {}
func (m *mockGenericLLM) IsAvailable() bool            { return true }
func (m *mockGenericLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockGenericLLM) AuditBinaryContent(filename, content string) (bool, error) { return false, nil }
func (m *mockGenericLLM) GenerateChangelogByArea(formattedGroups string, nameMap map[string]string) (domain.ChangelogByArea, error) {
	m.byAreaCalled = true
	if m.byAreaErr != nil {
		return nil, m.byAreaErr
	}
	return m.byAreaResult, nil
}
func (m *mockGenericLLM) GenerateChangelogGeneric(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	m.genericCalled = true
	if m.genericErr != nil {
		return nil, m.genericErr
	}
	return m.genericResult, nil
}
func (m *mockGenericLLM) RegenerateMessage(prev []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}
func (m *mockGenericLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) { return nil, nil }
func (m *mockGenericLLM) ClassifyBinary(prompt string) (string, error) {
	return "fix", nil
}

// --- filterForChangelog tests ---

func TestFilterForChangelog_ExcludesDocsAndInternal(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Excluded: []string{"docs", "scripts"},
	}
	commits := "abc1234 feat(core): add feature\ndef5678 docs: update readme\nghi9012 chore(scripts): update build\njkl3456 fix(core): fix bug"
	groups := filterForChangelog(commits, cfg)
	// core should have both feat and fix
	if len(groups["core"]) != 2 {
		t.Errorf("core area: want 2 commits, got %d", len(groups["core"]))
	}
	// chore(scripts) has type "chore" (skipType) so filtered by skipTypes
	if _, ok := groups["scripts"]; ok {
		t.Error("scripts scope should be excluded (skipType chore)")
	}
	// docs: update readme has no scope and type "docs" (not skipType) so it passes
	if len(groups[""]) != 1 {
		t.Errorf("no-scope group: want 1 commit, got %d", len(groups[""]))
	}
	// Total non-excluded items: feat(core) + fix(core) + docs(empty scope) = 3
	total := 0
	for _, items := range groups {
		total += len(items)
	}
	if total != 3 {
		t.Errorf("expected 3 user-facing commits after filtering, got %d", total)
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

// --- groupByArea tests ---

func TestGroupByArea_SortedMapping(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Areas: map[string][]string{
			"tui":    {"internal/ui"},
			"core":   {"internal/core"},
			"deploy": {"internal/deploy"},
		},
	}
	// Note: groupByArea receives pre-filtered groups from FilterAndGroupCommits
	// Keyed by conventional-commit scope (which is the area name from the area mapping)
	groups := map[string][]string{
		"core":   {"feat(core): add feature", "fix(core): fix bug"},
		"tui":    {"feat(tui): add screen"},
		"deploy": {"chore(deploy): update config"},
	}

	areaGroups, nameMap := groupByArea(groups, cfg)

	// Areas should be sorted: core=group_1, deploy=group_2, tui=group_3
	if len(areaGroups) != 3 {
		t.Errorf("expected 3 area groups, got %d", len(areaGroups))
	}
	// areaGroups keys should be group_N
	for key := range areaGroups {
		if !strings.HasPrefix(key, "group_") {
			t.Errorf("expected group_N key, got %q", key)
		}
	}
	// nameMap should map group_N back to area names
	if len(nameMap) != 3 {
		t.Errorf("expected 3 name mappings, got %d", len(nameMap))
	}
	// Verify reverse mapping: group_1 → core (alphabetically first)
	sortedAreas := []string{"core", "deploy", "tui"}
	for i, area := range sortedAreas {
		groupKey := fmt.Sprintf("group_%d", i+1)
		if nameMap[groupKey] != area {
			t.Errorf("nameMap[%q] = %q, want %q", groupKey, nameMap[groupKey], area)
		}
	}
}

func TestGroupByArea_EmptyAreas(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Areas: map[string][]string{},
	}
	groups := map[string][]string{
		"": {"feat: add feature"},
	}
	areaGroups, nameMap := groupByArea(groups, cfg)
	if len(areaGroups) != 0 {
		t.Errorf("expected 0 area groups with empty areas, got %d", len(areaGroups))
	}
	if len(nameMap) != 0 {
		t.Errorf("expected 0 name mappings with empty areas, got %d", len(nameMap))
	}
}

func TestGroupByArea_NilAreas(t *testing.T) {
	cfg := &domain.ProjectConfig{}
	groups := map[string][]string{
		"": {"feat: add feature"},
	}
	areaGroups, _ := groupByArea(groups, cfg)
	if len(areaGroups) != 0 {
		t.Errorf("expected 0 area groups with nil areas, got %d", len(areaGroups))
	}
}

// --- remapGroupKeys tests ---

func TestRemapGroupKeys_GroupToArea(t *testing.T) {
	ch := domain.ChangelogByArea{
		"group_1": []string{"add feature", "fix bug"},
		"group_2": []string{"update screen"},
	}
	nameMap := map[string]string{
		"group_1": "core",
		"group_2": "tui",
	}

	result := remapGroupKeys(ch, nameMap)

	if len(result) != 2 {
		t.Fatalf("expected 2 areas, got %d", len(result))
	}
	if len(result["core"]) != 2 {
		t.Errorf("expected 2 items in core, got %d", len(result["core"]))
	}
	if len(result["tui"]) != 1 {
		t.Errorf("expected 1 item in tui, got %d", len(result["tui"]))
	}
	if _, ok := result["group_1"]; ok {
		t.Error("group_1 key should not exist in result")
	}
}

func TestRemapGroupKeys_EmptyInput(t *testing.T) {
	ch := domain.ChangelogByArea{}
	nameMap := map[string]string{}

	result := remapGroupKeys(ch, nameMap)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d areas", len(result))
	}
}

// --- Generate routing tests ---

func TestGenerate_NilAreas_RoutesToGeneric(t *testing.T) {
	// When ProjectConfig has no areas, Generate should use the generic (by-type) path
	git := &mockGitForRelease{}
	llm := &mockGenericLLM{
		genericResult: &domain.Changelog{Features: []string{"add feature"}},
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	// Set nil project config explicitly
	svc.projectCfg = nil

	commits := "abc feat: add feature"
	changelog, warnings, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !llm.genericCalled {
		t.Error("GenerateChangelogGeneric should have been called when areas is nil")
	}
	if llm.byAreaCalled {
		t.Error("GenerateChangelogByArea should NOT have been called when areas is nil")
	}
	if !strings.Contains(changelog, "## Features") {
		t.Errorf("expected Features section in changelog, got:\n%s", changelog)
	}
}

func TestGenerate_WithAreas_RoutesToByArea(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockGenericLLM{
		byAreaResult: domain.ChangelogByArea{"core": []string{"add feature"}},
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	// Set project config with areas
	svc.projectCfg = &domain.ProjectConfig{
		Areas: map[string][]string{
			"core": {"internal/core"},
		},
	}

	commits := "abc feat(core): add feature"
	_, _, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !llm.byAreaCalled {
		t.Error("GenerateChangelogByArea should have been called when areas is configured")
	}
	if llm.genericCalled {
		t.Error("GenerateChangelogGeneric should NOT have been called when areas is configured")
	}
}

// --- nameMap routing tests ---

// mockNameMapLLM tracks that GenerateChangelogByArea receives the nameMap
type mockNameMapLLM struct {
	genericResult *domain.Changelog
	genericErr    error
	genericCalled bool

	byAreaResult domain.ChangelogByArea
	byAreaErr    error
	byAreaCalled bool
	byAreaInput  string        // captures the formatted groups input
	byAreaNameMap map[string]string // captures the nameMap parameter
}

func (m *mockNameMapLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) { return "", nil }
func (m *mockNameMapLLM) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	return "", nil
}
func (m *mockNameMapLLM) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return nil, nil
}
func (m *mockNameMapLLM) SetRetryContext(msg string)  {}
func (m *mockNameMapLLM) ClearRetryContext()           {}
func (m *mockNameMapLLM) IsAvailable() bool            { return true }
func (m *mockNameMapLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockNameMapLLM) AuditBinaryContent(filename, content string) (bool, error) { return false, nil }
func (m *mockNameMapLLM) GenerateChangelogByArea(formattedGroups string, nameMap map[string]string) (domain.ChangelogByArea, error) {
	m.byAreaCalled = true
	m.byAreaInput = formattedGroups
	m.byAreaNameMap = nameMap
	if m.byAreaErr != nil {
		return nil, m.byAreaErr
	}
	return m.byAreaResult, nil
}
func (m *mockNameMapLLM) GenerateChangelogGeneric(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	m.genericCalled = true
	if m.genericErr != nil {
		return nil, m.genericErr
	}
	return m.genericResult, nil
}
func (m *mockNameMapLLM) RegenerateMessage(prev []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}
func (m *mockNameMapLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) { return nil, nil }
func (m *mockNameMapLLM) ClassifyBinary(prompt string) (string, error) {
	return "fix", nil
}

func TestGenerate_WithAreas_PassesNameMap(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockNameMapLLM{
		byAreaResult: domain.ChangelogByArea{"core": []string{"Added feature"}},
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	svc.projectCfg = &domain.ProjectConfig{
		Areas: map[string][]string{
			"core": {"internal/core"},
		},
	}

	commits := "abc feat(core): add feature"
	_, _, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !llm.byAreaCalled {
		t.Fatal("GenerateChangelogByArea should have been called")
	}
	// Verify nameMap has the group_N → area mapping
	if len(llm.byAreaNameMap) == 0 {
		t.Error("nameMap should not be empty when areas are configured")
	}
	// With one area "core", nameMap should map group_1 → core
	if llm.byAreaNameMap["group_1"] != "core" {
		t.Errorf("nameMap[group_1] = %q, want %q", llm.byAreaNameMap["group_1"], "core")
	}
	// Verify the formatted groups input uses group_N keys, not area names
	if strings.Contains(llm.byAreaInput, "core:") && !strings.Contains(llm.byAreaInput, "group_1:") {
		t.Error("formatted groups should use group_N keys, not area names like 'core'")
	}
	if !strings.Contains(llm.byAreaInput, "group_1:") {
		t.Errorf("formatted groups should contain 'group_1:', got:\n%s", llm.byAreaInput)
	}
}

func TestGenerate_WithAreas_Remapping(t *testing.T) {
	// Test that group_N keys in the LLM response are remapped to area names
	git := &mockGitForRelease{}
	// LLM returns group_N keys — simulating real LLM behavior
	llm := &mockNameMapLLM{
		byAreaResult: domain.ChangelogByArea{
			"group_1": []string{"Added semantic diff analysis"},
			"group_2": []string{"Fixed webhook auth bypass"},
		},
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	svc.projectCfg = &domain.ProjectConfig{
		Areas: map[string][]string{
			"core":     {"internal/core"},
			"security": {"internal/auth"},
		},
	}

	commits := "abc feat(core): add feature\ndef fix(security): fix bug"
	changelog, _, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	// Verify remapping happened — should contain area names, not group_N
	if !strings.Contains(changelog, "## Core") && !strings.Contains(changelog, "## Security") {
		t.Errorf("changelog should contain area section headers (Core, Security), got:\n%s", changelog)
	}
	if strings.Contains(changelog, "group_1") || strings.Contains(changelog, "group_2") {
		t.Errorf("group_N keys should be remapped to area names, got:\n%s", changelog)
	}
}

func TestGenerate_GenericPath_DoesNotCallByArea(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockNameMapLLM{
		genericResult: &domain.Changelog{Features: []string{"Added feature"}, Fixes: []string{}},
	}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil, nil)
	// nil project config → generic path
	svc.projectCfg = nil

	commits := "abc feat(core): add feature"
	_, _, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !llm.genericCalled {
		t.Error("GenerateChangelogGeneric should be called when no areas configured")
	}
	if llm.byAreaCalled {
		t.Error("GenerateChangelogByArea should NOT be called when no areas configured")
	}
}

// --- Chunking support tests ---

func TestEstimateTokens_BasicEstimate(t *testing.T) {
	// Rough estimate: ~4 chars per token
	input := "group_1:\n- feat(core): add feature\n- fix(core): fix bug\n"
	estimate := estimateTokens(input)
	if estimate <= 0 {
		t.Errorf("estimateTokens should return positive value, got %d", estimate)
	}
	// The estimate should be roughly len(input)/4
	roughEstimate := len(input) / 4
	// Allow 50% tolerance since token estimation is approximate
	if estimate < roughEstimate/2 || estimate > roughEstimate*2 {
		t.Errorf("estimateTokens(%q) = %d, expected roughly %d (within 2x)", input, estimate, roughEstimate)
	}
}

func TestShouldChunkChangelog_FitsInOneCall(t *testing.T) {
	// Small groups should fit in one call
	groups := map[string][]string{
		"group_1": {"feat(core): add feature", "fix(core): fix bug"},
	}
	if shouldChunkChangelog(groups, 4096) {
		t.Error("small groups should fit in one call, should not need chunking")
	}
}

func TestShouldChunkChangelog_ExceedsThreshold(t *testing.T) {
	// Large groups should exceed threshold
	groups := map[string][]string{
		"group_1": make([]string, 100), // 100 items
	}
	for i := range groups["group_1"] {
		groups["group_1"][i] = "feat: this is a very long commit message that takes up tokens"
	}
	if !shouldChunkChangelog(groups, 100) {
		t.Error("large groups with low threshold should need chunking")
	}
}
