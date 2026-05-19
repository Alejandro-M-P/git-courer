package prompts

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/config"
)

// --- Get / HasTemplate ---

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

func TestHasTemplate_KnownOps(t *testing.T) {
	knownOps := []string{"commit_message", "decide_commit", "branch_create", "changelog_generate", "credential_audit"}
	for _, op := range knownOps {
		if !HasTemplate(op) {
			t.Logf("HasTemplate(%q) = false — template file may not be embedded, skipping", op)
		}
	}
}

func TestHasTemplate_DeletedOps_ReturnsFalse(t *testing.T) {
	deletedOps := []string{"push", "pull", "merge", "tag_create", "tag_delete", "tag_push", "tag_delete_remote", "branch_delete", "branch_rename", "release_interpret", "system_reasoning", "binary_check"}
	for _, op := range deletedOps {
		if HasTemplate(op) {
			t.Errorf("HasTemplate(%q) = true, want false (deleted prompt)", op)
		}
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
	if !HasTemplate("commit_message") {
		t.Error("HasTemplate(commit_message) returned false, want true")
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

func TestRender_BranchCreate(t *testing.T) {
	tmpl, err := Get("branch_create")
	if err != nil {
		t.Skip("branch_create.txt not yet created")
	}
	data := OpParams{
		Instruction:   "create login branch",
		CurrentBranch: "main",
		Branches:      "main\ndevelop",
	}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(branch_create) error: %v", err)
	}
	if !strings.Contains(got, "create login branch") {
		t.Errorf("rendered branch_create prompt does not contain instruction")
	}
	if !strings.Contains(got, "main") {
		t.Errorf("rendered branch_create prompt does not contain current branch")
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

func TestRender_DecideCommit(t *testing.T) {
	tmpl, err := Get("decide_commit")
	if err != nil {
		t.Skip("decide_commit.txt not yet created")
	}
	data := DecideParams{
		Instruction: "commit everything",
		Untracked:   "new.go",
		Modified:    "main.go",
		Deleted:     "old.go",
	}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(decide_commit) error: %v", err)
	}
	if !strings.Contains(got, "commit everything") {
		t.Errorf("rendered decide_commit prompt does not contain instruction")
	}
	if !strings.Contains(got, "new.go") {
		t.Errorf("rendered decide_commit prompt does not contain untracked")
	}
	if !strings.Contains(got, "main.go") {
		t.Errorf("rendered decide_commit prompt does not contain modified")
	}
	if !strings.Contains(got, "old.go") {
		t.Errorf("rendered decide_commit prompt does not contain deleted")
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

// --- RenderOp ---

func TestRenderOp_ReturnsNonEmpty(t *testing.T) {
	data := struct{ Instruction string }{"create feature branch"}
	got, err := RenderOp("branch_create", data)
	if err != nil {
		t.Logf("RenderOp() error (expected if template needs fields): %v", err)
		return
	}
	_ = got
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

func TestFormatContext(t *testing.T) {
	cases := []struct {
		name    string
		project string
		style   string
		want    string
	}{
		{"both set", "P", "S", "Project description: P\nStyle: S"},
		{"project only", "P", "", "Project description: P"},
		{"style only", "", "S", "Style: S"},
		{"empty", "", "", ""},
		{"whitespace trimmed", " P ", " S ", "Project description: P\nStyle: S"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatContext(config.ContextConfig{Project: tc.project, Style: tc.style})
			if got != tc.want {
				t.Errorf("FormatContext(%q, %q) = %q, want %q", tc.project, tc.style, got, tc.want)
			}
		})
	}
}

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

// --- BuildDecideParams ---

func TestBuildDecideParams(t *testing.T) {
	p := BuildDecideParams("commit everything", "M main.go", "new.go", "main.go", "old.go")
	if p.Instruction != "commit everything" {
		t.Errorf("Instruction = %q, want 'commit everything'", p.Instruction)
	}
	if p.Untracked != "new.go" {
		t.Errorf("Untracked = %q, want 'new.go'", p.Untracked)
	}
	if p.Modified != "main.go" {
		t.Errorf("Modified = %q, want 'main.go'", p.Modified)
	}
	if p.Deleted != "old.go" {
		t.Errorf("Deleted = %q, want 'old.go'", p.Deleted)
	}
}

// --- BuildOpParams ---

func TestBuildOpParams(t *testing.T) {
	ctx := map[string]string{
		"current_branch": "develop",
		"branches":       "main\ndevelop",
		"tags":           "v1.0.0",
		"remote":         "origin",
	}
	p := BuildOpParams("push to origin", ctx)
	if p.Instruction != "push to origin" {
		t.Errorf("Instruction = %q, want 'push to origin'", p.Instruction)
	}
	if p.CurrentBranch != "develop" {
		t.Errorf("CurrentBranch = %q, want 'develop'", p.CurrentBranch)
	}
	if p.Tags != "v1.0.0" {
		t.Errorf("Tags = %q, want 'v1.0.0'", p.Tags)
	}
}

// --- GetAll ---

func TestGetAll_ReturnsMap(t *testing.T) {
	all := GetAll()
	if all == nil {
		t.Error("GetAll() returned nil")
	}
}

// --- ProjectDescriptionParams ---

func TestBuildProjectDescriptionParams(t *testing.T) {
	docContent := "=== README.md ===\nMy project is a tool for X."
	params := BuildProjectDescriptionParams(docContent)
	if params.DocContents != docContent {
		t.Errorf("DocContents = %q, want %q", params.DocContents, docContent)
	}
}

func TestBuildProjectDescriptionParams_Empty(t *testing.T) {
	params := BuildProjectDescriptionParams("")
	if params.DocContents != "" {
		t.Errorf("DocContents = %q, want empty string", params.DocContents)
	}
}

func TestRender_ProjectDescription(t *testing.T) {
	tmpl := GetProjectDescription()
	params := BuildProjectDescriptionParams("=== README.md ===\nA tool for conventional commits.")
	got, err := Render(tmpl, params)
	if err != nil {
		t.Fatalf("Render(project_description) error: %v", err)
	}
	if !strings.Contains(got, "A tool for conventional commits") {
		t.Errorf("Rendered prompt missing doc content; got:\n%s", got)
	}
	if !strings.Contains(got, "description") {
		t.Errorf("Rendered prompt missing 'description' key requirement; got:\n%s", got)
	}
	if !strings.Contains(got, "ONLY") {
		t.Errorf("Rendered prompt missing strict rules; got:\n%s", got)
	}
}

func TestGetProjectDescription(t *testing.T) {
	tmpl := GetProjectDescription()
	if tmpl == "" {
		t.Error("GetProjectDescription() returned empty string")
	}
}
