package chunkers

import (
	"testing"
)

func TestDiffChunker_NewFileCodeTestPair(t *testing.T) {
	c := NewDiffChunker()

	diff := `diff --git a/config/project.go b/config/project.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/config/project.go
@@ -0,0 +1,40 @@
+package config
+
+type ProjectConfig struct {
+	Description string
+	TestCommand string
+}
diff --git a/config/project_test.go b/config/project_test.go
new file mode 100644
index 0000000..def5678
--- /dev/null
+++ b/config/project_test.go
@@ -0,0 +1,30 @@
+package config
+
+import (
+	"testing"
+)
+
+func TestProjectConfig_Load(t *testing.T) {
+	cfg := &ProjectConfig{}
+	if cfg.TestCommand != "" {
+		t.Error("expected empty")
+	}
+}`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	arePaired := c.GetLanguageCatalog().ArePaired("config/project.go", "config/project_test.go")
	t.Logf("ArePaired: %v", arePaired)
	t.Logf("Chunks: %d", len(chunks))
	for i, ch := range chunks {
		t.Logf("  Chunk %d: %v", i, ch.Files)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for code+test pair (new files), got %d", len(chunks))
	}
	
	if len(chunks) > 0 {
		foundCode := false
		foundTest := false
		for _, f := range chunks[0].Files {
			if f == "config/project.go" { foundCode = true }
			if f == "config/project_test.go" { foundTest = true }
		}
		if !foundCode || !foundTest {
			t.Errorf("Expected both files in chunk, got code=%v test=%v", foundCode, foundTest)
		}
	}
}
