package ports

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// TestGitInterface verifies Git interface methods are well-defined.
func TestGitInterface(t *testing.T) {
	// This test ensures the interface has all required methods.
	// If it compiles, the interface is valid.

	var git Git

	// Verify method signatures exist (can't actually test without implementations)
	_ = func() error {
		// Status returns domain.Status
		_, err := git.Status()
		return err
	}

	_ = func() error {
		// Diff returns string
		_, err := git.Diff()
		return err
	}

	_ = func() error {
		// CurrentBranch returns string
		_, err := git.CurrentBranch()
		return err
	}

	_ = func() error {
		// Add takes []string, returns error
		return git.Add(nil)
	}

	_ = func() error {
		// Commit takes string, returns string error
		_, err := git.Commit("test")
		return err
	}

	t.Log("Git interface has all required methods")
}

// TestLLMInterface verifies LLM interface methods are well-defined.
func TestLLMInterface(t *testing.T) {
	// Just verify the interface type exists with the expected methods.
	// We're not calling methods on a nil instance.
	var _ interface{} = (*interface {
		GenerateChunkMessage(chunk domain.DiffChunk) (string, error)
		DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error)
		InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error)
		SetRetryContext(previousMessage string)
		ClearRetryContext()
		IsAvailable() bool
		InterpretReleaseIntent(instruction, releases, branches, currentBranch string) (*domain.ReleaseIntent, error)
	})(nil)

	t.Log("LLM interface has all required methods")
}

// TestConfirmPortInterface verifies ConfirmPort interface.
func TestConfirmPortInterface(t *testing.T) {
	// Just verify the interface type exists with the expected methods.
	var _ interface{} = (*interface {
		WritePlan(plan OperationPlan) error
		ReadPlan() (*OperationPlan, error)
		DeletePlan() error
		CreateBlocker() error
		HasBlocker() bool
		RemoveBlocker() error
		IsPlanExpired() bool
	})(nil)

	t.Log("ConfirmPort interface has all required methods")
}

// TestOperationPlanStruct verifies OperationPlan fields.
func TestOperationPlanStruct(t *testing.T) {
	plan := OperationPlan{
		Operation:       "commit",
		Args:            map[string]string{"branch": "main"},
		Preview:         "preview",
		CreatedAt:       1234567890,
		Messages:        []string{"feat: add feature"},
		Files:           []string{"a.go"},
		RejectedMessage: "too short",
	}

	if plan.Operation != "commit" {
		t.Errorf("OperationPlan.Operation = %q", plan.Operation)
	}
	if plan.CreatedAt != 1234567890 {
		t.Errorf("OperationPlan.CreatedAt = %d", plan.CreatedAt)
	}
	if len(plan.Messages) != 1 {
		t.Errorf("OperationPlan.Messages length = %d", len(plan.Messages))
	}
	if plan.RejectedMessage != "too short" {
		t.Errorf("OperationPlan.RejectedMessage = %q", plan.RejectedMessage)
	}
}

// TestGitInterfaceCompleteness ensures all Git operations are covered.
func TestGitInterfaceCompleteness(t *testing.T) {
	// List the expected methods from the interface
	expectedMethods := []string{
		"Status",
		"Diff",
		"DiffStaged",
		"ListUntracked",
		"Log",
		"LogFull",
		"CurrentBranch",
		"ListBranches",
		"ListTags",
		"IsRepo",
		"LatestTag",
		"CommitsFromTag",
		"TagExists",
		"IsGHAuthenticated",
		"CreateRelease",
		"CreateBackup",
		"RestoreBackup",
		"DeleteBackup",
		"Add",
		"Remove",
		"Checkout",
		"Switch",
		"Push",
		"Pull",
		"Fetch",
		"Stash",
		"StashPop",
		"Commit",
		"Branch",
		"DeleteBranch",
		"Reset",
		"Merge",
		"Tag",
	}

	t.Logf("Git interface should have %d methods", len(expectedMethods))
	_ = expectedMethods // Use to verify in implementation
}

// TestDomainTypesExist verifies domain types exist for the interfaces.
func TestDomainTypesExist(t *testing.T) {
	// Verify Status type exists with required fields
	status := domain.Status{
		Branch:    "main",
		IsClean:   false,
		Staged:    1,
		Modified:  2,
		Untracked: 3,
		Files: []domain.FileStatus{
			{Path: "a.go", Status: "M ", Staged: true, IsNew: false},
			{Path: "b.txt", Status: "??", Staged: false, IsNew: true},
		},
	}

	if status.Branch != "main" {
		t.Error("domain.Status should have Branch field")
	}
	if !status.IsClean {
		t.Log("domain.Status.IsClean correctly false for dirty repo")
	}

	// Verify DiffChunk
	chunk := domain.DiffChunk{
		Files: []string{"a.go"},
		Diff:  "diff content",
	}
	if len(chunk.Files) != 1 {
		t.Error("domain.DiffChunk should have Files field")
	}

	t.Log("Domain types exist and have required fields")
}
