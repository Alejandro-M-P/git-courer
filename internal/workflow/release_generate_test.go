package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// mockConcurrentLLM tracks concurrent calls and supports per-chunk failures.
type mockConcurrentLLM struct {
	mu                sync.Mutex
	callCount         int
	currentConcurrent int
	maxConcurrent     int
	delay             time.Duration
	failIfContains    string
	alwaysFail        bool
}

func (m *mockConcurrentLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return "", nil
}

func (m *mockConcurrentLLM) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return domain.CommitIntent{}, nil
}

func (m *mockConcurrentLLM) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	return nil, nil
}

func (m *mockConcurrentLLM) SetRetryContext(previousMessage string) {}
func (m *mockConcurrentLLM) ClearRetryContext()                     {}

func (m *mockConcurrentLLM) IsAvailable() bool { return true }

func (m *mockConcurrentLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}

func (m *mockConcurrentLLM) AuditBinaryContent(filename, content string) (bool, error) {
	return false, nil
}

func (m *mockConcurrentLLM) GenerateChangelog(commits, previousChangelog, outputFile string) (string, error) {
	m.mu.Lock()
	m.callCount++
	m.currentConcurrent++
	if m.currentConcurrent > m.maxConcurrent {
		m.maxConcurrent = m.currentConcurrent
	}
	m.mu.Unlock()

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	m.mu.Lock()
	m.currentConcurrent--
	m.mu.Unlock()

	if m.alwaysFail {
		return "", fmt.Errorf("mock: always fail")
	}
	if m.failIfContains != "" && strings.Contains(commits, m.failIfContains) {
		return "", fmt.Errorf("mock error: contains %q", m.failIfContains)
	}
	return "changelog:" + commits, nil
}

func (m *mockConcurrentLLM) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}

// --- generateSync tests ---

// TestGenerateSync_NumParallel1_Serial verifies serial behavior when NumParallel=1.
func TestGenerateSync_NumParallel1_Serial(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockConcurrentLLM{delay: 5 * time.Millisecond}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 1, 100, filepath.Join(t.TempDir(), "release.log"))
	cfg.NumParallel = 1
	cfg.BackgroundThreshold = 10 // ensure sync path
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "A\nB\nC\nD\nE"
	changelog, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if llm.maxConcurrent != 1 {
		t.Errorf("maxConcurrent = %d, want 1 for NumParallel=1", llm.maxConcurrent)
	}

	expected := "changelog:A\n\nchangelog:B\n\nchangelog:C\n\nchangelog:D\n\nchangelog:E"
	if changelog != expected {
		t.Errorf("changelog mismatch.\nwant:\n%s\n\ngot:\n%s", expected, changelog)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warnings), warnings)
	}
}

// TestGenerateSync_NumParallel3_ParallelAndOrdered verifies bounded concurrency and preserved order.
func TestGenerateSync_NumParallel3_ParallelAndOrdered(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockConcurrentLLM{delay: 10 * time.Millisecond}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 1, 100, filepath.Join(t.TempDir(), "release.log"))
	cfg.NumParallel = 3
	cfg.BackgroundThreshold = 10
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "A\nB\nC\nD\nE"
	changelog, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if llm.maxConcurrent <= 1 {
		t.Errorf("maxConcurrent = %d, want >1 for NumParallel=3 (prove concurrency happened)", llm.maxConcurrent)
	}
	if llm.maxConcurrent > 3 {
		t.Errorf("maxConcurrent = %d, want <= 3 (bounded by NumParallel)", llm.maxConcurrent)
	}

	expected := "changelog:A\n\nchangelog:B\n\nchangelog:C\n\nchangelog:D\n\nchangelog:E"
	if changelog != expected {
		t.Errorf("changelog ordering wrong.\nwant:\n%s\n\ngot:\n%s", expected, changelog)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warnings), warnings)
	}
}

// TestGenerateSync_OneChunkFails_WarningPreserved verifies per-chunk errors become warnings and ordering is kept.
func TestGenerateSync_OneChunkFails_WarningPreserved(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockConcurrentLLM{failIfContains: "C"}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 1, 100, filepath.Join(t.TempDir(), "release.log"))
	cfg.NumParallel = 3
	cfg.BackgroundThreshold = 10
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "A\nB\nC\nD"
	changelog, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	expected := "changelog:A\n\nchangelog:B\n\nchangelog:D"
	if changelog != expected {
		t.Errorf("changelog mismatch.\nwant:\n%s\n\ngot:\n%s", expected, changelog)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "Chunk 3 failed") {
		t.Errorf("warning = %q, want contains 'Chunk 3 failed'", warnings[0])
	}
}

