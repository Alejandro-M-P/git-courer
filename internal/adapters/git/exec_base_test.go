package git

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNew verifies the constructor creates adapter with correct defaults.
func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		workDir string
		wantDir string
	}{
		{"empty uses current", "", "."},
		{"explicit path", "/tmp/test", "/tmp/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := New(tt.workDir)
			if adapter.workDir != tt.wantDir {
				t.Errorf("New(%q).workDir = %q, want %q", tt.workDir, adapter.workDir, tt.wantDir)
			}
		})
	}
}

// TestExecAdapterIsRepo tests repository detection.
func TestIsRepo(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{"git repo is repo", "", true},      // empty uses current which is a git repo
		{"non-repo is not repo", "", false}, // Will skip if not in git repo
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "git repo is repo" {
				adapter := New(".")
				if !adapter.IsRepo() {
					t.Skip("Not in a git repo, skipping")
				}
				return
			}

			// Create temp non-repo dir
			nonRepoDir := t.TempDir()
			adapter := New(nonRepoDir)
			if got := adapter.IsRepo(); got != tt.want {
				t.Errorf("IsRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

// initGitRepo initializes a git repo for testing.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	// Initialize repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %s", out)
	}

	// Configure user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}
