package prreview

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const testTimeout = 120 * time.Second

// runTests executes the test command with a 120s timeout and parses the output.
func runTests(ctx context.Context, command string) TestResult {
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	isGoTest := strings.HasPrefix(strings.TrimSpace(command), "go test")
	execCmd := buildCommand(ctx, command, isGoTest)

	output, execErr := execCmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return TestResult{
			Status: "timeout",
			Output: fmt.Sprintf("test command timed out after %v", testTimeout),
		}
	}

	exitCode := 0
	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return TestResult{
				Status: "fail",
				Output: fmt.Sprintf("failed to run test command: %v", execErr),
			}
		}
	}

	outputStr := string(output)

	if isGoTest {
		result := parseGoTestJSON(outputStr)
		if result.Status == "fail" {
			return result
		}
		if exitCode != 0 {
			result.Status = "fail"
		}
		return result
	}

	if exitCode != 0 {
		truncated := truncateString(outputStr, 500)
		return TestResult{
			Status: "fail",
			Output: truncated,
		}
	}

	return TestResult{
		Status: "pass",
		Output: truncateString(outputStr, 500),
	}
}

func buildCommand(ctx context.Context, command string, isGoTest bool) *exec.Cmd {
	parts := strings.Fields(command)
	if isGoTest && !hasJSONFlag(parts) {
		parts = append(parts, "-json")
	}
	return exec.CommandContext(ctx, parts[0], parts[1:]...)
}

func hasJSONFlag(parts []string) bool {
	for _, p := range parts {
		if p == "-json" {
			return true
		}
	}
	return false
}

func parseGoTestJSON(output string) TestResult {
	var failingTests []FailingTest
	var totalTests int
	failedPkgs := make(map[string]bool)
	testOutputs := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		action, _ := entry["Action"].(string)
		pkg, _ := entry["Package"].(string)
		testName, _ := entry["Test"].(string)

		switch action {
		case "pass":
			if testName != "" {
				totalTests++
			}
		case "fail":
			if testName != "" {
				totalTests++
				key := pkg + "/" + testName
				failingTests = append(failingTests, FailingTest{
					Package:  pkg,
					TestName: testName,
					Output:   truncateString(testOutputs[key], 500),
				})
			} else {
				failedPkgs[pkg] = true
			}
		case "output":
			if testName != "" {
				key := pkg + "/" + testName
				text, _ := entry["Output"].(string)
				testOutputs[key] += text + "\n"
			}
		}
	}

	if len(failingTests) > 0 || len(failedPkgs) > 0 {
		return TestResult{
			Status:       "fail",
			Total:        totalTests,
			Failed:       len(failingTests) + len(failedPkgs),
			FailingTests: failingTests,
			Output:       truncateString(output, 500),
		}
	}

	return TestResult{
		Status: "pass",
		Total:  totalTests,
		Output: truncateString(output, 500),
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
