package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Alejandro-M-P/git-courer/internal/infra/telemetry"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var telemetryDir string
	if len(args) > 0 {
		telemetryDir = args[0]
	} else {
		telemetryDir = filepath.Join(".gcourer", "telemetry")
	}

	// Ensure directory exists
	if err := os.MkdirAll(telemetryDir, 0755); err != nil {
		return fmt.Errorf("failed to create telemetry directory: %w", err)
	}

	generator := telemetry.NewConsoleReportGenerator()
	if err := generator.GenerateReport(telemetryDir); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}
	return nil
}
