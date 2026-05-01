package telemetry

import (
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

type TelemetryDecorator struct {
	base      ports.LLM
	collector TelemetryCollector
}

func NewTelemetryDecorator(base ports.LLM, collector TelemetryCollector) *TelemetryDecorator {
	return &TelemetryDecorator{
		base:      base,
		collector: collector,
	}
}

func (d *TelemetryDecorator) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	start := time.Now()
	res, err := d.base.GenerateChunkMessage(chunk)
	latency := time.Since(start)

	call := LLMCall{
		Timestamp: start,
		Operation: "GenerateChunkMessage",
		Latency:   latency,
		Prompt:    chunk.Diff,
		Response:  res,
		Success:   err == nil,
	}
	if err != nil {
		call.Error = err.Error()
	}

	d.collector.RecordLLMCall(call)
	return res, err
}

func (d *TelemetryDecorator) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return d.base.DecideCommit(instruction, gitStatus, untracked, modified, deleted)
}

func (d *TelemetryDecorator) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	return d.base.InterpretGitOp(op, instruction, context)
}

func (d *TelemetryDecorator) SetRetryContext(previousMessage string) {
	d.base.SetRetryContext(previousMessage)
}

func (d *TelemetryDecorator) ClearRetryContext() {
	d.base.ClearRetryContext()
}

func (d *TelemetryDecorator) IsAvailable() bool {
	return d.base.IsAvailable()
}

func (d *TelemetryDecorator) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return d.base.VerifySecrets(diff, findings)
}

func (d *TelemetryDecorator) AuditBinaryContent(filename, content string) (bool, error) {
	return d.base.AuditBinaryContent(filename, content)
}

func (d *TelemetryDecorator) GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	return d.base.GenerateChangelog(commits, previousChangelog, outputFile)
}

func (d *TelemetryDecorator) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	return d.base.RegenerateMessage(previousMessages, feedback, chunks)
}
