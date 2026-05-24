package chunkers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/stretchr/testify/assert"
)

// --- Mock ContentProvider ---

type mockContentProvider struct {
	contents []ports.FileContent
	err      error
}

func (m *mockContentProvider) GetContents(files []string) ([]ports.FileContent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.contents, nil
}

// --- Test AnnotateDiffForRead ---

func TestAnnotateDiffForRead(t *testing.T) {
	t.Run("go_new_func", func(t *testing.T) {
		const goBefore = `package main
func existing() {}
`
		const goAfter = `package main
func existing() {}
func Helper() {
	return
}
`
		// New file diff: handler.go is new
		const rawDiff = `diff --git a/handler.go b/handler.go
new file mode 100644
--- /dev/null
+++ b/handler.go
@@ -0,0 +1,4 @@
+package main
+func Helper() {
+	return
+}
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler.go", Before: []byte(goBefore), After: []byte(goAfter)},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for Go file with new function")
		assert.Contains(t, result, "NEW_FUNC: Helper", "should label new function as NEW_FUNC")
		assert.Contains(t, result, "handler.go", "should include filename header")
	})

	t.Run("py_modified_sig", func(t *testing.T) {
		const pyBefore = `def old_func(x):
	pass
`
		const pyAfter = `def old_func(x, y):
	pass
`
		// Modified file with signature change of a public function
		const rawDiff = `diff --git a/handler.py b/handler.py
index abc123..def456 100644
--- a/handler.py
+++ b/handler.py
@@ -1,2 +1,2 @@
-def old_func(x):
+def old_func(x, y):
 `
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler.py", Before: []byte(pyBefore), After: []byte(pyAfter)},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for Python file with modified signature")
		assert.Contains(t, result, "MOD_SIG", "should label modified signature with MOD_SIG")
		// Breaking because old_func is a public function and its signature changed
		assert.Contains(t, result, "BREAKING", "public function change should be marked BREAKING")
	})

	t.Run("go_modified_sig", func(t *testing.T) {
		const goBefore = `package main
func OldFunc(x int) error {
	return nil
}
`
		const goAfter = `package main
func OldFunc(x int, y string) error {
	return nil
}
`
		const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -2,2 +2,2 @@
-func OldFunc(x int) error {
+func OldFunc(x int, y string) error {
 `
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler.go", Before: []byte(goBefore), After: []byte(goAfter)},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for Go file with modified signature")
		assert.Contains(t, result, "MOD_SIG", "should label modified signature with MOD_SIG")
		assert.Contains(t, result, "BREAKING", "public function change should be marked BREAKING")
	})

	t.Run("go_modified_body", func(t *testing.T) {
		const goBefore = `package main
func Process(data string) error {
	return nil
}
`
		const goAfter = `package main
func Process(data string) error {
	if data == "" {
		return fmt.Errorf("empty")
	}
	return nil
}
`
		const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -2,1 +2,4 @@
 func Process(data string) error {
+	if data == "" {
+		return fmt.Errorf("empty")
+	}
 }
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler.go", Before: []byte(goBefore), After: []byte(goAfter)},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for Go file with modified body")
		assert.Contains(t, result, "MOD_BODY", "should label body modification with MOD_BODY variant")
	})

	t.Run("category_deps", func(t *testing.T) {
		const rawDiff = `diff --git a/go.mod b/go.mod
index abc123..def456 100644
--- a/go.mod
+++ b/go.mod
@@ -1,1 +1,1 @@
-module github.com/example
+module github.com/example/v2
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "go.mod", Before: []byte("module github.com/example\n"), After: []byte("module github.com/example/v2\n")},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for go.mod")
		assert.Contains(t, result, "[DEPS]", "should label go.mod as DEPS")
		assert.NotContains(t, result, "[DEPS:", "category label should not have colon+name suffix")
	})

	t.Run("category_config", func(t *testing.T) {
		const rawDiff = `diff --git a/config.yaml b/config.yaml
index abc123..def456 100644
--- a/config.yaml
+++ b/config.yaml
@@ -1,1 +1,1 @@
-key: old
+key: new
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "config.yaml", Before: []byte("key: old\n"), After: []byte("key: new\n")},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for yaml config")
		assert.Contains(t, result, "[CONFIG]", "should label yaml as CONFIG")
		assert.NotContains(t, result, "[CONFIG:", "category label should not have colon+name suffix")
	})

	t.Run("category_docs", func(t *testing.T) {
		const rawDiff = `diff --git a/README.md b/README.md
index abc123..def456 100644
--- a/README.md
+++ b/README.md
@@ -1,1 +1,1 @@
-# Old Title
+# New Title
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "README.md", Before: []byte("# Old Title\n"), After: []byte("# New Title\n")},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for md docs")
		assert.Contains(t, result, "[DOCS]", "should label .md as DOCS")
	})

	t.Run("category_ci", func(t *testing.T) {
		const rawDiff = `diff --git a/Dockerfile b/Dockerfile
index abc123..def456 100644
--- a/Dockerfile
+++ b/Dockerfile
@@ -1,1 +1,1 @@
-FROM golang:1.20
+FROM golang:1.21
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "Dockerfile", Before: []byte("FROM golang:1.20\n"), After: []byte("FROM golang:1.21\n")},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for Dockerfile")
		assert.Contains(t, result, "[CI]", "should label Dockerfile as CI")
	})

	t.Run("mixed_code_and_config", func(t *testing.T) {
		const handlerBefore = `package main
func Existing() {}
`
		const handlerAfter = `package main
func Existing() {}
func NewHelper() {}
`
		// handler.go: 2 old lines (context) + 1 add = 2 old, 3 new
		const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -1,2 +1,3 @@
 package main
 func Existing() {}
+func NewHelper() {}
diff --git a/config.yaml b/config.yaml
index abc123..def456 100644
--- a/config.yaml
+++ b/config.yaml
@@ -1,1 +1,1 @@
-key: old
+key: new
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler.go", Before: []byte(handlerBefore), After: []byte(handlerAfter)},
				{Filename: "config.yaml", Before: []byte("key: old\n"), After: []byte("key: new\n")},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for mixed files")
		assert.Contains(t, result, "NEW_FUNC: NewHelper", "handler.go hunks should label new function")
		assert.Contains(t, result, "[CONFIG]", "config.yaml hunks should get CONFIG label")
	})

	t.Run("no_grammar", func(t *testing.T) {
		const rawDiff = `diff --git a/data.xyz b/data.xyz
index abc123..def456 100644
--- a/data.xyz
+++ b/data.xyz
@@ -1,1 +1,1 @@
-old line
+new line
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "data.xyz", Before: []byte("old line\n"), After: []byte("new line\n")},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		// No grammar → file is skipped entirely (no UNKNOWN_GENERIC in reading mode)
		assert.NotContains(t, result, "data.xyz", "no-grammar file should be omitted from output")
		assert.NotContains(t, result, "UNKNOWN_GENERIC", "should never produce UNKNOWN_GENERIC label")
	})

	t.Run("binary_file", func(t *testing.T) {
		const rawDiff = `diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		// Binary file should be skipped
		assert.NotContains(t, result, "image.png", "binary file should be omitted from output")
	})

	t.Run("content_provider_fails", func(t *testing.T) {
		const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -1,1 +1,1 @@
-old
+new
`
		cp := &mockContentProvider{
			err: assert.AnError,
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.Empty(t, result, "should return empty string when ContentProvider fails")
	})

	t.Run("nil_content_provider", func(t *testing.T) {
		const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -1,1 +1,1 @@
-old
+new
`
		result := AnnotateDiffForRead(rawDiff, nil)

		assert.Empty(t, result, "should return empty string when ContentProvider is nil")
	})

	t.Run("empty_diff", func(t *testing.T) {
		cp := &mockContentProvider{}

		result := AnnotateDiffForRead("", cp)

		assert.Empty(t, result, "should return empty string for empty diff input")
	})

	t.Run("multi_file_partial_failure", func(t *testing.T) {
		const handlerBefore = `package main
func Existing() {}
`
		const handlerAfter = `package main
func Existing() {}
func Helper() {}
`
		// Binary + code in same diff
		const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -1,2 +1,3 @@
 package main
 func Existing() {}
+func Helper() {}
diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler.go", Before: []byte(handlerBefore), After: []byte(handlerAfter)},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		// handler.go should be annotated, binary file skipped
		assert.NotEmpty(t, result, "should produce annotation for handler.go even with binary in diff")
		assert.Contains(t, result, "NEW_FUNC: Helper", "should annotate handler.go with new function label")
		assert.NotContains(t, result, "image.png", "binary file should be omitted")
	})

	t.Run("no_test_label", func(t *testing.T) {
		// Test files should get semantic labels from AST, NOT a "TEST" label
		const testBefore = `package main
import "testing"
func TestSomething(t *testing.T) {}
`
		const testAfter = `package main
import "testing"
func TestSomething(t *testing.T) {}
func TestNewThing(t *testing.T) {}
`
		// 3 old lines, 4 new lines (3 context + 1 add)
		const rawDiff = `diff --git a/handler_test.go b/handler_test.go
index abc123..def456 100644
--- a/handler_test.go
+++ b/handler_test.go
@@ -1,3 +1,4 @@
 package main
 import "testing"
 func TestSomething(t *testing.T) {}
+func TestNewThing(t *testing.T) {}
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler_test.go", Before: []byte(testBefore), After: []byte(testAfter)},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation for test file")
		assert.NotContains(t, result, "[TEST", "should NEVER produce TEST label")
		// Should get a semantic label like NEW_FUNC
		hasSemanticLabel := strings.Contains(result, "NEW_FUNC:") || strings.Contains(result, "MOD_BODY") || strings.Contains(result, "MOD_SIG")
		assert.True(t, hasSemanticLabel, "test file should get a semantic label from AST analysis, got: %s", result)
	})

	t.Run("full_hunk_context", func(t *testing.T) {
		const goBefore = `package main
func Process() error {
	return nil
}
func Helper() {}
`
		const goAfter = `package main
func Process() error {
	if true {
		return fmt.Errorf("fail")
	}
	return nil
}
func Helper() {}
`
		const rawDiff = `diff --git a/handler.go b/handler.go
index abc123..def456 100644
--- a/handler.go
+++ b/handler.go
@@ -2,1 +2,4 @@
 func Process() error {
+	if true {
+		return fmt.Errorf("fail")
+	}
 }
`
		cp := &mockContentProvider{
			contents: []ports.FileContent{
				{Filename: "handler.go", Before: []byte(goBefore), After: []byte(goAfter)},
			},
		}

		result := AnnotateDiffForRead(rawDiff, cp)

		assert.NotEmpty(t, result, "should produce annotation")
		// Verify full hunk body is preserved
		assert.Contains(t, result, "+	if true {", "full hunk context should be preserved")
		assert.Contains(t, result, "+		return fmt.Errorf(\"fail\")", "hunk lines should not be trimmed")
	})
}

// --- Integration Test (skipped with -short) ---

func TestAnnotateDiffForRead_RealGitRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration test skipped with -short flag")
	}

	// Create a temp git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@test.com")
	runGit(t, tmpDir, "config", "user.name", "Test")

	// Create initial Go file
	initialContent := `package main

func Existing() error {
	return nil
}
`
	writeFile(t, tmpDir, "handler.go", initialContent)
	runGit(t, tmpDir, "add", "handler.go")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Modify the Go file: add a new function
	modifiedContent := `package main

func Existing() error {
	return nil
}

func Helper() error {
	return fmt.Errorf("helper")
}
`
	writeFile(t, tmpDir, "handler.go", modifiedContent)

	// Stage changes so GitContentProvider can read from the index
	runGit(t, tmpDir, "add", "handler.go")

	// Get the raw diff (staged).
	// Use --no-ext-diff to bypass global diff.external=difft config.
	rawDiff := runGit(t, tmpDir, "diff", "--no-ext-diff", "--cached")

	if rawDiff == "" {
		t.Fatal("expected non-empty staged diff from real git repo")
	}

	// Create a GitContentProvider for the real repo
	cp := git.NewGitContentProvider(tmpDir)

	// Run AnnotateDiffForRead
	result := AnnotateDiffForRead(rawDiff, cp)

	assert.NotEmpty(t, result, "should produce annotation for real Go diff")
	assert.Contains(t, result, "handler.go", "should include filename header")
	assert.Contains(t, result, "NEW_FUNC: Helper", "should label the new function")
	// Verify bare filename (no diff --git prefix in output)
	lines := strings.Split(result, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		assert.True(t, strings.HasPrefix(firstLine, "handler.go"),
			"first line should be bare filename header, got: %s", firstLine)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}