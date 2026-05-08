package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// mockInitLLM is a test double for ports.LLM that only implements ProjectInit and IsAvailable.
type mockInitLLM struct {
	available    bool
	projectInit  *domain.ProjectConfig
	projectError error
}

func (m *mockInitLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) {
	if m.projectError != nil {
		return nil, m.projectError
	}
	return m.projectInit, nil
}
func (m *mockInitLLM) IsAvailable() bool { return m.available }
func (m *mockInitLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return "", nil
}
func (m *mockInitLLM) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return domain.CommitIntent{}, nil
}
func (m *mockInitLLM) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	return nil, nil
}
func (m *mockInitLLM) SetRetryContext(previousMessage string)        {}
func (m *mockInitLLM) ClearRetryContext()                             {}
func (m *mockInitLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockInitLLM) AuditBinaryContent(filename, content string) (bool, error) {
	return false, nil
}
func (m *mockInitLLM) GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	return nil, nil
}
func (m *mockInitLLM) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}

// --- Collision guard tests ---

func TestRunInit_ConfigCollisionRefuses(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".git-courer")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Create existing config
	existingConfig := &domain.ProjectConfig{Description: "existing"}
	data, _ := json.MarshalIndent(existingConfig, "", "  ")
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	llm := &mockInitLLM{available: true}
	var stdout, stderr bytes.Buffer
	in := bufio.NewReader(strings.NewReader(""))

	err := runInit(tmpDir, llm, in, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when config already exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err.Error())
	}
}

// --- LLM happy path ---

func TestRunInit_LLMHappyPath(t *testing.T) {
	tmpDir := t.TempDir()

	llmResult := &domain.ProjectConfig{
		Description: "A git helper project",
		Areas: map[string][]string{
			"core":     {"internal/core/"},
			"adapters": {"internal/adapters/"},
		},
	}
	llm := &mockInitLLM{
		available:   true,
		projectInit: llmResult,
	}

	// User confirms with "y"
	in := bufio.NewReader(strings.NewReader("y\n"))
	var stdout, stderr bytes.Buffer

	err := runInit(tmpDir, llm, in, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	// Verify config was saved
	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "A git helper project" {
		t.Errorf("Description = %q, want %q", loaded.Description, "A git helper project")
	}
	if len(loaded.Areas) != 2 {
		t.Errorf("Areas count = %d, want 2", len(loaded.Areas))
	}

	// Verify output mentioned AI analysis
	if !strings.Contains(stdout.String(), "Analyzing project structure") {
		t.Errorf("stdout should mention AI analysis; got:\n%s", stdout.String())
	}
	// Verify saved message
	if !strings.Contains(stdout.String(), "saved") {
		t.Errorf("stdout should mention saved; got:\n%s", stdout.String())
	}
}

// --- LLM declined → cancellation ---

func TestRunInit_LLMDeclinedNoSave(t *testing.T) {
	tmpDir := t.TempDir()

	llmResult := &domain.ProjectConfig{
		Description: "A project",
		Areas:       map[string][]string{"auth": {"internal/auth/"}},
	}
	llm := &mockInitLLM{
		available:   true,
		projectInit: llmResult,
	}

	// User declines with "n"
	in := bufio.NewReader(strings.NewReader("n\n"))
	var stdout, stderr bytes.Buffer

	err := runInit(tmpDir, llm, in, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	// Config should NOT be saved
	configPath := filepath.Join(tmpDir, ".git-courer", "config.json")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file should NOT exist when user declines")
	}

	if !strings.Contains(stdout.String(), "cancelled") {
		t.Errorf("stdout should mention cancellation; got:\n%s", stdout.String())
	}
}

// --- LLM unavailable → manual entry ---

func TestRunInit_ManualEntryWhenLLMUnavailable(t *testing.T) {
	tmpDir := t.TempDir()

	llm := &mockInitLLM{available: false}

	// Manual entry: description + one area + confirmation
	input := "My test project\nauth=internal/auth/\n\ny\n"
	in := bufio.NewReader(strings.NewReader(input))
	var stdout, stderr bytes.Buffer

	err := runInit(tmpDir, llm, in, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	// Verify config was saved with manual data
	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "My test project" {
		t.Errorf("Description = %q, want %q", loaded.Description, "My test project")
	}
	if len(loaded.Areas) != 1 {
		t.Fatalf("Areas count = %d, want 1", len(loaded.Areas))
	}
	if len(loaded.Areas["auth"]) != 1 || loaded.Areas["auth"][0] != "internal/auth/" {
		t.Errorf("Areas[auth] = %v, want [internal/auth/]", loaded.Areas["auth"])
	}

	// Verify output mentioned manual entry
	if !strings.Contains(stdout.String(), "manual") {
		t.Errorf("stdout should mention manual mode; got:\n%s", stdout.String())
	}
}

// --- LLM fails → fallback to manual ---

func TestRunInit_LLMFailsFallbackManual(t *testing.T) {
	tmpDir := t.TempDir()

	llm := &mockInitLLM{
		available:    true,
		projectError: fmt.Errorf("LLM connection failed"),
	}

	// Manual entry after fallback
	input := "Fallback project\ncore=internal/core/\n\ny\n"
	in := bufio.NewReader(strings.NewReader(input))
	var stdout, stderr bytes.Buffer

	err := runInit(tmpDir, llm, in, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	// Verify config was saved with manual data
	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "Fallback project" {
		t.Errorf("Description = %q, want %q", loaded.Description, "Fallback project")
	}

	// Verify stderr mentioned LLM failure
	if !strings.Contains(stderr.String(), "AI analysis failed") {
		t.Errorf("stderr should mention LLM failure; got:\n%s", stderr.String())
	}
}

// --- Manual entry with multiple areas ---

func TestRunInit_ManualEntryMultipleAreas(t *testing.T) {
	tmpDir := t.TempDir()

	llm := &mockInitLLM{available: false}

	// Enter 2 areas then stop with empty line
	input := "Multi-area project\nauth=internal/auth/\ncore=internal/core/\n\ny\n"
	in := bufio.NewReader(strings.NewReader(input))
	var stdout, stderr bytes.Buffer

	err := runInit(tmpDir, llm, in, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if len(loaded.Areas) != 2 {
		t.Errorf("Areas count = %d, want 2", len(loaded.Areas))
	}
	if loaded.Description != "Multi-area project" {
		t.Errorf("Description = %q, want %q", loaded.Description, "Multi-area project")
	}
}

// --- LLM result with empty areas ---

func TestRunInit_LLMEmptyAreasSavesCorrectly(t *testing.T) {
	tmpDir := t.TempDir()

	llmResult := &domain.ProjectConfig{
		Description: "Simple project with no area definitions",
		Areas:       map[string][]string{},
	}
	llm := &mockInitLLM{
		available:   true,
		projectInit: llmResult,
	}

	in := bufio.NewReader(strings.NewReader("y\n"))
	var stdout, stderr bytes.Buffer

	err := runInit(tmpDir, llm, in, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	loaded, err := domain.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error: %v", err)
	}
	if loaded.Description != "Simple project with no area definitions" {
		t.Errorf("Description = %q, want %q", loaded.Description, "Simple project with no area definitions")
	}
	if len(loaded.Areas) != 0 {
		t.Errorf("Areas count = %d, want 0", len(loaded.Areas))
	}
}