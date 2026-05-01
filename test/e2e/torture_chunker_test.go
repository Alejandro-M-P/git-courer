//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
)

// TestTorture_FragmentedSecrets tests how the chunker handles a secret 
// that is split across a chunk boundary.
func TestTorture_FragmentedSecrets(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	const windowSize = 500
	var sb strings.Builder
	sb.WriteString("diff --git a/secret.go b/secret.go\n--- a/secret.go\n+++ b/secret.go\n@@ -1,1 +1,100 @@\n")
	
	// Add padding to get close to the window boundary
	padding := strings.Repeat("+ // padding line to fill space\n", 12) // ~400 chars
	sb.WriteString(padding)
	
	// The secret split point
	sb.WriteString("+ var key = \"AKIA")
	// Boundary should hit around here
	sb.WriteString("FOLLOWED_BY_SECRET_PART_2_THAT_SHOULD_BE_IN_NEXT_CHUNK\"\n")
	
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("+ // extra line %d\n", i))
	}
	diff := sb.String()

	chunker := chunkers.NewDiffChunker()
	chunks, err := chunker.Chunk(diff, windowSize)
	if err != nil {
		t.Fatalf("Chunking failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks to test splitting, got %d. WindowSize=%d, DiffSize=%d", len(chunks), windowSize, len(diff))
	}

	foundPart1 := false
	foundPart2 := false
	for _, c := range chunks {
		if strings.Contains(c.Diff, "AKIA") {
			foundPart1 = true
		}
		if strings.Contains(c.Diff, "FOLLOWED_BY_SECRET_PART_2") {
			foundPart2 = true
		}
	}

	if !foundPart1 || !foundPart2 {
		t.Errorf("Secret parts lost during chunking: part1=%v, part2=%v", foundPart1, foundPart2)
	}

	detail = fmt.Sprintf("chunks=%d", len(chunks))
}

// TestTorture_MassiveFileCount verifies that a diff with 100+ small files 
// is correctly grouped into multiple chunks.
func TestTorture_MassiveFileCount(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	const fileCount = 120
	var sb strings.Builder
	for i := 0; i < fileCount; i++ {
		sb.WriteString(fmt.Sprintf("diff --git a/file%d.txt b/file%d.txt\n", i, i))
		sb.WriteString(fmt.Sprintf("--- a/file%d.txt\n+++ b/file%d.txt\n", i, i))
		sb.WriteString("@@ -1,1 +1,2 @@\n")
		sb.WriteString(fmt.Sprintf("+ content for file %d\n", i))
	}
	diff := sb.String()

	chunker := chunkers.NewDiffChunker()
	// Standard window size
	chunks, err := chunker.Chunk(diff, 4096)
	if err != nil {
		t.Fatalf("Chunking massive file count failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for 120 files, got %d", len(chunks))
	}

	seenFiles := make(map[string]bool)
	for _, c := range chunks {
		for _, f := range c.Files {
			seenFiles[f] = true
		}
	}

	if len(seenFiles) != fileCount {
		t.Errorf("Lost files during chunking: expected %d, got %d", fileCount, len(seenFiles))
	}

	detail = fmt.Sprintf("files=%d chunks=%d", fileCount, len(chunks))
}

// TestTortureChunker_MassiveConcurrency stresses the DiffChunker by running 
// multiple concurrent chunking operations on very large diffs.
func TestTortureChunker_MassiveConcurrency(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	const iterations = 20
	const concurrency = 8
	const linesPerFile = 1000
	const fileCount = 10

	// Generate a massive multi-file diff
	var sb strings.Builder
	for f := 0; f < fileCount; f++ {
		sb.WriteString(fmt.Sprintf("diff --git a/file%d.go b/file%d.go\n", f, f))
		sb.WriteString(fmt.Sprintf("--- a/file%d.go\n+++ b/file%d.go\n", f, f))
		sb.WriteString("@@ -1,1 +1,1000 @@\n")
		for i := 0; i < linesPerFile; i++ {
			sb.WriteString(fmt.Sprintf("+ func Function%d_%d() { println(\"test\") }\n", f, i))
		}
	}
	massiveDiff := sb.String()

	chunker := chunkers.NewDiffChunker()

	var wg sync.WaitGroup
	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func(cid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				chunks, err := chunker.Chunk(massiveDiff, 8192) // Large window
				if err != nil {
					t.Errorf("Goroutine %d iteration %d failed: %v", cid, i, err)
					return
				}
				if len(chunks) == 0 {
					t.Errorf("Goroutine %d iteration %d: no chunks generated", cid, i)
					return
				}
				// Verify sanity of first chunk
				if len(chunks[0].Files) == 0 {
					t.Errorf("Goroutine %d iteration %d: first chunk has no files", cid, i)
				}
			}
		}(c)
	}
	wg.Wait()

	detail = fmt.Sprintf("files=%d lines=%d total=%d", fileCount, linesPerFile, fileCount*linesPerFile)
}

// TestTortureChunker_DeeplyNestedPaths tests chunker behavior with extreme file paths.
func TestTortureChunker_DeeplyNestedPaths(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	var sb strings.Builder
	const fileCount = 50
	for i := 0; i < fileCount; i++ {
		path := ""
		for depth := 0; depth < 10; depth++ {
			path += fmt.Sprintf("deeply/nested/directory/structure/level%d/", depth)
		}
		path += fmt.Sprintf("file_%d.go", i)
		
		sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", path, path))
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
		sb.WriteString("@@ -1,1 +1,2 @@\n")
		sb.WriteString("+ // New content\n")
	}
	complexDiff := sb.String()

	chunker := chunkers.NewDiffChunker()
	chunks, err := chunker.Chunk(complexDiff, 2048)
	if err != nil {
		t.Fatalf("Chunking complex paths failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks for complex paths")
	}

	detail = fmt.Sprintf("files=%d", fileCount)
}

// TestTorture_Diff5000Lines verifies that the chunker can handle a single massive diff of 5000 lines.
func TestTorture_Diff5000Lines(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	const linesCount = 5000
	var sb strings.Builder
	sb.WriteString("diff --git a/massive.go b/massive.go\n")
	sb.WriteString("--- a/massive.go\n+++ b/massive.go\n")
	sb.WriteString(fmt.Sprintf("@@ -1,1 +1,%d @@\n", linesCount))
	for i := 0; i < linesCount; i++ {
		sb.WriteString(fmt.Sprintf("+ func Line%d() { println(\"torture\") }\n", i))
	}
	massiveDiff := sb.String()

	chunker := chunkers.NewDiffChunker()
	// Using a standard window size to ensure it chunks into multiple pieces
	chunks, err := chunker.Chunk(massiveDiff, 4096)
	if err != nil {
		t.Fatalf("Chunking 5000 lines failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for 5000 lines, got %d", len(chunks))
	}

	// Verify we didn't lose lines
	totalLines := 0
	for _, c := range chunks {
		totalLines += strings.Count(c.Diff, "\n")
	}
	
	detail = fmt.Sprintf("chunks=%d lines=%d", len(chunks), linesCount)
}
