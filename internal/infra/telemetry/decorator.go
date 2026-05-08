package telemetry

import (
	"fmt"
	"strings"
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

func (d *TelemetryDecorator) record(start time.Time, op string, prompt interface{}, response interface{}, err error) {
	latency := time.Since(start)
	call := LLMCall{
		Timestamp: start,
		Operation: op,
		Latency:   latency,
		Prompt:    fmt.Sprintf("%v", prompt),
		Response:  fmt.Sprintf("%v", response),
		Success:   err == nil,
	}
	if err != nil {
		call.Error = err.Error()
	}
	d.collector.RecordLLMCall(call)
}

func (d *TelemetryDecorator) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	start := time.Now()
	res, err := d.base.GenerateChunkMessage(chunk)
	d.record(start, "GenerateChunkMessage", chunk.Diff, res, err)
	return res, err
}

func (d *TelemetryDecorator) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	start := time.Now()
	res, err := d.base.DecideCommit(instruction, gitStatus, untracked, modified, deleted)
	prompt := fmt.Sprintf("Instruction: %s\nStatus: %s\nUntracked: %s\nModified: %s\nDeleted: %s",
		instruction, gitStatus, untracked, modified, deleted)
	// CommitIntent doesn't have an Action field, it's a struct with filters.
	d.record(start, "DecideCommit", prompt, fmt.Sprintf("%v", res), err)
	return res, err
}

func (d *TelemetryDecorator) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	start := time.Now()
	res, err := d.base.InterpretGitOp(op, instruction, context)
	prompt := fmt.Sprintf("Op: %s\nInstruction: %s\nContext: %v", op, instruction, context)
	d.record(start, "InterpretGitOp", prompt, res, err)
	return res, err
}

func (d *TelemetryDecorator) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	start := time.Now()
	res, err := d.base.VerifySecrets(diff, findings)
	prompt := fmt.Sprintf("Diff: %s\nFindings: %v", diff, findings)
	d.record(start, "VerifySecrets", prompt, res, err)
	return res, err
}

func (d *TelemetryDecorator) AuditBinaryContent(filename, content string) (bool, error) {
	start := time.Now()
	res, err := d.base.AuditBinaryContent(filename, content)
	prompt := fmt.Sprintf("File: %s\nContent (sample): %s", filename, truncateContent(content, 500))
	d.record(start, "AuditBinaryContent", prompt, res, err)
	return res, err
}

func (d *TelemetryDecorator) GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	start := time.Now()
	res, err := d.base.GenerateChangelog(commits, previousChangelog, outputFile)
	prompt := fmt.Sprintf("Commits: %s\nPrevious: %s", commits, previousChangelog)
	d.record(start, "GenerateChangelog", prompt, res, err)
	return res, err
}

func (d *TelemetryDecorator) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	start := time.Now()
	res, err := d.base.RegenerateMessage(previousMessages, feedback, chunks)
	prompt := fmt.Sprintf("Previous: %v\nFeedback: %s", previousMessages, feedback)
	d.record(start, "RegenerateMessage", prompt, strings.Join(res, " | "), err)
	return res, err
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

func (d *TelemetryDecorator) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) {
	start := time.Now()
	res, err := d.base.ProjectInit(repoRoot)
	prompt := fmt.Sprintf("RepoRoot: %s", repoRoot)
	d.record(start, "ProjectInit", prompt, fmt.Sprintf("%v", res), err)
	return res, err
}

func truncateContent(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... [truncated]"
}
