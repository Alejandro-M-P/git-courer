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
			wantMax:  12,
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
			wantMax:  12,
			wantMin:  3,
			wantSize: 0,
		},
		{
			name:     "custom chunk size",
			opts:     []Option{WithChunkSize(6000)},
			wantMax:  12,
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

func TestDiffChunker_Chunk_MediumDiffSemanticClustering(t *testing.T) {
	c := NewDiffChunker(WithMaxFilesPerChunk(12), WithMinForce(3), WithChunkSize(6000))
	var diffParts []string
	files := []string{
		"internal/infra/chunkers/diff.go",
		"internal/infra/chunkers/unified.go",
		"internal/infra/chunkers/catalog.go",
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

func TestDiffChunker_Chunk_FiltersTestFiles(t *testing.T) {
	c := NewDiffChunker()

	// auth_test.go should be filtered out; only auth.go remains.
	diff := `diff --git a/auth.go b/auth.go
--- a/auth.go
+++ b/auth.go
@@ -1 +1,3 @@
+ func Auth() {}
diff --git a/auth_test.go b/auth_test.go
--- a/auth_test.go
+++ b/auth_test.go
@@ -1 +1,3 @@
+ func TestAuth(t *testing.T) {}`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk (code only), got %d", len(chunks))
	}

	for _, f := range chunks[0].Files {
		if f == "auth_test.go" {
			t.Errorf("Test file auth_test.go should have been filtered out")
		}
	}
}

func TestDiffChunker_Chunk_FiltersMetadataFiles(t *testing.T) {
	c := NewDiffChunker()

	// Metadata files should be filtered out.
	diff := `diff --git a/auth.go b/auth.go
--- a/auth.go
+++ b/auth.go
@@ -1 +1,3 @@
+ func Auth() {}
diff --git a/.git-courer/config.json b/.git-courer/config.json
--- a/.git-courer/config.json
+++ b/.git-courer/config.json
@@ -1 +1,3 @@
+ { "some": "config" }
diff --git a/.git-courer/branches/main/commits.json b/.git-courer/branches/main/commits.json
--- a/.git-courer/branches/main/commits.json
+++ b/.git-courer/branches/main/commits.json
@@ -1 +1,3 @@
+ []`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk (code only), got %d", len(chunks))
	} else {
		for _, f := range chunks[0].Files {
			if strings.HasPrefix(f, ".git-courer") {
				t.Errorf("Metadata file %q should have been filtered out", f)
			}
		}
	}
}

func TestDiffChunker_Chunk_FallbackFiltersMetadataFiles(t *testing.T) {
	c := NewDiffChunker()

	// Malformed diff to force fallback path
	diff := `diff --git a/auth.go b/auth.go
--- a/auth.go
+++ b/auth.go
+ func Auth() {}
diff --git a/.git-courer/config.json b/.git-courer/config.json
--- a/.git-courer/config.json
+++ b/.git-courer/config.json
+ { "some": "config" }`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk (code only), got %d", len(chunks))
	} else {
		for _, f := range chunks[0].Files {
			if strings.HasPrefix(f, ".git-courer") {
				t.Errorf("Fallback metadata file %q should have been filtered out", f)
			}
		}
	}
}


func TestDiffChunker_Chunk_FiltersPythonTestFiles(t *testing.T) {
	c := NewDiffChunker()

	// Python test files (test_* prefix) should also be filtered out.
	diff := `diff --git a/app.py b/app.py
--- a/app.py
+++ b/app.py
@@ -1 +1,3 @@
+ def handler():
+     pass
diff --git a/test_app.py b/test_app.py
--- a/test_app.py
+++ b/test_app.py
@@ -1 +1,3 @@
+ def test_handler():
+     assert True`

	chunks, err := c.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk (code only), got %d", len(chunks))
	} else {
		for _, f := range chunks[0].Files {
			if f == "test_app.py" {
				t.Errorf("Python test file test_app.py should have been filtered out")
			}
		}
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

	blacklisted := []string{"diff --git", "index ", "\\"}
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