// TestGenerateSync_AllChunksFail_WarningsCollected verifies all failures are collected as warnings.
func TestGenerateSync_AllChunksFail_WarningsCollected(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockConcurrentLLM{alwaysFail: true}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 1, 100, filepath.Join(t.TempDir(), "release.log"))
	cfg.NumParallel = 3
	cfg.BackgroundThreshold = 10
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "A\nB\nC"
	changelog, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// When all chunks fail, generateSync falls back to raw commits joined with "\n\n"
	expected := "A\n\nB\n\nC"
	if changelog != expected {
		t.Errorf("fallback changelog wrong.\nwant:\n%s\n\ngot:\n%s", expected, changelog)
	}
	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d: %v", len(warnings), warnings)
	}
}

// --- generateBackground tests ---

// TestGenerateBackground_NumParallel3_ConcurrentAndOrdered verifies background path with bounded parallelism.
func TestGenerateBackground_NumParallel3_ConcurrentAndOrdered(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockConcurrentLLM{delay: 5 * time.Millisecond}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 1, 100, filepath.Join(t.TempDir(), "release.log"))
	cfg.NumParallel = 3
	cfg.BackgroundThreshold = 3 // 4 chunks triggers background
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "A\nB\nC\nD"
	resp, warnings, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(resp, "running") {
		t.Errorf("expected background running response, got: %s", resp)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings from background start, got %d", len(warnings))
	}

	// Poll for background completion
	var changelog string
	for i := 0; i < 100; i++ {
		changelog, _ = svc.LoadChangelog()
		if changelog != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if changelog == "" {
		t.Fatal("background did not produce changelog within timeout")
	}

	expected := "changelog:A\n\nchangelog:B\n\nchangelog:C\n\nchangelog:D"
	if changelog != expected {
		t.Errorf("changelog ordering wrong.\nwant:\n%s\n\ngot:\n%s", expected, changelog)
	}

	if llm.maxConcurrent <= 1 {
		t.Errorf("maxConcurrent = %d, want >1 for NumParallel=3 background", llm.maxConcurrent)
	}
	if llm.maxConcurrent > 3 {
		t.Errorf("maxConcurrent = %d, want <= 3", llm.maxConcurrent)
	}

	// Verify progress was logged
	logPath := svc.GetConfig().LogPath
	data, _ := os.ReadFile(logPath)
	logContent := string(data)
	if !strings.Contains(logContent, "PROGRESS") {
		t.Errorf("expected log to contain PROGRESS entries, got:\n%s", logContent)
	}
}

// TestGenerateBackground_OneChunkFails_Proceeds verifies background path skips failed chunks and continues.
func TestGenerateBackground_OneChunkFails_Proceeds(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockConcurrentLLM{failIfContains: "C", delay: 5 * time.Millisecond}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 1, 100, filepath.Join(t.TempDir(), "release.log"))
	cfg.NumParallel = 3
	cfg.BackgroundThreshold = 3
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "A\nB\nC\nD"
	_, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var changelog string
	for i := 0; i < 100; i++ {
		changelog, _ = svc.LoadChangelog()
		if changelog != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	expected := "changelog:A\n\nchangelog:B\n\nchangelog:D"
	if changelog != expected {
		t.Errorf("changelog wrong.\nwant:\n%s\n\ngot:\n%s", expected, changelog)
	}

	// Check log for error entry
	logPath := svc.GetConfig().LogPath
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "Chunk 3 failed") {
		t.Errorf("expected log to contain 'Chunk 3 failed', got:\n%s", string(data))
	}
}

// TestGenerateBackground_NumParallel1_SerialBackground verifies NumParallel=1 still works in background path.
func TestGenerateBackground_NumParallel1_SerialBackground(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockConcurrentLLM{delay: 5 * time.Millisecond}
	chunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 1, 100, filepath.Join(t.TempDir(), "release.log"))
	cfg.NumParallel = 1
	cfg.BackgroundThreshold = 3
	svc := NewReleaseService(git, llm, chunker, cfg, nil)

	commits := "A\nB\nC\nD"
	_, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var changelog string
	for i := 0; i < 100; i++ {
		changelog, _ = svc.LoadChangelog()
		if changelog != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if changelog == "" {
		t.Fatal("background did not produce changelog within timeout")
	}

	if llm.maxConcurrent != 1 {
		t.Errorf("maxConcurrent = %d, want 1 for NumParallel=1", llm.maxConcurrent)
	}
}
