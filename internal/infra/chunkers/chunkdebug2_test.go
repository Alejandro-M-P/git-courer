package chunkers

import (
	"testing"
)

// Reproduce: 2 new files (code+test) with a maxChunkSize that fits 1 file but not both.
func TestDiffChunker_NewFileCodeTestPair_SmallChunkSize(t *testing.T) {
	c := NewDiffChunker()

	diff := `diff --git a/config/project.go b/config/project.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/config/project.go
@@ -0,0 +1,40 @@
+package config
+
+import (
+	"encoding/json"
+	"fmt"
+	"os"
+	"path/filepath"
+)
+
+type ProjectConfig struct {
+	Description string ` + "`" + `json:"description"` + "`" + `
+	TestCommand string ` + "`" + `json:"test_command"` + "`" + `
+}
+
+func LoadProjectConfig(workDir string) (*ProjectConfig, error) {
+	configPath := filepath.Join(workDir, ".git-courer", "config.json")
+	data, err := os.ReadFile(configPath)
+	if err != nil {
+		return nil, err
+	}
+	cfg := &ProjectConfig{}
+	if err := json.Unmarshal(data, cfg); err != nil {
+		return nil, err
+	}
+	return cfg, nil
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
+	"encoding/json"
+	"os"
+	"path/filepath"
+	"testing"
+)
+
+func TestProjectConfig_Load(t *testing.T) {
+	tmpDir := t.TempDir()
+	gitcourerDir := filepath.Join(tmpDir, ".git-courer")
+	os.MkdirAll(gitcourerDir, 0755)
+
+	cfg := &ProjectConfig{Description: "test"}
+	data, _ := json.Marshal(cfg)
+	os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644)
+
+	loaded, err := LoadProjectConfig(tmpDir)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if loaded.Description != "test" {
+		t.Fatal("expected test")
+	}
+}`

	// maxChunkSize small enough that each file fits but combined doesn't
	// This simulates what happens with a real CommitServiceConfig
	chunks, err := c.Chunk(diff, 1500)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	arePaired := c.GetLanguageCatalog().ArePaired("config/project.go", "config/project_test.go")
	t.Logf("ArePaired: %v", arePaired)
	t.Logf("Chunks: %d", len(chunks))
	for i, ch := range chunks {
		t.Logf("  Chunk %d: %v (diff size=%d)", i, ch.Files, len(ch.Diff))
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for code+test pair, got %d chunks", len(chunks))
	}
	
	if len(chunks) > 0 {
		foundCode := false
		foundTest := false
		for _, f := range chunks[0].Files {
			if f == "config/project.go" { foundCode = true }
			if f == "config/project_test.go" { foundTest = true }
		}
		if !foundCode || !foundTest {
			t.Errorf("BUG: code+test pair split by size threshold! Code=%v Test=%v", foundCode, foundTest)
		}
	}
}
