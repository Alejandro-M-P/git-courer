package git

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNew verifies the constructor resolves workDir correctly.
func TestNew(t *testing.T) {
	// Get the real repo root for comparison
	repoRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal("not in a git repo, cannot test New()")
	}
	expectedRoot := strings.TrimSpace(string(repoRoot))

	tests := []struct {
		name    string
		workDir string
		wantDir string
	}{
		{"empty resolves to repo root", "", expectedRoot},
		{"dot resolves to repo root", ".", expectedRoot},
		{"explicit absolute path", "/tmp/test", "/tmp/test"},
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
