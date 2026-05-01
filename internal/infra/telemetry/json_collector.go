package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type JSONCollector struct {
	dir  string
	file *os.File
}

func NewJSONCollector(dir string) (*JSONCollector, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create telemetry directory: %w", err)
	}

	filename := fmt.Sprintf("telemetry_%d.jsonl", os.Getpid())
	path := filepath.Join(dir, filename)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open telemetry file: %w", err)
	}

	return &JSONCollector{
		dir:  dir,
		file: file,
	}, nil
}

func (c *JSONCollector) RecordLLMCall(call LLMCall) {
	data, err := json.Marshal(call)
	if err != nil {
		return
	}
	_, _ = c.file.Write(append(data, '\n'))
}

func (c *JSONCollector) RecordMetric(name string, value float64, labels map[string]string) {
	metric := Metric{
		Name:   name,
		Value:  value,
		Labels: labels,
	}
	data, err := json.Marshal(metric)
	if err != nil {
		return
	}
	_, _ = c.file.Write(append(data, '\n'))
}

func (c *JSONCollector) Flush() error {
	return c.file.Sync()
}

func (c *JSONCollector) Close() error {
	return c.file.Close()
}
