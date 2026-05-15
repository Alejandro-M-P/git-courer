package chunkers

import (
	"strings"
	"testing"
)

// Generate a chunk of content with approximate size
func sizedBlock(chars int) string {
	return strings.Repeat("+line " + strings.Repeat("x", 60) + "\n", chars / 67)
}

func TestDiffChunker_NewFileCodeTestPair_RealisticSize(t *testing.T) {
	c := NewDiffChunker()

	// Simulate the real scenario: project.go (~2500 bytes) and project_test.go (~5700 bytes)
	projectGo := sizedBlock(2400)
	projectTest := sizedBlock(5600)

	diff := `diff --git a/config/project.go b/config/project.go
new file mode 100644
--- /dev/null
+++ b/config/project.go
@@ -0,0 +1,100 @@
package config
` + projectGo +
`diff --git a/config/project_test.go b/config/project_test.go
new file mode 100644
--- /dev/null
+++ b/config/project_test.go
@@ -0,0 +1,200 @@
package config
` + projectTest

	t.Logf("Diff total size: %d bytes", len(diff))

	// maxChunkSize=4000 — this is the maximum from DefaultCommitServiceConfig
	chunks, err := c.Chunk(diff, 4000)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	arePaired := c.GetLanguageCatalog().ArePaired("config/project.go", "config/project_test.go")
	t.Logf("ArePaired: %v", arePaired)
	t.Logf("Chunks: %d", len(chunks))
	for i, ch := range chunks {
		t.Logf("  Chunk %d: %v (diff size=%d)", i, ch.Files, len(ch.Diff))
	}

	foundCode := false
	foundTest := false
	for _, ch := range chunks {
		for _, f := range ch.Files {
			if f == "config/project.go" { foundCode = true }
			if f == "config/project_test.go" { foundTest = true }
		}
	}

	if len(chunks) != 1 {
		t.Errorf("BUG REPRODUCED: Expected 1 chunk for code+test pair, got %d chunks. Code=%v Test=%v",
			len(chunks), foundCode, foundTest)
	}
}
