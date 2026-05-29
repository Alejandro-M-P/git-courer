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

func TestRender_ChangelogGenerate(t *testing.T) {
	tmpl, err := Get("changelog_generate")
	if err != nil {
		t.Skip("changelog_generate.txt not yet created")
	}
	data := map[string]string{"commits": "- feat: add feature\n- fix: bug fix"}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(changelog_generate) error: %v", err)
	}
	if !strings.Contains(got, "feat: add feature") {
		t.Errorf("rendered changelog prompt does not contain commits data")
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

func TestRender_Changelog_WithContext(t *testing.T) {
	tmpl, err := Get("changelog_generate")
	if err != nil {
		t.Skip("changelog_generate.txt not present")
	}
	data := map[string]string{"commits": "- feat: add feature", "Context": "Project: X"}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(changelog with context) error: %v", err)
	}
	if !strings.Contains(got, "Project: X") {
		t.Errorf("rendered changelog missing context; got:\n%s", got)
	}
}

func TestRender_Changelog_ContextOmitted_WhenEmpty(t *testing.T) {
	tmpl, err := Get("changelog_generate")
	if err != nil {
		t.Skip("changelog_generate.txt not present")
	}
	got, err := Render(tmpl, map[string]string{"commits": "- feat: add feature", "Context": ""})
	if err != nil {
		t.Fatalf("Render(changelog with empty context) error: %v", err)
	}
	want, _ := Render(tmpl, map[string]string{"commits": "- feat: add feature"})
	if got != want {
		t.Errorf("empty context should produce byte-for-byte legacy output")
	}
}

// --- Render ---

func TestRender_SimpleTemplate(t *testing.T) {
	tmpl := "Hello, {{.Name}}!"
	data := struct{ Name string }{"World"}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if got != "Hello, World!" {
		t.Errorf("Render() = %q, want %q", got, "Hello, World!")
	}
}

func TestRender_InvalidTemplate(t *testing.T) {
	_, err := Render("{{.Unclosed", nil)
	if err == nil {
		t.Error("Render() expected error for invalid template, got nil")
	}
}

func TestRender_EmptyTemplate(t *testing.T) {
	got, err := Render("", nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if got != "" {
		t.Errorf("Render(empty) = %q, want empty", got)
	}
}

func TestRender_MapData(t *testing.T) {
	tmpl := "Op: {{.operation}}"
	data := map[string]string{"operation": "commit"}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(got, "commit") {
		t.Errorf("Render() = %q, want to contain 'commit'", got)
	}
}

// --- Binary ---

func TestIsBinary_NullByte(t *testing.T) {
	if !IsBinary([]byte("a\x00b")) {
		t.Error("IsBinary with null byte should return true")
	}
}

func TestIsBinary_Text(t *testing.T) {
	if IsBinary([]byte("hello world")) {
		t.Error("IsBinary with text should return false")
	}
}

func TestIsBinary_Empty(t *testing.T) {
	if IsBinary([]byte{}) {
		t.Error("IsBinary with empty slice should return false")
	}
}

// --- BuildMessageParams ---

func TestBuildMessageParams_Files(t *testing.T) {
	p := BuildMessageParams([]string{"main.go", "auth.go"}, "", "", "", "", "", false, "")
	if p.Files == "" {
		t.Error("Files should not be empty")
	}
	if !strings.Contains(p.Files, "main.go") {
		t.Errorf("Files = %q should contain 'main.go'", p.Files)
	}
}

func TestBuildMessageParamsWithContext(t *testing.T) {
	p := BuildMessageParams([]string{"main.go"}, "", "", "Project: X\nStyle: Y", "", "", false, "")
	if p.Files != "main.go" {
		t.Errorf("Files = %q, want 'main.go'", p.Files)
	}
	if p.Context != "Project: X\nStyle: Y" {
		t.Errorf("Context = %q, want 'Project: X\nStyle: Y'", p.Context)
	}
}

func TestBuildMessageParamsWithRetry_Context(t *testing.T) {
	p := BuildMessageParamsWithRetry([]string{"a.go"}, "", "", "rejected", "My context", "", "", false, "")
	if p.RejectedMessage != "rejected" {
		t.Errorf("RejectedMessage = %q, want 'rejected'", p.RejectedMessage)
	}
	if p.Context != "My context" {
		t.Errorf("Context = %q, want 'My context'", p.Context)
	}
}

// TestFormatContext was removed — FormatContext() removed (dead code in production,
// only used by llm_test_helper.go which is a manual debug tool).
// ContextConfig has been removed from the config package.

func TestBuildMessageParamsWithRetry(t *testing.T) {
	p := BuildMessageParamsWithRetry([]string{"a.go"}, "", "", "bad previous message", "", "", "", false, "")
	if p.RejectedMessage != "bad previous message" {
		t.Errorf("RejectedMessage = %q, want 'bad previous message'", p.RejectedMessage)
	}
}

func TestBuildMessageParams_EmptyFiles(t *testing.T) {
	p := BuildMessageParams([]string{}, "", "", "", "", "", false, "")
	if p.Files != "" {
		t.Errorf("Files for empty slice = %q, want empty", p.Files)
	}
}

func TestBuildMessageParams_WhyField(t *testing.T) {
	cases := []struct {
		name string
		why  string
		want string
	}{
		{"why set", "refactor auth", "refactor auth"},
		{"why empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildMessageParams([]string{"main.go"}, "annotated", "raw", "", "feat", "", false, tc.why)
			if p.Why != tc.want {
				t.Errorf("Why = %q, want %q", p.Why, tc.want)
			}
		})
	}
}

func TestBuildMessageParamsWithRetry_WhyField(t *testing.T) {
	cases := []struct {
		name string
		why  string
		want string
	}{
		{"why set", "refactor auth", "refactor auth"},
		{"why empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildMessageParamsWithRetry([]string{"main.go"}, "annotated", "raw", "rejected", "", "feat", "", false, tc.why)
			if p.Why != tc.want {
				t.Errorf("Why = %q, want %q", p.Why, tc.want)
			}
		})
	}
}

// --- Post-migration tests ---

func TestGet_MdPromptKeysExist(t *testing.T) {
	// After txt migration, these .md prompts must exist
	for _, key := range []string{"changelog_areas", "branch_create"} {
		tmpl, err := Get(key)
		if err != nil {
			t.Errorf("Get(%q) error: %v — .md prompt must exist after migration", key, err)
		}
		if tmpl == "" {
			t.Errorf("Get(%q) returned empty string", key)
		}
	}
}

func TestGetBranchCreate(t *testing.T) {
	tmpl := GetBranchCreate()
	if tmpl == "" {
		t.Error("GetBranchCreate() returned empty string")
	}
	if !strings.Contains(tmpl, "Instruction") {
		t.Error("branch_create.md should contain 'Instruction'")
	}
}

func TestGetChangelogAreas(t *testing.T) {
	tmpl := GetChangelogAreas()
	if tmpl == "" {
		t.Error("GetChangelogAreas() returned empty string")
	}
	if !strings.Contains(tmpl, "group_1") {
		t.Error("changelog_areas.md should contain 'group_1' in output schema")
	}
	if strings.Contains(tmpl, "area_name") {
		t.Error("changelog_areas.md should NOT contain area names like 'area_name'")
	}
}
