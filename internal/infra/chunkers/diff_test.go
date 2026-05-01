package chunkers

import (
	"fmt"
	"strings"
	"testing"
)

func TestDiffChunker_Options(t *testing.T) {
	tests := []struct {
		name     string
		opts     []Option
		wantMax  int
		wantMin  int
		wantSize int
	}{
		{
			name:     "defaults when no options",
			opts:     nil,
			wantMax:  5,
			wantMin:  2,
			wantSize: 0,
		},
		{
			name:     "custom max files",
			opts:     []Option{WithMaxFilesPerChunk(12)},
			wantMax:  12,
			wantMin:  2,
			wantSize: 0,
		},
		{
			name:     "custom min force",
			opts:     []Option{WithMinForce(3)},
			wantMax:  5,
			wantMin:  3,
			wantSize: 0,
		},
		{
			name:     "custom chunk size",
			opts:     []Option{WithChunkSize(6000)},
			wantMax:  5,
			wantMin:  2,
			wantSize: 6000,
		},
		{
			name:     "all options combined",
			opts:     []Option{WithMaxFilesPerChunk(12), WithMinForce(3), WithChunkSize(6000)},
			wantMax:  12,
			wantMin:  3,
			wantSize: 6000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewDiffChunker(tt.opts...)
			if c.maxFilesPerChunk != tt.wantMax {
				t.Errorf("maxFilesPerChunk = %d, want %d", c.maxFilesPerChunk, tt.wantMax)
			}
			if c.minForce != tt.wantMin {
				t.Errorf("minForce = %d, want %d", c.minForce, tt.wantMin)
			}
			if c.chunkSize != tt.wantSize {
				t.Errorf("chunkSize = %d, want %d", c.chunkSize, tt.wantSize)
			}
		})
	}
}

func TestDiffChunker_ChunkSizeFallback(t *testing.T) {
	// When maxChunkSize arg is 0, chunker should use its configured chunkSize fallback
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
+ ` + strings.Repeat("X", 500) + `
diff --git b/b.go b/b.go
--- b/b.go
+++ b/b.go
+ ` + strings.Repeat("Y", 500)

	c := NewDiffChunker(WithChunkSize(800))
	chunks, err := c.Chunk(diff, 0)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks when fallback chunkSize=800 applied, got %d", len(chunks))
	}
}

func TestDiffChunker_smartSplitBySubdir(t *testing.T) {
	c := NewDiffChunker()
	cluster := []string{
		"internal/infra/chunkers/diff.go",
		"internal/infra/chunkers/diff_graph.go",
		"internal/infra/chunkers/diff_test.go",
		"internal/core/domain/chunk.go",
		"internal/core/ports/chunker.go",
		"internal/delivery/mcp/server.go",
		"internal/workflow/commit.go",
		"internal/adapters/git/repo.go",
		"internal/config/config.go",
		"internal/security/scanner.go",
	}

	clusters, leftovers := c.smartSplitBySubdir(cluster)

	// Expect at least 2 clusters (infra: 3 files, core: 2 files)
	if len(clusters) < 2 {
		t.Errorf("Expected at least 2 clusters after split, got %d", len(clusters))
	}
	if len(leftovers) == 0 {
		t.Errorf("Expected some leftovers for single-file groups")
	}

	// No cluster should be a 1-file group (those become leftovers)
	for _, cl := range clusters {
		if len(cl) < 2 {
			t.Errorf("Expected no 1-file clusters, got %v", cl)
		}
	}

	// All original files must be accounted for
	seen := make(map[string]bool)
	for _, cl := range clusters {
		for _, f := range cl {
			seen[f] = true
		}
	}
	for _, f := range leftovers {
		seen[f] = true
	}
	if len(seen) != len(cluster) {
		t.Errorf("Expected all %d files accounted for, got %d", len(cluster), len(seen))
	}
}

func TestDiffChunker_absorbLeftovers(t *testing.T) {
	c := NewDiffChunker()
	clusters := [][]string{
		{"internal/infra/chunkers/diff.go", "internal/infra/chunkers/diff_graph.go"},
		{"internal/core/domain/chunk.go", "internal/core/ports/chunker.go"},
	}
	leftovers := []string{"internal/infra/chunkers/diff_test.go"}
	files := []fileInfo{
		{name: "internal/infra/chunkers/diff.go", size: 100},
		{name: "internal/infra/chunkers/diff_graph.go", size: 100},
		{name: "internal/infra/chunkers/diff_test.go", size: 100},
		{name: "internal/core/domain/chunk.go", size: 100},
		{name: "internal/core/ports/chunker.go", size: 100},
	}

	result := c.absorbLeftovers(clusters, leftovers, files)

	found := false
	for _, cl := range result {
		for _, f := range cl {
			if f == "internal/infra/chunkers/diff_test.go" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("Expected leftover to be absorbed into a cluster")
	}
}

func TestDiffChunker_sortClustersByForce(t *testing.T) {
	c := NewDiffChunker()
	graph := map[string]map[string]int{
		"a.go": {"b.go": 1000},
		"b.go": {"a.go": 1000},
		"c.go": {},
		"d.go": {"e.go": 100},
		"e.go": {"d.go": 100},
	}
	clusters := [][]string{
		{"c.go"},
		{"d.go", "e.go"},
		{"a.go", "b.go"},
	}

	sorted := c.sortClustersByForce(clusters, graph)

	if len(sorted) != 3 {
		t.Fatalf("Expected 3 clusters, got %d", len(sorted))
	}
	// Highest force first: a.go+b.go (score 2000)
	if !sliceContains(sorted[0], "a.go") || !sliceContains(sorted[0], "b.go") {
		t.Errorf("Expected first cluster to be a.go+b.go, got %v", sorted[0])
	}
}

func sliceContains(sl []string, s string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}
	return false
}

func TestDiffChunker_Chunk_MediumDiffSemanticClustering(t *testing.T) {
	c := NewDiffChunker(WithMaxFilesPerChunk(12), WithMinForce(3), WithChunkSize(6000))
	var diffParts []string
	files := []string{
		"internal/infra/chunkers/diff.go",
		"internal/infra/chunkers/diff_graph.go",
		"internal/infra/chunkers/diff_test.go",
		"internal/core/domain/chunk.go",
		"internal/core/ports/chunker.go",
		"internal/delivery/mcp/server.go",
		"internal/workflow/commit.go",
		"internal/adapters/git/repo.go",
		"internal/config/config.go",
		"internal/security/scanner.go",
		"cmd/main.go",
		"README.md",
	}
	for i, f := range files {
		diffParts = append(diffParts, fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n+ func Func%d() {}", f, f, f, f, i))
	}
	diff := strings.Join(diffParts, "\n")

	chunks, err := c.Chunk(diff, 0) // uses chunkSize=6000 fallback
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	var totalFiles int
	for _, ch := range chunks {
		totalFiles += len(ch.Files)
	}
	if totalFiles != len(files) {
		t.Errorf("Expected %d total files across chunks, got %d", len(files), totalFiles)
	}

	// With 12 files and chunkSize=6000, we should have fewer than len(files) chunks
	// because chunker groups related files. 12 files with semantic clustering should yield <= 11.
	if len(chunks) >= len(files)-1 {
		t.Errorf("Expected some clustering to reduce chunks, got %d chunks for %d files", len(chunks), len(files))
	}
}

func TestDiffChunker_Chunk_SemanticGo(t *testing.T) {
	c := NewDiffChunker()

	// Create a diff where a.go calls a function defined in b.go
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
+ func Caller() {
+ 	ProcessPayment()
+ }
diff --git b/b.go b/b.go
--- b/b.go
+++ b/b.go
+ func ProcessPayment() {
+ 	println("done")
+ }`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Because they have a semantic link (Caller calls ProcessPayment), they should be in the SAME chunk
	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk due to semantic link, got %d", len(chunks))
	}
}

