package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskLogger_LogStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.log")
	l := newTaskLogger(path, 100)
	l.logStart()

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "START") {
		t.Errorf("log should contain START, got: %s", string(data))
	}
}

func TestTaskLogger_LogCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.log")
	l := newTaskLogger(path, 100)
	l.logCommit("feat: add feature")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "feat: add feature") {
		t.Errorf("log should contain commit message, got: %s", string(data))
	}
}

func TestTaskLogger_LogProgress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.log")
	l := newTaskLogger(path, 100)
	l.logProgress(2, 5)

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "2/5") {
		t.Errorf("log should contain '2/5', got: %s", string(data))
	}
}

func TestTaskLogger_LogPush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.log")
	l := newTaskLogger(path, 100)
	l.logPush("origin/main")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "PUSH") {
		t.Errorf("log should contain PUSH, got: %s", string(data))
	}
}

func TestTaskLogger_LogError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.log")
	l := newTaskLogger(path, 100)
	l.logError("something went wrong")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "ERROR") {
		t.Errorf("log should contain ERROR, got: %s", string(data))
	}
}

func TestTaskLogger_LogDone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.log")
	l := newTaskLogger(path, 100)
	l.logDone(3)

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "DONE") {
		t.Errorf("log should contain DONE, got: %s", string(data))
	}
}

func TestTaskLogger_RotatesLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotate.log")
	l := newTaskLogger(path, 3)

	for i := 0; i < 10; i++ {
		l.logCommit("msg")
	}

	lines, _ := l.readLines()
	if len(lines) > 3 {
		t.Errorf("readLines() = %d lines after rotation, want <= 3", len(lines))
	}
}

func TestTaskLogger_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "task.log")
	l := newTaskLogger(path, 10)
	l.logStart()

	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		t.Error("newTaskLogger should create nested directories")
	}
}

func TestTaskLogger_ReadLines_NonExistentFile(t *testing.T) {
	l := &taskLogger{logPath: "/nonexistent/path/log.txt", maxLogLines: 10}
	lines, err := l.readLines()
	if err != nil {
		t.Errorf("readLines() on non-existent file should return nil error, got: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("readLines() on non-existent file should return empty slice, got %d lines", len(lines))
	}
}

func TestTaskLogger_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.log")
	l := newTaskLogger(path, 100)

	l.logStart()
	l.logCommit("feat: first")
	l.logCommit("fix: second")
	l.logProgress(2, 2)
	l.logDone(2)

	lines, _ := l.readLines()
	if len(lines) < 5 {
		t.Errorf("readLines() = %d lines, want >= 5", len(lines))
	}
}

// Test concurrent logging does not panic
func TestTaskLogger_ConcurrentLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.log")
	l := newTaskLogger(path, 100)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			l.logCommit("concurrent commit")
			if n == 9 {
				close(done)
			}
		}(i)
	}
	<-done
}
