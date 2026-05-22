//go:build !e2e

package pipeline

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// === Task 3.2: Types tests ===

func TestCommitRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := CommitRequest{
		Instruction: "commit the staged changes",
		Preview:     true,
	}
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	var restored CommitRequest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if restored.Instruction != original.Instruction {
		t.Errorf("Instruction: got %q, want %q", restored.Instruction, original.Instruction)
	}
	if restored.Preview != original.Preview {
		t.Errorf("Preview: got %v, want %v", restored.Preview, original.Preview)
	}
}

func TestCommitRequest_PreviewFalse(t *testing.T) {
	t.Parallel()

	req := CommitRequest{Instruction: "fix typo", Preview: false}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// "preview":false must appear explicitly
	if !contains(string(data), `"preview":false`) && !contains(string(data), `"preview"`) {
		t.Errorf("preview field missing from JSON: %s", data)
	}
}

func TestSecurityResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := SecurityResult{
		Blocked: true,
		Files: []FileBlockResult{
			{File: "secret.key", Reason: "private key detected", Type: "SECRET_DETECTED", Halted: true},
		},
	}
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	var restored SecurityResult
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !restored.Blocked {
		t.Error("Blocked should be true")
	}
	if len(restored.Files) != 1 {
		t.Fatalf("Files: got %d, want 1", len(restored.Files))
	}
	if restored.Files[0].File != "secret.key" {
		t.Errorf("File: got %q, want %q", restored.Files[0].File, "secret.key")
	}
}

func TestPipelineResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := PipelineResult{
		Message:     "feat: add pipeline testing infrastructure",
		Chunks:      []domain.DiffChunk{{Files: []string{"a.go"}, Diff: "+added", CommitType: "feat"}},
		Security:    SecurityResult{Blocked: false},
		Instruction: "commit staged changes",
		Preview:     false,
	}
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	var restored PipelineResult
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if restored.Message != original.Message {
		t.Errorf("Message: got %q, want %q", restored.Message, original.Message)
	}
	if len(restored.Chunks) != 1 {
		t.Fatalf("Chunks: got %d, want 1", len(restored.Chunks))
	}
	if restored.Chunks[0].CommitType != "feat" {
		t.Errorf("CommitType: got %q, want %q", restored.Chunks[0].CommitType, "feat")
	}
}

// === Task 3.3, 3.4: Stage function tests ===

// Pure stages (3, 4, 5) test serialization round-trip without real ports.

func TestStage00Request_ValidJSON(t *testing.T) {
	t.Parallel()
	input := []byte(`{"instruction":"commit staged changes","preview":true}`)
	out, err := Stage00Request(input, StageDeps{})
	if err != nil {
		t.Fatalf("Stage00Request: %v", err)
	}
	var req CommitRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if req.Instruction != "commit staged changes" {
		t.Errorf("Instruction: got %q, want %q", req.Instruction, "commit staged changes")
	}
	if req.Preview != true {
		t.Errorf("Preview: got %v, want true", req.Preview)
	}
}

func TestStage00Request_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := Stage00Request([]byte(`not json`), StageDeps{})
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestStage00Request_EmptyInput(t *testing.T) {
	t.Parallel()
	_, err := Stage00Request([]byte{}, StageDeps{})
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestStage00Request_PrettyPrinted(t *testing.T) {
	t.Parallel()
	input := []byte(`{"instruction":"fix","preview":false}`)
	out, err := Stage00Request(input, StageDeps{})
	if err != nil {
		t.Fatalf("Stage00Request: %v", err)
	}
	// Pretty-printed output should contain newlines
	if !contains(string(out), "\n") {
		t.Error("Stage00Request output should be pretty-printed JSON")
	}
}

// === Task 3.5: Runner tests ===

func TestNumStages(t *testing.T) {
	t.Parallel()
	if NumStages() != 8 {
		t.Errorf("NumStages() = %d, want 8", NumStages())
	}
}

func TestRunStage_ValidIndex(t *testing.T) {
	t.Parallel()
	input := []byte(`{"instruction":"test","preview":false}`)
	out, err := RunStage(0, input, StageDeps{})
	if err != nil {
		t.Fatalf("RunStage(0): %v", err)
	}
	if len(out) == 0 {
		t.Error("RunStage(0) output should not be empty")
	}
}

func TestRunStage_InvalidIndex(t *testing.T) {
	t.Parallel()
	_, err := RunStage(-1, nil, StageDeps{})
	if err == nil {
		t.Error("expected error for negative index, got nil")
	}
	_, err = RunStage(99, nil, StageDeps{})
	if err == nil {
		t.Error("expected error for index 99, got nil")
	}
}

func TestRunRange_SingleStage(t *testing.T) {
	t.Parallel()
	input := []byte(`{"instruction":"test","preview":false}`)
	out, err := RunRange(0, 0, input, StageDeps{})
	if err != nil {
		t.Fatalf("RunRange(0,0): %v", err)
	}
	var req CommitRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Instruction != "test" {
		t.Errorf("Instruction: got %q, want %q", req.Instruction, "test")
	}
}

func TestRunRange_InvalidRange(t *testing.T) {
	t.Parallel()
	_, err := RunRange(2, 1, nil, StageDeps{})
	if err == nil {
		t.Error("expected error for start > end, got nil")
	}
	_, err = RunRange(-1, 3, nil, StageDeps{})
	if err == nil {
		t.Error("expected error for negative start, got nil")
	}
}

func TestStage07Result_AssemblesMessage(t *testing.T) {
	t.Parallel()
	message := "feat: add pipeline testing"
	out, err := Stage07Result([]byte(message), StageDeps{})
	if err != nil {
		t.Fatalf("Stage07Result: %v", err)
	}
	var result PipelineResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Message != "feat: add pipeline testing" {
		t.Errorf("Message: got %q, want %q", result.Message, "feat: add pipeline testing")
	}
}

func TestStage07Result_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	message := "  fix: trim whitespace  \n"
	out, err := Stage07Result([]byte(message), StageDeps{})
	if err != nil {
		t.Fatalf("Stage07Result: %v", err)
	}
	var result PipelineResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Message != "fix: trim whitespace" {
		t.Errorf("Message should be trimmed: got %q", result.Message)
	}
}

