package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMetadataPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{".git/git-courer", true},
		{".git/git-courer/", true},
		{".git/git-courer/config.json", true},
		{".git/git-courer/branches/feat-add-pi/commits.json", true},
		{"internal/core/domain/metadata.go", false},
		{"git-courer", false},
		{"foo/.git-courer", false},
		{"foo/.git/git-courer", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := IsMetadataPath(tt.path)
			if got != tt.want {
				t.Errorf("IsMetadataPath(%q) = %v; want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestMetadataDirConstant(t *testing.T) {
	if MetadataDir != ".git/git-courer" {
		t.Errorf("MetadataDir = %q; want %q", MetadataDir, ".git/git-courer")
	}
}

func TestResolveMetadataDir(t *testing.T) {
	t.Run("normal repository metadata resolution", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a .git directory
		if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
			t.Fatalf("failed to create .git dir: %v", err)
		}

		got := ResolveMetadataDir(tmpDir)
		want := filepath.Join(tmpDir, ".git", "git-courer")
		if got != want {
			t.Errorf("ResolveMetadataDir() = %q; want %q", got, want)
		}
	})

	t.Run("repository without git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// .git does not exist

		got := ResolveMetadataDir(tmpDir)
		want := filepath.Join(tmpDir, ".git", "git-courer")
		if got != want {
			t.Errorf("ResolveMetadataDir() = %q; want %q", got, want)
		}
	})

	t.Run("linked Git worktree resolution with commondir", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create mock main repository .git directory
		mainGitDir := filepath.Join(t.TempDir(), ".git")
		if err := os.MkdirAll(mainGitDir, 0755); err != nil {
			t.Fatalf("failed to create main .git dir: %v", err)
		}

		// Create worktree gitdir
		gitDir := filepath.Join(mainGitDir, "worktrees", "wt1")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create worktree gitdir: %v", err)
		}

		// Write commondir file pointing to main repository .git directory (relative to gitDir)
		// Usually commondir in <mainGitDir>/worktrees/wt1 contains "../.."
		if err := os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../.."), 0644); err != nil {
			t.Fatalf("failed to write commondir: %v", err)
		}

		// Create the .git file in the worktree root
		gitFileContent := "gitdir: " + gitDir
		if err := os.WriteFile(filepath.Join(tmpDir, ".git"), []byte(gitFileContent), 0644); err != nil {
			t.Fatalf("failed to write .git file: %v", err)
		}

		got := ResolveMetadataDir(tmpDir)
		want := filepath.Clean(filepath.Join(mainGitDir, "git-courer"))
		if got != want {
			t.Errorf("ResolveMetadataDir() = %q; want %q", got, want)
		}
	})

	t.Run("linked Git worktree resolution without commondir file", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create worktree gitdir
		gitDir := t.TempDir()

		// Create the .git file in the worktree root pointing to gitDir
		gitFileContent := "gitdir: " + gitDir
		if err := os.WriteFile(filepath.Join(tmpDir, ".git"), []byte(gitFileContent), 0644); err != nil {
			t.Fatalf("failed to write .git file: %v", err)
		}

		got := ResolveMetadataDir(tmpDir)
		want := filepath.Join(gitDir, "git-courer")
		if got != want {
			t.Errorf("ResolveMetadataDir() = %q; want %q", got, want)
		}
	})

	t.Run("invalid gitdir format fallback", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create the .git file with invalid format
		if err := os.WriteFile(filepath.Join(tmpDir, ".git"), []byte("invalid content"), 0644); err != nil {
			t.Fatalf("failed to write invalid .git file: %v", err)
		}

		got := ResolveMetadataDir(tmpDir)
		want := filepath.Join(tmpDir, ".git", "git-courer")
		if got != want {
			t.Errorf("ResolveMetadataDir() = %q; want %q", got, want)
		}
	})
}

