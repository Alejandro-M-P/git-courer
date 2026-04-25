package chunkers

import (
	"strings"
	"testing"
)

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
