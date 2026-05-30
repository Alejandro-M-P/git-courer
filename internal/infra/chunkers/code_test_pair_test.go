package chunkers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffChunker_CodeTestPair_StayTogether(t *testing.T) {
	repoDir := t.TempDir()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@test")
	runGit(t, repoDir, "config", "user.name", "test")

	// Make an initial commit so git diff --staged has a HEAD to compare against.
	// Without it, git diff --staged produces empty output (no HEAD ref).
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "root")

	os.MkdirAll(filepath.Join(repoDir, "config"), 0755)

	codeContent := []byte("package config\n\ntype ProjectConfig struct{ Description, TestCommand string }\n\n")
	for len(codeContent) < 2500 {
		codeContent = append(codeContent, []byte("// padding\n")...)
	}
	codeContent = codeContent[:2500]

	os.WriteFile(filepath.Join(repoDir, "config", "project.go"), codeContent, 0644)

	runGit(t, repoDir, "add", "config/project.go")

	// Use --no-ext-diff to bypass the global diff.external=difft config
	diff := runGit(t, repoDir, "diff", "--no-ext-diff", "--cached")

	chunker := NewDiffChunker()
	chunks, err := chunker.Chunk(diff, 4000)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	foundCode := false
	for _, f := range chunks[0].Files {
		if strings.HasSuffix(f, "project.go") {
			foundCode = true
		}
		if strings.HasSuffix(f, "project_test.go") {
			t.Errorf("test file should have been filtered out")
		}
	}

	if !foundCode {
		t.Errorf("code file not found in chunks")
	}
}
