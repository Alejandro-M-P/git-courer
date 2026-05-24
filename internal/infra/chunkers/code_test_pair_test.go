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

	testContent := []byte("package config\n\nimport \"testing\"\n\n")
	for len(testContent) < 5500 {
		testContent = append(testContent, []byte("// test padding\n")...)
	}
	testContent = testContent[:5500]

	os.WriteFile(filepath.Join(repoDir, "config", "project.go"), codeContent, 0644)
	os.WriteFile(filepath.Join(repoDir, "config", "project_test.go"), testContent, 0644)

	runGit(t, repoDir, "add", "config/project.go", "config/project_test.go")

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

	foundCode, foundTest := false, false
	for _, f := range chunks[0].Files {
		if strings.HasSuffix(f, "project.go") {
			foundCode = true
		}
		if strings.HasSuffix(f, "project_test.go") {
			foundTest = true
		}
	}

	if !foundCode || !foundTest {
		t.Errorf("code+test pair split across chunks. code=%v test=%v", foundCode, foundTest)
	}

	if len(chunks) > 1 {
		t.Logf("WARNING: got %d chunks, but at least the first chunk has both files", len(chunks))
	}
}
