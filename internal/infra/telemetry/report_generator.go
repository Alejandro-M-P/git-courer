package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... [truncated]"
}

// ConsoleReportGenerator implements the ReportGenerator interface for console output.
type ConsoleReportGenerator struct{}

// NewConsoleReportGenerator creates a new ConsoleReportGenerator.
func NewConsoleReportGenerator() *ConsoleReportGenerator {
	return &ConsoleReportGenerator{}
}

// GenerateReport reads telemetry data and displays the full report.
// In a real terminal: piped through `less -R` for scroll + copy-paste.
// In CI / pipe: printed directly to stdout.
// Always writes a plain-text copy to <telemetryDir>/report.txt.
func (g *ConsoleReportGenerator) GenerateReport(telemetryDir string) error {
	files, err := os.ReadDir(telemetryDir)
	if err != nil {
		return fmt.Errorf("failed to read telemetry dir: %w", err)
	}

	var calls []LLMCall
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".jsonl" {
			path := filepath.Join(telemetryDir, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", f.Name(), err)
			}
			for _, line := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				var call LLMCall
				if err := json.Unmarshal([]byte(line), &call); err != nil {
					return fmt.Errorf("failed to unmarshal line in %s: %w", f.Name(), err)
				}
				calls = append(calls, call)
			}
		}
	}

	sort.Slice(calls, func(i, j int) bool {
		return calls[i].Timestamp.Before(calls[j].Timestamp)
	})

	content := buildReportContent(calls)

	// Plain-text copy for easy grep / share
	reportPath := filepath.Join(telemetryDir, "report.txt")
	_ = os.WriteFile(reportPath, []byte(stripANSI(content)), 0644)

	if isTerminal() {
		if err := pipeLess(content); err == nil {
			fmt.Fprintf(os.Stderr, "\nSaved: %s\n", reportPath)
			return nil
		}
	}
	fmt.Print(content)
	fmt.Fprintf(os.Stderr, "\nSaved: %s\n", reportPath)
	return nil
}

// buildReportContent generates all sections as a single styled string.
func buildReportContent(calls []LLMCall) string {
	if len(calls) == 0 {
		return titleStyle.Render("LLM TELEMETRY") + "\n\nNo calls recorded.\n"
	}

	var totalLatency float64
	successCount := 0
	evaluator := NewQualityEvaluator()
	var totalQuality float64
	evalCount := 0
	var failures []LLMCall

	for _, call := range calls {
		totalLatency += call.Latency.Seconds()
		if call.Success {
			successCount++
			if call.Operation == "GenerateCommitMessage" || call.Operation == "GenerateChunkMessage" {
				res := evaluator.EvaluateCommitMessage(call.Response)
				totalQuality += res.Score
				evalCount++
			}
		} else {
			failures = append(failures, call)
		}
	}

	successRate := float64(successCount) / float64(len(calls)) * 100
	avgLatency := totalLatency / float64(len(calls))
	avgQuality := 0.0
	if evalCount > 0 {
		avgQuality = totalQuality / float64(evalCount)
	}

	rateStyle := successStyle
	if successRate < 70 {
		rateStyle = errorStyle
	} else if successRate < 90 {
		rateStyle = warningStyle
	}

	sep := labelStyle.Render(strings.Repeat("─", 70))
	var sb strings.Builder

	// ── Header ──────────────────────────────────────────────────────────────
	sb.WriteString(titleStyle.Render("LLM TELEMETRY"))
	sb.WriteString("\n" + sep + "\n\n")

	// ── SUMMARY DASHBOARD ───────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("SUMMARY DASHBOARD"))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render(fmt.Sprintf("  Success Rate:  %s\n",
		rateStyle.Render(fmt.Sprintf("%.1f%%  (%d/%d)", successRate, successCount, len(calls))))))
	sb.WriteString(labelStyle.Render(fmt.Sprintf("  Avg Latency:   %.2fs\n", avgLatency)))
	sb.WriteString(labelStyle.Render(fmt.Sprintf("  Avg Quality:   %.2f  (n=%d)\n", avgQuality, evalCount)))
	sb.WriteString("\n")

	// ── FAILURES ─────────────────────────────────────────────────────────────
	if len(failures) > 0 {
		sb.WriteString(errorHeader.Render(fmt.Sprintf("FAILURES  (%d)", len(failures))))
		sb.WriteString("\n" + sep + "\n")
		for _, call := range failures {
			sb.WriteString(errorStyle.Render(fmt.Sprintf(
				"  [%s] %s (%s) %.2fs\n",
				call.Timestamp.Format("15:04:05"),
				call.Operation,
				call.Model,
				call.Latency.Seconds(),
			)))
			if call.Error != "" {
				sb.WriteString(errorStyle.Render("    Error:    "+call.Error) + "\n")
			}
			if call.Prompt != "" {
				sb.WriteString(labelStyle.Render("    Prompt:   "+truncate(call.Prompt, 500)) + "\n")
			}
			if call.Response != "" {
				sb.WriteString(labelStyle.Render("    Response: "+truncate(call.Response, 500)) + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// ── LLM CALLS ────────────────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("LLM CALLS"))
	sb.WriteString("\n" + sep + "\n")
	sb.WriteString(labelStyle.Render(fmt.Sprintf("  %-10s  %-30s  %-8s  %-6s  %-7s\n",
		"Time", "Operation", "Latency", "Status", "Quality")))
	sb.WriteString(labelStyle.Render("  " + strings.Repeat("─", 66) + "\n"))

	for _, call := range calls {
		status := successStyle.Render("OK  ")
		if !call.Success {
			status = errorStyle.Render("FAIL")
		}
		qualityStr := labelStyle.Render("-")
		if call.Success && (call.Operation == "GenerateCommitMessage" || call.Operation == "GenerateChunkMessage") {
			res := evaluator.EvaluateCommitMessage(call.Response)
			qualityStr = fmt.Sprintf("%.2f", res.Score)
		}
		sb.WriteString(labelStyle.Render(fmt.Sprintf("  %-10s  %-30s  %-8s  ",
			call.Timestamp.Format("15:04:05"),
			truncate(call.Operation, 30),
			fmt.Sprintf("%.2fs", call.Latency.Seconds()),
		)) + status + "  " + qualityStr + "\n")
	}
	sb.WriteString("\n")

	// ── LLM CALL DETAILS ────────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("LLM CALL DETAILS"))
	sb.WriteString("\n" + sep + "\n")
	for _, call := range calls {
		status := successStyle.Render("OK")
		if !call.Success {
			status = errorStyle.Render("FAIL")
		}
		sb.WriteString(labelStyle.Render(fmt.Sprintf("  [%s] %s (%s) %.2fs — ",
			call.Timestamp.Format("15:04:05"),
			call.Operation,
			call.Model,
			call.Latency.Seconds(),
		)) + status + "\n")
		if call.Prompt != "" {
			sb.WriteString(labelStyle.Render("    Prompt:   "+truncate(call.Prompt, 500)) + "\n")
		}
		if call.Response != "" {
			sb.WriteString(labelStyle.Render("    Response: "+truncate(call.Response, 500)) + "\n")
		}
		if call.Error != "" {
			sb.WriteString(errorStyle.Render("    Error:    "+call.Error) + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// isTerminal reports whether stdout is a real terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// pipeLess pipes content through `less -R` for scrollable, copy-pasteable viewing.
func pipeLess(content string) error {
	path, err := exec.LookPath("less")
	if err != nil {
		return err
	}
	cmd := exec.Command(path, "-R") // -R: keep ANSI color codes
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
