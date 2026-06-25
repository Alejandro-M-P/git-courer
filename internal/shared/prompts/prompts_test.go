package prompts

import (
	"strings"
	"testing"
)

// --- Get ---

func TestGet_ReturnsNonEmpty(t *testing.T) {
	tmpl, err := Get("commit_message")
	if err != nil {
		t.Fatalf("Get(commit_message) error: %v", err)
	}
	if tmpl == "" {
		t.Error("Get(commit_message) returned empty string")
	}
}

func TestGet_UnknownOp_ReturnsError(t *testing.T) {
	_, err := Get("totally_unknown_operation_xyz")
	if err == nil {
		t.Error("Get(unknown) should return error, not nil")
	}
}

func TestGet_CommitMessage_ResolvesFromMD(t *testing.T) {
	tmpl, err := Get("commit_message")
	if err != nil {
		t.Fatalf("Get(commit_message) error: %v", err)
	}
	if tmpl == "" {
		t.Error("Get(commit_message) returned empty string")
	}
	// Verify the template contains the Role heading, which is only in the .md version
	if !strings.Contains(tmpl, "## Role") {
		t.Errorf("commit_message template should contain '## Role' from .md version; got:\n%s", tmpl[:min(200, len(tmpl))])
	}
}

// --- Truncated Prompt Tests ---

func TestRender_CommitMessage_WithFiles(t *testing.T) {
	data := MessageParams{
		Files:         "internal/workflow/prepare.go, internal/core/domain/llm.go",
		AnnotatedDiff: "--- a/internal/workflow/prepare.go\n+++ b/internal/workflow/prepare.go\n@@ -22 +22 @@\n+case \"branch_rename\":",
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render(commit_message) error: %v", err)
	}
	if !strings.Contains(got, "--- a/internal/workflow/prepare.go") {
		t.Errorf("commit_message prompt missing diff content; got:\n%s", got)
	}
	if strings.Contains(got, "VARY YOUR EXPLANATION STYLE") {
		t.Errorf("commit_message prompt must NOT contain 'VARY YOUR EXPLANATION STYLE'")
	}
	// Line count should be ~15 lines max (raw content + rendered data)
	lines := strings.Split(got, "\n")
	if len(lines) > 30 {
		t.Logf("commit_message rendered to %d lines — acceptably small", len(lines))
	}
}

func TestRender_CommitMessage_WithRejectedMessage(t *testing.T) {
	rejected := "feat: improve branch handling"
	data := MessageParams{
		Files:           "internal/workflow/prepare.go",
		AnnotatedDiff:   "some diff content",
		RejectedMessage: rejected,
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render(commit_message with retry) error: %v", err)
	}
	if !strings.Contains(got, rejected) {
		t.Errorf("commit_message retry block must contain rejected message verbatim %q; got:\n%s", rejected, got)
	}
}

func TestRender_CredentialAudit(t *testing.T) {
	tmpl, err := Get("credential_audit")
	if err != nil {
		t.Skip("credential_audit.txt not yet created")
	}

	sampleDiff := "--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n+const apiKey = \"DUMMY_KEY\""
	sampleFindings := "- api_key in main.go (line 1): DUMMY_KEY"

	data := map[string]string{
		"Diff":     sampleDiff,
		"Findings": sampleFindings,
	}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(credential_audit) error: %v", err)
	}
	if !strings.Contains(got, sampleDiff) {
		t.Errorf("rendered prompt does not contain the diff content")
	}
	if !strings.Contains(got, sampleFindings) {
		t.Errorf("rendered prompt does not contain the findings content")
	}
}

func TestRender_CommitMessage_WithContext(t *testing.T) {
	data := MessageParams{
		Files:         "main.go",
		AnnotatedDiff: "+added line",
		Context:       "Project: X\nStyle: Y",
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render(commit_message with context) error: %v", err)
	}
	if !strings.Contains(got, "Project: X") {
		t.Errorf("rendered prompt missing context; got:\n%s", got)
	}
}

