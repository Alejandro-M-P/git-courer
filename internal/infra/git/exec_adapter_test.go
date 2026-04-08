package git

import (
	"testing"
)

// TestStatus tests the Status function
func TestStatus(t *testing.T) {
	adapter := NewExecAdapter(".")

	status, err := adapter.Status()
	if err != nil {
		t.Errorf("Status() error = %v", err)
		return
	}

	if status.Branch == "" {
		t.Error("Branch should not be empty")
	}
}

// TestCurrentBranch tests getting current branch name
func TestCurrentBranch(t *testing.T) {
	adapter := NewExecAdapter(".")

	branch, err := adapter.CurrentBranch()
	if err != nil {
		t.Errorf("CurrentBranch() error = %v", err)
		return
	}

	if branch == "" {
		t.Error("Branch name should not be empty")
	}
}

// TestIsRepo tests repository detection
func TestIsRepo(t *testing.T) {
	adapter := NewExecAdapter(".")

	isRepo := adapter.IsRepo()
	// This test only passes if we're in a git repository
	// Skip if not in a repo
	if !isRepo {
		t.Skip("Not in a git repository, skipping test")
	}
}

// TestDiff tests getting diff output
func TestDiff(t *testing.T) {
	adapter := NewExecAdapter(".")

	diff, err := adapter.Diff()
	if err != nil {
		t.Errorf("Diff() error = %v", err)
		return
	}

	// Diff can be empty if no changes, that's ok
	_ = diff
}

// TestLog tests getting commit log
func TestLog(t *testing.T) {
	adapter := NewExecAdapter(".")

	log, err := adapter.Log(5)
	if err != nil {
		t.Errorf("Log() error = %v", err)
		return
	}

	if log == "" {
		t.Error("Log should have output")
	}
}
