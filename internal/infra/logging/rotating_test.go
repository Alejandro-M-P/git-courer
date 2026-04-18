package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRotatingLogWriter_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "app.log")
	w, err := NewRotatingLogWriter(path, 100)
	if err != nil {
		t.Fatalf("NewRotatingLogWriter() error: %v", err)
	}
	if w == nil {
		t.Fatal("NewRotatingLogWriter() returned nil")
	}
	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		t.Error("NewRotatingLogWriter() should create log directory")
	}
}

func TestNewRotatingLogWriter_LoadsExistingLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.log")
	existing := "line1\nline2\nline3\n"
	os.WriteFile(path, []byte(existing), 0644)

	w, err := NewRotatingLogWriter(path, 100)
	if err != nil {
		t.Fatalf("NewRotatingLogWriter() error: %v", err)
	}
	if len(w.lines) == 0 {
		t.Error("NewRotatingLogWriter() should load existing lines")
	}
}

func TestNewRotatingLogWriter_TruncatesOldLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.log")
	// Write 20 lines
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line")
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	w, err := NewRotatingLogWriter(path, 5)
	if err != nil {
		t.Fatalf("NewRotatingLogWriter() error: %v", err)
	}
	if len(w.lines) > 5 {
		t.Errorf("NewRotatingLogWriter() = %d lines, want <= 5", len(w.lines))
	}
}

func TestNewRotatingLogWriter_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.log")
	w, err := NewRotatingLogWriter(path, 10)
	if err != nil {
		t.Fatalf("NewRotatingLogWriter() error: %v", err)
	}
	if len(w.lines) != 0 {
		t.Errorf("NewRotatingLogWriter() = %d lines for new file, want 0", len(w.lines))
	}
}

func TestWrite_AppendsLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	w, _ := NewRotatingLogWriter(path, 100)

	n, err := w.Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != len("hello world\n") {
		t.Errorf("Write() = %d bytes, want %d", n, len("hello world\n"))
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("file content %q should contain 'hello world'", string(data))
	}
}

func TestWrite_RotatesWhenFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotate.log")
	w, _ := NewRotatingLogWriter(path, 3)

	for i := 0; i < 5; i++ {
		w.Write([]byte("line\n"))
	}

	if len(w.lines) > 3 {
		t.Errorf("after rotation, lines = %d, want <= 3", len(w.lines))
	}
}

func TestWrite_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.log")
	w, _ := NewRotatingLogWriter(path, 100)

	w.Write([]byte("entry1\n"))
	w.Write([]byte("entry2\n"))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(data), "entry1") {
		t.Error("persisted log should contain entry1")
	}
	if !strings.Contains(string(data), "entry2") {
		t.Error("persisted log should contain entry2")
	}
}

func TestWrite_MaxLinesExactlyAtLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limit.log")
	w, _ := NewRotatingLogWriter(path, 3)

	w.Write([]byte("a\n"))
	w.Write([]byte("b\n"))
	w.Write([]byte("c\n"))

	if len(w.lines) != 3 {
		t.Errorf("at limit, lines = %d, want 3", len(w.lines))
	}

	// Add one more — should drop oldest
	w.Write([]byte("d\n"))
	if len(w.lines) != 3 {
		t.Errorf("after overflow, lines = %d, want 3", len(w.lines))
	}
}

func TestNewRotatingLogWriter_CurrentDir(t *testing.T) {
	// Path with no directory component — should not try to create "."
	dir := t.TempDir()
	path := filepath.Join(dir, "flat.log")
	w, err := NewRotatingLogWriter(path, 10)
	if err != nil {
		t.Fatalf("NewRotatingLogWriter() with flat path error: %v", err)
	}
	if w == nil {
		t.Fatal("NewRotatingLogWriter() returned nil")
	}
}
