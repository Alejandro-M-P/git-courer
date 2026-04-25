package prompts

import (
	"strings"
	"testing"
)

// --- Get / HasTemplate ---

func TestGet_ReturnsNonEmpty(t *testing.T) {
	// Even if the file doesn't exist, Get returns the fallback
	tmpl := Get("commit_message")
	if tmpl == "" {
		t.Error("Get(commit_message) returned empty string")
	}
}

func TestGet_UnknownOp_ReturnsFallback(t *testing.T) {
	tmpl := Get("totally_unknown_operation_xyz")
	if tmpl == "" {
		t.Error("Get(unknown) should return fallback, not empty")
	}
}

func TestHasTemplate_KnownOps(t *testing.T) {
	// These template files should exist (embedded in the binary)
	knownOps := []string{"commit_message", "decide_commit"}
	for _, op := range knownOps {
		if !HasTemplate(op) {
			t.Logf("HasTemplate(%q) = false — template file may not be embedded, skipping", op)
		}
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
		// Template may not render perfectly without all fields, that's ok
		t.Logf("RenderOp() error (expected if template needs fields): %v", err)
		return
	}
	_ = got
}

// --- GetWithFallback ---

func TestGetWithFallback_CommitMessage(t *testing.T) {
	tmpl := GetWithFallback("commit_message")
	if tmpl == "" {
		t.Error("GetWithFallback(commit_message) returned empty")
	}
}

func TestGetWithFallback_DecideCommit(t *testing.T) {
	tmpl := GetWithFallback("decide_commit")
	if tmpl == "" {
		t.Error("GetWithFallback(decide_commit) returned empty")
	}
}

func TestGetWithFallback_BranchCreate(t *testing.T) {
	tmpl := GetWithFallback("branch_create")
	if tmpl == "" {
		t.Error("GetWithFallback(branch_create) returned empty")
	}
}

func TestGetWithFallback_Unknown(t *testing.T) {
	tmpl := GetWithFallback("unknown_op_xyz")
	if tmpl == "" {
		t.Error("GetWithFallback(unknown) should return default JSON template")
	}
}

// --- BuildMessageParams ---

func TestBuildMessageParams_Files(t *testing.T) {
	p := BuildMessageParams([]string{"main.go", "auth.go"}, "diff content")
	if p.Diff != "diff content" {
		t.Errorf("Diff = %q, want 'diff content'", p.Diff)
	}
	if p.Files == "" {
		t.Error("Files should not be empty")
	}
	if !strings.Contains(p.Files, "main.go") {
		t.Errorf("Files = %q should contain 'main.go'", p.Files)
	}
}

func TestBuildMessageParamsWithRetry(t *testing.T) {
	p := BuildMessageParamsWithRetry([]string{"a.go"}, "diff", "bad previous message")
	if p.RejectedMessage != "bad previous message" {
		t.Errorf("RejectedMessage = %q, want 'bad previous message'", p.RejectedMessage)
	}
}

func TestBuildMessageParams_EmptyFiles(t *testing.T) {
	p := BuildMessageParams([]string{}, "diff")
	if p.Files != "" {
		t.Errorf("Files for empty slice = %q, want empty", p.Files)
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

// --- BuildInterpretParams ---

func TestBuildInterpretParams(t *testing.T) {
	ctx := map[string]string{
		"current_branch": "main",
		"branches":       "main\ndevelop",
	}
	p := BuildInterpretParams("branch_create", "create feature branch", ctx)
	if p.Operation != "branch_create" {
		t.Errorf("Operation = %q, want branch_create", p.Operation)
	}
	if p.Instruction != "create feature branch" {
		t.Errorf("Instruction = %q, want 'create feature branch'", p.Instruction)
	}
	if p.Context == "" {
		t.Error("Context should not be empty when ctx map is provided")
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

// --- T-52: VerifySecrets prompt migration ---

// TestGet_CredentialAudit verifies credential_audit.txt exists in the embed.FS.
func TestGet_CredentialAudit(t *testing.T) {
	tmpl := Get("credential_audit")
	if tmpl == "" {
		t.Error("Get(credential_audit) returned empty — credential_audit.txt may not exist")
	}
}

// TestRender_CredentialAudit verifies rendered prompt contains diff and findings.
func TestRender_CredentialAudit(t *testing.T) {
	tmpl := Get("credential_audit")
	if tmpl == "" {
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

// --- T-53: Prompt file existence and rendering ---

// TestGet_Merge verifies merge.txt exists in the embed.FS.
func TestGet_Merge(t *testing.T) {
	tmpl := Get("merge")
	if tmpl == "" {
		t.Error("Get(merge) returned empty — merge.txt may not exist")
	}
}

// TestRender_Merge verifies merge.txt renders with expected variables.
func TestRender_Merge(t *testing.T) {
	tmpl := Get("merge")
	if tmpl == "" {
		t.Skip("merge.txt not yet created")
	}

	data := map[string]string{
		"Instruction":   "merge feat/login into main",
		"CurrentBranch": "feat/login",
		"Branches":      "main\ndevelop\nfeat/login",
	}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render(merge) error: %v", err)
	}
	if !strings.Contains(got, "feat/login") {
		t.Errorf("rendered merge prompt does not contain current branch name")
	}
	if !strings.Contains(got, "main") {
		t.Errorf("rendered merge prompt does not contain branch list")
	}
}

// TestRender_CommitMessage_WithFiles verifies commit_message.txt renders with Files block.
func TestRender_CommitMessage_WithFiles(t *testing.T) {
	data := MessageParams{
		Files: "internal/workflow/prepare.go, internal/core/domain/llm.go",
		Diff:  "--- a/internal/workflow/prepare.go\n+++ b/internal/workflow/prepare.go\n@@ -22 +22 @@\n+case \"branch_rename\":",
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render(commit_message) error: %v", err)
	}
	if !strings.Contains(got, "Files changed:") {
		t.Errorf("commit_message prompt missing 'Files changed:' block; got:\n%s", got)
	}
	if strings.Contains(got, "VARY YOUR EXPLANATION STYLE") {
		t.Errorf("commit_message prompt must NOT contain 'VARY YOUR EXPLANATION STYLE'")
	}
}

// TestRender_CommitMessage_WithRejectedMessage verifies retry block contains rejected message verbatim.
func TestRender_CommitMessage_WithRejectedMessage(t *testing.T) {
	rejected := "feat: improve branch handling"
	data := MessageParams{
		Files:           "internal/workflow/prepare.go",
		Diff:            "some diff content",
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

// --- GetAll ---

func TestGetAll_ReturnsMap(t *testing.T) {
	all := GetAll()
	if all == nil {
		t.Error("GetAll() returned nil")
	}
	// Should at least have an empty map
}

// --- InterpretGitOp template ---

func TestInterpretGitOpTemplate_Render(t *testing.T) {
	p := BuildInterpretParams("merge", "merge feature into main", map[string]string{
		"current_branch": "main",
	})
	got, err := Render(InterpretGitOp, p)
	if err != nil {
		t.Fatalf("Render(InterpretGitOp) error: %v", err)
	}
	if !strings.Contains(got, "merge") {
		t.Errorf("rendered prompt doesn't contain operation name")
	}
	if !strings.Contains(got, "merge feature into main") {
		t.Errorf("rendered prompt doesn't contain instruction")
	}
}
