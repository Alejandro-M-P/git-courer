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
func (m *mockAreaLLM) DecideCommit(instruction, status, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return domain.CommitIntent{}, nil
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
func (m *mockAreaLLM) GenerateChangelog(commits, prev, out string) (*domain.Changelog, error) {
	return &domain.Changelog{}, nil
}
func (m *mockAreaLLM) GenerateChangelogByArea(formattedGroups string) (domain.ChangelogByArea, error) {
	m.called = formattedGroups
	return m.result, m.err
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
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

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
	llm := &mockAreaLLM{}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(t.TempDir(), "release.log"))
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

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
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "abc feat(core): add new feature"
	_, warnings, _, err := svc.Generate(commits)
	if err == nil {
		t.Error("expected error from LLM failure")
	}
	if len(warnings) == 0 {
		t.Error("expected warning on LLM failure")
	}
}
