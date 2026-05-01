//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestBranchCreateExplicit creates a branch with explicit args (no LLM interpretation).
func TestBranchCreateExplicit(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf := makeDirectWorkflow(t, gitA, llmA)

	ctx := context.Background()
	result, err := wf.Run(ctx, "branch_create", "create feat/auth branch",
		map[string]string{"branch": "feat/auth"})
	if err != nil {
		t.Fatalf("Run branch_create: %v", err)
	}

	detail = result.Status
	t.Logf("status=%s output=%s", result.Status, result.Output)

	branches := gitBranches(dir)
	if !contains(branches, "feat/auth") {
		t.Errorf("expected feat/auth in branches, got: %v", branches)
	}
}

// TestBranchCreateViaLLM lets the LLM interpret the instruction and choose the branch name.
func TestBranchCreateViaLLM(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf := makeDirectWorkflow(t, gitA, llmA)

	ctx := context.Background()
	result, err := wf.Run(ctx, "branch_create", "create a branch for the user authentication feature", nil)
	if err != nil {
		t.Fatalf("Run branch_create via LLM: %v", err)
	}

	detail = result.Status
	t.Logf("status=%s output=%s preview=%s", result.Status, result.Output, result.Preview)

	branches := gitBranches(dir)
	t.Logf("branches after LLM create: %v", branches)
	if len(branches) < 2 {
		t.Errorf("expected ≥2 branches (main + new), got: %v", branches)
	}
}

// TestBranchCreateWithApprovalFlow tests the full Start→review→Apply path.
func TestBranchCreateWithApprovalFlow(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf, _ := makeWorkflow(t, gitA, llmA)

	ctx := context.Background()
	startResult, err := wf.Run(ctx, "branch_create", "create feat/approval-test branch",
		map[string]string{"branch": "feat/approval-test"})
	if err != nil {
		t.Fatalf("Run START: %v", err)
	}
	if startResult.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %s", startResult.Status)
	}

	detail = "start→apply"
	t.Logf("START: status=%s preview=%s", startResult.Status, startResult.Preview)

	applyResult, err := wf.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	t.Logf("APPLY: status=%s output=%s", applyResult.Status, applyResult.Output)

	branches := gitBranches(dir)
	if !contains(branches, "feat/approval-test") {
		t.Errorf("expected feat/approval-test after Apply, got: %v", branches)
	}
}

// TestBranchCreateAbort verifies that aborting after START does NOT create the branch.
func TestBranchCreateAbort(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf, _ := makeWorkflow(t, gitA, llmA)

	ctx := context.Background()
	branchesBefore := gitBranches(dir)

	startResult, err := wf.Run(ctx, "branch_create", "create feat/abort-test branch",
		map[string]string{"branch": "feat/abort-test"})
	if err != nil {
		t.Fatalf("Run START: %v", err)
	}
	if startResult.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %s", startResult.Status)
	}

	if err := wf.Abort(); err != nil {
		t.Fatalf("Abort: %v", err)
	}

	detail = "aborted"
	branchesAfter := gitBranches(dir)
	t.Logf("branches before=%v after=%v", branchesBefore, branchesAfter)

	if contains(branchesAfter, "feat/abort-test") {
		t.Error("feat/abort-test should NOT exist after Abort")
	}
}

// TestBranchDelete creates a branch then deletes it via workflow.
func TestBranchDelete(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	gitExec(t, dir, "checkout", "-b", "feat/to-delete")
	gitExec(t, dir, "checkout", "-")

	if !contains(gitBranches(dir), "feat/to-delete") {
		t.Fatalf("setup failed: branch not created")
	}

	wf := makeDirectWorkflow(t, gitA, llmA)
	ctx := context.Background()
	result, err := wf.Run(ctx, "branch_delete", "delete the feat/to-delete branch",
		map[string]string{"branch": "feat/to-delete"})
	if err != nil {
		t.Fatalf("Run branch_delete: %v", err)
	}

	detail = result.Status
	t.Logf("delete: status=%s output=%s", result.Status, result.Output)

	if contains(gitBranches(dir), "feat/to-delete") {
		t.Errorf("feat/to-delete should be gone, got: %v", gitBranches(dir))
	}
}

