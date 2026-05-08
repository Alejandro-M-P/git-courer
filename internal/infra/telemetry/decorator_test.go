package telemetry

import (
	"errors"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

type mockLLM struct {
	available bool
	generate  func(chunk domain.DiffChunk) (string, error)
}

func (m *mockLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return m.generate(chunk)
}

func (m *mockLLM) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return domain.CommitIntent{}, nil
}

func (m *mockLLM) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	return nil, nil
}

func (m *mockLLM) SetRetryContext(previousMessage string) {}
func (m *mockLLM) ClearRetryContext()                     {}
func (m *mockLLM) IsAvailable() bool                      { return m.available }
func (m *mockLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (m *mockLLM) AuditBinaryContent(filename, content string) (bool, error) { return false, nil }
func (m *mockLLM) GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	return nil, nil
}
func (m *mockLLM) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return nil, nil
}
func (m *mockLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) { return nil, nil }

type mockCollector struct {
	calls []LLMCall
}

func (m *mockCollector) RecordLLMCall(call LLMCall) {
	m.calls = append(m.calls, call)
}
func (m *mockCollector) RecordMetric(name string, value float64, labels map[string]string) {}
func (m *mockCollector) Flush() error                                                      { return nil }

func TestTelemetryDecorator(t *testing.T) {
	mock := &mockLLM{
		available: true,
		generate: func(chunk domain.DiffChunk) (string, error) {
			time.Sleep(10 * time.Millisecond)
			return "feat: test", nil
		},
	}
	collector := &mockCollector{}
	decorator := NewTelemetryDecorator(mock, collector)

	t.Run("Record Success Call", func(t *testing.T) {
		_, err := decorator.GenerateChunkMessage(domain.DiffChunk{Diff: "test diff"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(collector.calls) != 1 {
			t.Fatalf("expected 1 call recorded, got %d", len(collector.calls))
		}

		call := collector.calls[0]
		if call.Operation != "GenerateChunkMessage" {
			t.Errorf("expected operation GenerateChunkMessage, got %s", call.Operation)
		}
		if call.Latency < 10*time.Millisecond {
			t.Errorf("expected latency >= 10ms, got %v", call.Latency)
		}
		if !call.Success {
			t.Error("expected success=true")
		}
		if call.Response != "feat: test" {
			t.Errorf("expected response 'feat: test', got %s", call.Response)
		}
	})

	t.Run("Record Error Call", func(t *testing.T) {
		mock.generate = func(chunk domain.DiffChunk) (string, error) {
			return "", errors.New("llm error")
		}
		collector.calls = nil

		_, err := decorator.GenerateChunkMessage(domain.DiffChunk{Diff: "test diff"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if len(collector.calls) != 1 {
			t.Fatalf("expected 1 call recorded, got %d", len(collector.calls))
		}

		call := collector.calls[0]
		if call.Success {
			t.Error("expected success=false")
		}
		if call.Error != "llm error" {
			t.Errorf("expected error 'llm error', got %s", call.Error)
		}
	})

	t.Run("Record DecideCommit Call", func(t *testing.T) {
		collector.calls = nil
		_, _ = decorator.DecideCommit("instruction", "status", "untracked", "modified", "deleted")

		if len(collector.calls) != 1 {
			t.Fatalf("expected 1 call recorded, got %d", len(collector.calls))
		}

		call := collector.calls[0]
		if call.Operation != "DecideCommit" {
			t.Errorf("expected operation DecideCommit, got %s", call.Operation)
		}
	})
}