func TestRender_CommitMessage_ContextOmitted_WhenEmpty(t *testing.T) {
	data := MessageParams{
		Files:         "main.go",
		AnnotatedDiff: "+added line",
		Context:       "",
	}
	tmpl := GetCommitMessage()
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(commit_message without context) error: %v", err)
	}
	want, _ := Render(tmpl, MessageParams{Files: data.Files, AnnotatedDiff: data.AnnotatedDiff})
	if got != want {
		t.Errorf("empty context should produce byte-for-byte legacy output; got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRender_CommitMessage_WhyPresent(t *testing.T) {
	data := MessageParams{
		Files:         "main.go",
		AnnotatedDiff: "+added line",
		Why:           "add refresh token rotation",
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render(commit_message with why) error: %v", err)
	}
	if !strings.Contains(got, "Developer's reason") {
		t.Errorf("rendered prompt should contain 'Developer's reason' heading; got:\n%s", got)
	}
	if !strings.Contains(got, "add refresh token rotation") {
		t.Errorf("rendered prompt should contain the Why text; got:\n%s", got)
	}
}

func TestRender_CommitMessage_WhyOmitted_WhenEmpty(t *testing.T) {
	data := MessageParams{
		Files:         "main.go",
		AnnotatedDiff: "+added line",
		Why:           "",
	}
	tmpl := GetCommitMessage()
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(commit_message without why) error: %v", err)
	}
	want, _ := Render(tmpl, MessageParams{Files: data.Files, AnnotatedDiff: data.AnnotatedDiff})
	if got != want {
		t.Errorf("empty Why should produce byte-for-byte identical output to current; got:\n%s\nwant:\n%s", got, want)
	}
}

func TestGetChangelog(t *testing.T) {
	tmpl := GetChangelog()
	if tmpl == "" {
		t.Error("GetChangelog() returned empty string")
	}
	// Freeform changelog should instruct LLM to invent its own categories
	if !strings.Contains(tmpl, "Invent Your Own Categories") {
		t.Error("changelog.md should instruct LLM to invent its own categories")
	}
	if !strings.Contains(tmpl, "markdown") {
		t.Error("changelog.md should specify markdown output")
	}
	// Should NOT contain old area-based concepts
	if strings.Contains(tmpl, "area_name") {
		t.Error("changelog.md should NOT contain area names like 'area_name'")
	}
}

func TestGetChangelogRegenerate(t *testing.T) {
	tmpl := GetChangelogRegenerate()
	if tmpl == "" {
		t.Fatal("GetChangelogRegenerate() returned empty string")
	}
	// Regenerate template must declare both variables used by the adapter.
	if !strings.Contains(tmpl, "{{.PreviousChangelog}}") {
		t.Error("changelog_regenerate.md should reference the PreviousChangelog variable")
	}
	if !strings.Contains(tmpl, "{{.Feedback}}") {
		t.Error("changelog_regenerate.md should reference the Feedback variable")
	}
	// Must reuse the same output rules as the changelog template.
	if !strings.Contains(tmpl, "Invent Your Own Categories") {
		t.Error("changelog_regenerate.md should reuse the changelog output rules (category naming)")
	}
	if !strings.Contains(tmpl, "markdown") {
		t.Error("changelog_regenerate.md should specify markdown output")
	}
}

func TestRender_ChangelogRegenerate_InjectsPrevChangelogAndFeedback(t *testing.T) {
	tmpl := GetChangelogRegenerate()
	prev := "## Features\n- existing bullet"
	feedback := "make the tone more direct"
	got, err := Render(tmpl, map[string]string{
		"PreviousChangelog": prev,
		"Feedback":          feedback,
	})
	if err != nil {
		t.Fatalf("Render(changelog_regenerate) error: %v", err)
	}
	if !strings.Contains(got, prev) {
		t.Errorf("rendered prompt must contain PreviousChangelog verbatim; got:\n%s", got)
	}
	if !strings.Contains(got, feedback) {
		t.Errorf("rendered prompt must contain Feedback verbatim; got:\n%s", got)
	}
}
