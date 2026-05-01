package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	summaryStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			MarginRight(2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	tableStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	detailsStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			MarginBottom(1)
)

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

// GenerateReport reads telemetry data from the directory and prints a report to stdout.
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

			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
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

	// Sort calls by timestamp
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].Timestamp.Before(calls[j].Timestamp)
	})

	fmt.Println(headerStyle.Render("GIT-COURER TELEMETRY REPORT"))

	if len(calls) == 0 {
		fmt.Println("No telemetry data found.")
		return nil
	}

	// 1. Summary Dashboard
	var totalLatency float64
	successCount := 0
	evaluator := NewQualityEvaluator()
	var totalQuality float64
	evalCount := 0

	for _, call := range calls {
		totalLatency += call.Latency.Seconds()
		if call.Success {
			successCount++
			if call.Operation == "GenerateCommitMessage" || call.Operation == "GenerateChunkMessage" {
				res := evaluator.EvaluateCommitMessage(call.Response)
				totalQuality += res.Score
				evalCount++
			}
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

	summary := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("SUMMARY DASHBOARD"),
		fmt.Sprintf("Total LLM Calls: %d", len(calls)),
		fmt.Sprintf("Success Rate:    %s", rateStyle.Render(fmt.Sprintf("%.2f%%", successRate))),
		fmt.Sprintf("Avg Latency:     %.2fs", avgLatency),
		fmt.Sprintf("Avg Quality:     %.2f (n=%d)", avgQuality, evalCount),
	)

	fmt.Println(summaryStyle.Render(summary))

	// 2. LLM Calls Table
	columns := []table.Column{
		{Title: "Time", Width: 10},
		{Title: "Operation", Width: 25},
		{Title: "Model", Width: 15},
		{Title: "Latency", Width: 10},
		{Title: "Result", Width: 8},
		{Title: "Quality", Width: 8},
	}

	rows := make([]table.Row, len(calls))
	for i, call := range calls {
		result := successStyle.Render("OK")
		if !call.Success {
			result = errorStyle.Render("FAIL")
		}

		qualityStr := "-"
		if call.Success && (call.Operation == "GenerateCommitMessage" || call.Operation == "GenerateChunkMessage") {
			res := evaluator.EvaluateCommitMessage(call.Response)
			qualityStr = fmt.Sprintf("%.2f", res.Score)
		}

		rows[i] = table.Row{
			call.Timestamp.Format("15:04:05"),
			call.Operation,
			call.Model,
			fmt.Sprintf("%.2fs", call.Latency.Seconds()),
			result,
			qualityStr,
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(len(calls)+1),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	fmt.Println(titleStyle.Render("LLM CALLS"))
	fmt.Println(tableStyle.Render(t.View()))

	// 3. Detailed Interaction Log
	fmt.Println("\n" + titleStyle.Render("LLM CALL DETAILS"))
	for _, call := range calls {
		status := successStyle.Render("SUCCESS")
		if !call.Success {
			status = errorStyle.Render("FAILED")
		}

		header := fmt.Sprintf("[%s] %s (%s) - %s",
			call.Timestamp.Format("15:04:05"),
			call.Operation,
			call.Model,
			status)

		promptTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Render("PROMPT:")
		responseTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Render("RESPONSE:")

		content := lipgloss.JoinVertical(lipgloss.Left,
			header,
			"",
			promptTitle,
			truncate(call.Prompt, 500),
			"",
			responseTitle,
			truncate(call.Response, 500),
		)

		if call.Error != "" {
			errorTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("ERROR:")
			content = lipgloss.JoinVertical(lipgloss.Left, content, "", errorTitle, call.Error)
		}

		fmt.Println(detailsStyle.Render(content))
	}

	return nil
}
