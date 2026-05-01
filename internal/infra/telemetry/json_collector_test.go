package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONCollector(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	collector, err := NewJSONCollector(tmpDir)
	if err != nil {
		t.Fatalf("failed to create JSONCollector: %v", err)
	}

	t.Run("RecordLLMCall", func(t *testing.T) {
		call := LLMCall{
			Timestamp: time.Now().Truncate(time.Second),
			Operation: "TestOp",
			Model:     "TestModel",
			Latency:   100 * time.Millisecond,
			Prompt:    "TestPrompt",
			Response:  "TestResponse",
			Success:   true,
		}

		collector.RecordLLMCall(call)
		if err := collector.Flush(); err != nil {
			t.Fatalf("failed to flush: %v", err)
		}

		// Verify file exists and content is correct
		files, _ := filepath.Glob(filepath.Join(tmpDir, "*.jsonl"))
		if len(files) == 0 {
			t.Fatal("expected jsonl file to be created")
		}

		file, err := os.Open(files[0])
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		defer file.Close()

		var readCall LLMCall
		if err := json.NewDecoder(file).Decode(&readCall); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}

		if readCall.Operation != call.Operation || readCall.Model != call.Model {
			t.Errorf("decoded call mismatch: %+v != %+v", readCall, call)
		}
	})

	t.Run("RecordMetric", func(t *testing.T) {
		metric := Metric{
			Name:   "test_metric",
			Value:  42.0,
			Labels: map[string]string{"foo": "bar"},
		}

		collector.RecordMetric(metric.Name, metric.Value, metric.Labels)
		if err := collector.Flush(); err != nil {
			t.Fatalf("failed to flush: %v", err)
		}

		files, _ := filepath.Glob(filepath.Join(tmpDir, "*.jsonl"))
		file, _ := os.Open(files[0])
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var lastLine string
		for scanner.Scan() {
			lastLine = scanner.Text()
		}

		var readMetric Metric
		if err := json.Unmarshal([]byte(lastLine), &readMetric); err != nil {
			t.Fatalf("failed to unmarshal last line: %v", err)
		}

		if readMetric.Name != metric.Name || readMetric.Value != metric.Value {
			t.Errorf("decoded metric mismatch: %+v != %+v", readMetric, metric)
		}
	})
}