// TestBranchRename creates a branch, renames it, verifies the new name exists.
func TestBranchRename(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	gitExec(t, dir, "checkout", "-b", "old-name")
	gitExec(t, dir, "checkout", "-")

	wf := makeDirectWorkflow(t, gitA, llmA)
	ctx := context.Background()
	result, err := wf.Run(ctx, "branch_rename", "rename old-name to new-name",
		map[string]string{"name": "old-name", "new_name": "new-name"})
	if err != nil {
		t.Fatalf("Run branch_rename: %v", err)
	}

	detail = result.Status
	branches := gitBranches(dir)
	t.Logf("rename: status=%s branches=%v", result.Status, branches)

	if contains(branches, "old-name") {
		t.Error("old-name should be gone after rename")
	}
	if !contains(branches, "new-name") {
		t.Errorf("new-name should exist after rename, got: %v", branches)
	}
}

// TestTagCreate creates a tag via workflow and verifies it exists in git.
func TestTagCreate(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf := makeDirectWorkflow(t, gitA, llmA)

	ctx := context.Background()
	result, err := wf.Run(ctx, "tag_create", "create release tag v1.0.0",
		map[string]string{"tag": "v1.0.0"})
	if err != nil {
		t.Fatalf("Run tag_create: %v", err)
	}

	detail = result.Status
	t.Logf("tag_create: status=%s output=%s", result.Status, result.Output)

	if !contains(gitTags(dir), "v1.0.0") {
		t.Errorf("expected v1.0.0 in tags, got: %v", gitTags(dir))
	}
}

// TestTagDelete creates a tag and then deletes it via workflow.
func TestTagDelete(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)

	gitExec(t, dir, "tag", "v0.9.0")
	if !contains(gitTags(dir), "v0.9.0") {
		t.Fatalf("setup failed: tag v0.9.0 not created")
	}

	wf := makeDirectWorkflow(t, gitA, llmA)
	ctx := context.Background()
	result, err := wf.Run(ctx, "tag_delete", "delete tag v0.9.0",
		map[string]string{"tag": "v0.9.0"})
	if err != nil {
		t.Fatalf("Run tag_delete: %v", err)
	}

	detail = result.Status
	t.Logf("tag_delete: status=%s output=%s", result.Status, result.Output)

	if contains(gitTags(dir), "v0.9.0") {
		t.Errorf("v0.9.0 should be gone after delete, got: %v", gitTags(dir))
	}
}

// TestTagCreateWithApprovalFlow tests tag creation through Start→Apply.
func TestTagCreateWithApprovalFlow(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf, _ := makeWorkflow(t, gitA, llmA)

	ctx := context.Background()
	startResult, err := wf.Run(ctx, "tag_create", "create v2.0.0 release tag",
		map[string]string{"tag": "v2.0.0"})
	if err != nil {
		t.Fatalf("Run START tag_create: %v", err)
	}
	if startResult.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %s", startResult.Status)
	}

	detail = "start→apply"
	t.Logf("START: status=%s preview=%s", startResult.Status, startResult.Preview)

	applyResult, err := wf.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	t.Logf("APPLY: status=%s output=%s", applyResult.Status, applyResult.Output)

	if !contains(gitTags(dir), "v2.0.0") {
		t.Errorf("expected v2.0.0 after Apply, got: %v", gitTags(dir))
	}
}

// TestMultipleBranchesAndTags stress-tests by creating several branches and tags in sequence.
func TestMultipleBranchesAndTags(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	wf := makeDirectWorkflow(t, gitA, llmA)
	ctx := context.Background()

	wantBranches := []string{"feat/one", "feat/two", "fix/three"}
	wantTags := []string{"v0.1.0", "v0.2.0"}

	for _, b := range wantBranches {
		if _, err := wf.Run(ctx, "branch_create", fmt.Sprintf("create %s", b),
			map[string]string{"branch": b}); err != nil {
			t.Errorf("branch_create %s: %v", b, err)
		}
	}
	for _, tag := range wantTags {
		if _, err := wf.Run(ctx, "tag_create", fmt.Sprintf("create %s tag", tag),
			map[string]string{"tag": tag}); err != nil {
			t.Errorf("tag_create %s: %v", tag, err)
		}
	}

	gotBranches := gitBranches(dir)
	gotTags := gitTags(dir)

	for _, b := range wantBranches {
		if !contains(gotBranches, b) {
			t.Errorf("branch %s missing from %v", b, gotBranches)
		}
	}
	for _, tag := range wantTags {
		if !contains(gotTags, tag) {
			t.Errorf("tag %s missing from %v", tag, gotTags)
		}
	}

	detail = fmt.Sprintf("b=%d t=%d", len(gotBranches), len(gotTags))
	t.Logf("branches=%v tags=%v", gotBranches, gotTags)
}

// ─── internal helper ─────────────────────────────────────────────────────────

func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