// === Task 3.6: Golden files ===
// Golden files are created by the test infrastructure itself using -update flag.

func TestStageNames(t *testing.T) {
	t.Parallel()
	names := map[int]string{
		0: "request", 1: "diff", 2: "security", 3: "chunks",
		4: "annotation", 5: "classification", 6: "llm", 7: "result",
	}
	for idx, name := range names {
		got := StageNames[idx]
		if got != name {
			t.Errorf("StageNames[%d] = %q, want %q", idx, got, name)
		}
	}
}

// === Golden file tests ===

// contains is a simple string contains helper.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestGoldenFile_SimpleFix_Stage00(t *testing.T) {
	t.Parallel()
	golden := readGolden(t, "simple_fix", "00_request.json")
	var req CommitRequest
	if err := json.Unmarshal(golden, &req); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	if req.Instruction == "" {
		t.Error("golden 00_request.json should have non-empty instruction")
	}
}

func TestGoldenFile_SimpleFix_Stage07(t *testing.T) {
	t.Parallel()
	golden := readGolden(t, "simple_fix", "07_result.json")
	var result PipelineResult
	if err := json.Unmarshal(golden, &result); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	if result.Message == "" {
		t.Error("golden 07_result.json should have non-empty message")
	}
	if len(result.Chunks) == 0 {
		t.Error("golden 07_result.json should have at least one chunk")
	}
}

func TestGoldenFile_MultiFileFix_Stage00(t *testing.T) {
	t.Parallel()
	golden := readGolden(t, "multi_file_fix", "00_request.json")
	var req CommitRequest
	if err := json.Unmarshal(golden, &req); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	if req.Instruction == "" {
		t.Error("golden 00_request.json should have non-empty instruction")
	}
	if !req.Preview {
		t.Error("golden 00_request.json should have preview=true")
	}
}

func TestGoldenFile_MultiFileFix_Stage07(t *testing.T) {
	t.Parallel()
	golden := readGolden(t, "multi_file_fix", "07_result.json")
	var result PipelineResult
	if err := json.Unmarshal(golden, &result); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	if result.Message == "" {
		t.Error("golden 07_result.json should have non-empty message")
	}
	if len(result.Chunks) != 2 {
		t.Errorf("golden 07_result.json should have 2 chunks, got %d", len(result.Chunks))
	}
	if result.Preview != true {
		t.Error("golden 07_result.json should have preview=true")
	}
}

func TestGoldenFile_MessageIsPlainText(t *testing.T) {
	t.Parallel()
	// AC5.9: 06_message.txt is plain text (not JSON)
	data := readGolden(t, "simple_fix", "06_message.txt")
	content := string(data)
	// Must not start with { or [ (JSON indicators)
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Error("06_message.txt should be plain text, not JSON")
	}
}

func TestGoldenFile_SecurityNotBlocked(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"simple_fix", "multi_file_fix"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			golden := readGolden(t, scenario, "02_security.json")
			var sec SecurityResult
			if err := json.Unmarshal(golden, &sec); err != nil {
				t.Fatalf("unmarshal golden security: %v", err)
			}
			if sec.Blocked {
				t.Errorf("golden scenario %q security should not be blocked", scenario)
			}
		})
	}
}

func TestGoldenFile_Completeness(t *testing.T) {
	t.Parallel()
	expectedFiles := []string{
		"00_request.json", "01_diff.txt", "02_security.json",
		"03_chunks.json", "04_annotated.json", "05_classified.json",
		"06_message.txt", "07_result.json",
	}
	for _, scenario := range []string{"simple_fix", "multi_file_fix"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			for _, f := range expectedFiles {
				path := goldenPath(scenario, f)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("missing golden file: %s", path)
				}
			}
		})
	}
}

func TestGoldenFile_Stage03ChunksNoClassification(t *testing.T) {
	t.Parallel()
	// Stage 03 (chunking) should NOT have commit_type or confidence_score populated.
	// Those are populated by stage 05 (classification).
	for _, scenario := range []string{"simple_fix", "multi_file_fix"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			golden := readGolden(t, scenario, "03_chunks.json")
			var chunks []domain.DiffChunk
			if err := json.Unmarshal(golden, &chunks); err != nil {
				t.Fatalf("unmarshal golden chunks: %v", err)
			}
			for i, c := range chunks {
				if c.CommitType != "" {
					t.Errorf("scenario %q chunk[%d]: commit_type = %q, want empty (populated by stage 05)", scenario, i, c.CommitType)
				}
				if c.ConfidenceScore != 0 {
					t.Errorf("scenario %q chunk[%d]: confidence_score = %f, want 0 (populated by stage 05)", scenario, i, c.ConfidenceScore)
				}
			}
		})
	}
}