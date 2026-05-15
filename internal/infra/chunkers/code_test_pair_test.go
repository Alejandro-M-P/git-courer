package chunkers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffChunker_CodeTestPair_StayTogether(t *testing.T) {
	repoDir := t.TempDir()

	for _, cmd := range []string{
		"git init",
		"git config user.email test@test",
		"git config user.name test",
	} {
		parts := strings.Fields(cmd)
		c := exec.Command(parts[0], parts[1:]...)
		c.Dir = repoDir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", cmd, err, out)
		}
	}

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

	for _, f := range []string{"config/project.go", "config/project_test.go"} {
		c := exec.Command("git", "add", f)
		c.Dir = repoDir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git add %s: %v\n%s", f, err, out)
		}
	}

	c := exec.Command("git", "diff", "--staged")
	c.Dir = repoDir
	diffBytes, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	diff := string(diffBytes)

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
