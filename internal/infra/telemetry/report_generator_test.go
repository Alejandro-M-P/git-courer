package telemetry

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsoleReportGenerator_GenerateReport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy jsonl file
	jsonlContent := `{"timestamp":"2026-05-01T21:33:45Z","operation":"GenerateCommitMessage","model":"llama3","latency":1000000000,"prompt":"test","response":"feat: test","success":true}` + "\n"
	err = os.WriteFile(filepath.Join(tempDir, "test.jsonl"), []byte(jsonlContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	gen := NewConsoleReportGenerator()
	err = gen.GenerateReport(tempDir)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConsoleReportGenerator_BeautifulReport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "telemetry-beautiful-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy jsonl file with diverse data
	calls := []string{
		`{"timestamp":"2026-05-01T21:33:45Z","operation":"GenerateCommitMessage","model":"llama3","latency":1000000000,"prompt":"test","response":"feat: test","success":true}`,
		`{"timestamp":"2026-05-01T21:33:46Z","operation":"GenerateChunkMessage","model":"llama3","latency":500000000,"prompt":"test chunk","response":"part of code","success":true}`,
		`{"timestamp":"2026-05-01T21:33:47Z","operation":"GenerateCommitMessage","model":"llama3","latency":2000000000,"prompt":"error","response":"","success":false}`,
	}
	err = os.WriteFile(filepath.Join(tempDir, "test.jsonl"), []byte(strings.Join(calls, "\n")), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// We want to capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	gen := NewConsoleReportGenerator()
	err = gen.GenerateReport(tempDir)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	out, _ := io.ReadAll(r)
	output := string(out)

	expectedSubstrings := []string{
		"SUMMARY DASHBOARD",
		"LLM CALLS",
		"LLM CALL DETAILS",
		"Success Rate",
		"Avg Latency",
		"llama3",
		"GenerateCommitMessage",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(strings.ToUpper(output), strings.ToUpper(sub)) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", sub, output)
		}
	}
}

func TestConsoleReportGenerator_Truncation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "telemetry-truncation-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	longString := strings.Repeat("A", 600)
	call := `{"timestamp":"2026-05-01T21:33:45Z","operation":"Test","model":"m","latency":1,"prompt":"` + longString + `","response":"` + longString + `","success":true}`

	err = os.WriteFile(filepath.Join(tempDir, "test.jsonl"), []byte(call), 0644)
	if err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	gen := NewConsoleReportGenerator()
	err = gen.GenerateReport(tempDir)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "[truncated]") {
		t.Error("expected output to contain '[truncated]' for long strings")
	}
}
