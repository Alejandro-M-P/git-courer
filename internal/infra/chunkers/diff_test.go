package chunkers

import (
	"strings"
	"testing"
)

const simpleDiff = `diff --git a/main.go b/main.go
index abc123..def456 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main

+import "fmt"
+
 func main() {
-	println("hello")
+	fmt.Println("hello world")
 }`

const multiFileDiff = `diff --git a/auth/login.go b/auth/login.go
index 111..222 100644
--- a/auth/login.go
+++ b/auth/login.go
@@ -1,3 +1,4 @@
 package auth
+// Login handler
 func Login() {}
diff --git a/auth/logout.go b/auth/logout.go
index 333..444 100644
--- a/auth/logout.go
+++ b/auth/logout.go
@@ -1,3 +1,4 @@
 package auth
+// Logout handler
 func Logout() {}`

// --- DiffChunker.Chunk ---

func TestDiffChunker_EmptyDiff(t *testing.T) {
	c := NewDiffChunker()
	chunks, err := c.Chunk("", 4096)
	if err != nil {
		t.Fatalf("Chunk(empty) error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("Chunk(empty) = %d chunks, want 0", len(chunks))
	}
}

func TestDiffChunker_SimpleDiff(t *testing.T) {
	c := NewDiffChunker()
	chunks, err := c.Chunk(simpleDiff, 4096)
	if err != nil {
		t.Fatalf("Chunk() error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("Chunk() returned 0 chunks for a valid diff")
	}
	// Each chunk should have at least one file
	for i, chunk := range chunks {
		if len(chunk.Files) == 0 {
			t.Errorf("chunk[%d] has no files", i)
		}
		if chunk.Diff == "" {
			t.Errorf("chunk[%d] has empty diff", i)
		}
	}
}

func TestDiffChunker_MultiFileDiff(t *testing.T) {
	c := NewDiffChunker()
	chunks, err := c.Chunk(multiFileDiff, 4096)
	if err != nil {
		t.Fatalf("Chunk() error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("Chunk() returned 0 chunks for multi-file diff")
	}
	// All files should appear in some chunk
	allFiles := make(map[string]bool)
	for _, chunk := range chunks {
		for _, f := range chunk.Files {
			allFiles[f] = true
		}
	}
	for _, expected := range []string{"auth/login.go", "auth/logout.go"} {
		if !allFiles[expected] {
			t.Errorf("file %q not found in any chunk", expected)
		}
	}
}

func TestDiffChunker_SmallMaxChunkSize(t *testing.T) {
	c := NewDiffChunker()
	// Very small chunk size forces splitting
	chunks, err := c.Chunk(multiFileDiff, 50)
	if err != nil {
		t.Fatalf("Chunk() error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("Chunk() returned 0 chunks")
	}
}

func TestDiffChunker_FallbackOnInvalidDiff(t *testing.T) {
	c := NewDiffChunker()
	// Invalid diff — should use fallback (not return error)
	invalidDiff := `diff --git a/foo.go b/foo.go
not a valid unified diff header
+added line
-removed line`
	chunks, err := c.Chunk(invalidDiff, 4096)
	if err != nil {
		t.Fatalf("Chunk() should not error on invalid diff: %v", err)
	}
	_ = chunks // may be empty or have content depending on fallback
}

// --- fallbackChunk ---

func TestDiffChunker_FallbackChunk_DetectsFiles(t *testing.T) {
	c := NewDiffChunker()
	chunks := c.fallbackChunk(multiFileDiff, 4096)
	if len(chunks) == 0 {
		t.Error("fallbackChunk() returned 0 chunks")
	}
}

func TestDiffChunker_FallbackChunk_EmptyDiff(t *testing.T) {
	c := NewDiffChunker()
	chunks := c.fallbackChunk("", 4096)
	if len(chunks) == 0 {
		t.Error("fallbackChunk(empty) should still return at least one chunk (empty content)")
	}
}

// --- splitLargeFile ---

func TestDiffChunker_SplitLargeFile(t *testing.T) {
	c := NewDiffChunker()
	largeFileDiff := strings.Repeat("@@ -1,5 +1,5 @@\n+line\n-line\n", 20)
	chunks := c.splitLargeFile(largeFileDiff, "big.go", 100)
	if len(chunks) == 0 {
		t.Error("splitLargeFile() returned 0 chunks")
	}
	for _, chunk := range chunks {
		if len(chunk.Files) != 1 || chunk.Files[0] != "big.go" {
			t.Errorf("chunk.Files = %v, want [big.go]", chunk.Files)
		}
	}
}

// --- isCommonWord ---

func TestIsCommonWord(t *testing.T) {
	common := []string{"func", "type", "var", "const", "return", "if", "else", "for", "range"}
	for _, w := range common {
		if !isCommonWord(w) {
			t.Errorf("isCommonWord(%q) = false, want true", w)
		}
	}
	notCommon := []string{"Login", "UserService", "handleRequest", "authToken"}
	for _, w := range notCommon {
		if isCommonWord(w) {
			t.Errorf("isCommonWord(%q) = true, want false", w)
		}
	}
}

// --- getPrefix ---

func TestGetPrefix(t *testing.T) {
	cases := []struct{ name, want string }{
		{"auth/login.go", "auth/login"},
		{"main.go", "main"},
		{"noext", "noext"},
		{"path/to/file.ts", "path/to/file"},
	}
	for _, tc := range cases {
		got := getPrefix(tc.name)
		if got != tc.want {
			t.Errorf("getPrefix(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// --- fmtFileHeader ---

func TestFmtFileHeader(t *testing.T) {
	got := fmtFileHeader("auth/login.go")
	if !strings.Contains(got, "auth/login.go") {
		t.Errorf("fmtFileHeader() = %q, should contain filename", got)
	}
	if !strings.HasPrefix(got, "## ") {
		t.Errorf("fmtFileHeader() = %q, should start with '## '", got)
	}
}

// --- buildGraph / pruneGraph / calculateForce ---

func TestDiffChunker_CalculateForce_SamePrefix(t *testing.T) {
	c := NewDiffChunker()
	force := c.calculateForce("auth/login.go", "auth/logout.go", []string{"token"}, []string{"token"})
	// Same prefix "auth/login" vs "auth/logout" — no, getPrefix strips extension
	// "auth/login" vs "auth/logout" — different, but shared token "token"
	if force <= 0 {
		t.Errorf("calculateForce should be > 0 for files with shared tokens, got %d", force)
	}
}

func TestDiffChunker_CalculateForce_NoSharedTokens(t *testing.T) {
	c := NewDiffChunker()
	force := c.calculateForce("a.go", "b.go", []string{"Alpha", "Beta"}, []string{"Gamma", "Delta"})
	if force != 0 {
		t.Errorf("calculateForce should be 0 for files with no shared tokens, got %d", force)
	}
}

func TestDiffChunker_PruneGraph_FiltersLowForce(t *testing.T) {
	c := NewDiffChunker()
	c.minForce = 5
	graph := map[string]map[string]int{
		"a.go": {"b.go": 3, "c.go": 10},
		"b.go": {"a.go": 3},
		"c.go": {"a.go": 10},
	}
	pruned := c.pruneGraph(graph)
	if _, ok := pruned["a.go"]["b.go"]; ok {
		t.Error("pruneGraph should remove edge with force < minForce")
	}
	if _, ok := pruned["a.go"]["c.go"]; !ok {
		t.Error("pruneGraph should keep edge with force >= minForce")
	}
}