func TestDiffChunker_Chunk_PolyglotPython(t *testing.T) {
	c := NewDiffChunker()

	// Create a python diff with a class and a call
	diff := `diff --git a/service.py b/a.py
--- a/service.py
+++ b/service.py
+ class PaymentService:
+     def authorize(self):
+         pass
diff --git b/client.py b/b.py
--- b/client.py
+++ b/client.py
+ svc = PaymentService()
+ svc.authorize()`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should be grouped because of class/method reference
	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk due to python semantic link, got %d", len(chunks))
	}
}

func TestDiffChunker_Chunk_CodeTestPair(t *testing.T) {
	c := NewDiffChunker()

	// Code and Test should always be together
	diff := `diff --git a/auth.go b/auth.go
--- a/auth.go
+++ b/auth.go
+ func Auth() {}
diff --git b/auth_test.go b/auth_test.go
--- b/auth_test.go
+++ b/auth_test.go
+ func TestAuth(t *testing.T) {}`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for code-test pair, got %d", len(chunks))
	}
}

func TestDiffChunker_Chunk_LargeSplit(t *testing.T) {
	c := NewDiffChunker()

	// Two unrelated files that exceed chunk size should be split
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
+ ` + strings.Repeat("A", 2000) + `
diff --git b/b.go b/b.go
--- b/b.go
+++ b/b.go
+ ` + strings.Repeat("B", 2000)

	// Set small max chunk size to force split
	chunks, err := c.Chunk(diff, 1000)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks for unrelated large files, got %d", len(chunks))
	}
}

func TestDiffChunker_Chunk_FiltersBlacklistedPrefixes(t *testing.T) {
	c := NewDiffChunker()

	diff := `diff --git a/a.go b/a.go
index aaaaaaa..bbbbbbb 100644
--- a/a.go
+++ b/a.go
@@ -10,5 +10,5 @@ func Bar() {
    x := 1
-   y := 2
+   y := 3
 }
\ No newline at end of file
diff --git b/b.go b/b.go
index cccccc..dddddd 100644
--- b/b.go
+++ b/b.go
@@ -1,2 +1,2 @@ func Foo() {
-   a := 1
+   a := 2
}`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	blacklisted := []string{"diff --git", "index ", "@@", "\\"}
	for _, chunk := range chunks {
		lines := strings.Split(chunk.Diff, "\n")
		for _, line := range lines {
			for _, prefix := range blacklisted {
				if strings.HasPrefix(line, prefix) {
					t.Errorf("chunk contains blacklisted prefix %q: %q", prefix, line)
				}
			}
		}
	}
}
