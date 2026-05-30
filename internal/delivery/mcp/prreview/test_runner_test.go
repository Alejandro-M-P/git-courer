package prreview

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunTests_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test command execution in short mode")
	}

	ctx := context.Background()
	result := runTests(ctx, "echo test_pass")

	assert.Equal(t, "pass", result.Status)
}

func TestRunTests_FailExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test command execution in short mode")
	}

	ctx := context.Background()
	result := runTests(ctx, "false")

	assert.Equal(t, "fail", result.Status)
}

func TestParseGoTestJSON_Pass(t *testing.T) {
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"pkg/example","Test":"TestFoo"}
{"Time":"2024-01-01T00:00:01Z","Action":"pass","Package":"pkg/example","Test":"TestBar"}`

	result := parseGoTestJSON(input)
	assert.Equal(t, "pass", result.Status)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 0, result.Failed)
	assert.Empty(t, result.FailingTests)
}

func TestParseGoTestJSON_Fail(t *testing.T) {
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"pkg/example","Test":"TestFoo"}
{"Time":"2024-01-01T00:00:01Z","Action":"output","Package":"pkg/example","Test":"TestBar","Output":"expected true, got false\n"}
{"Time":"2024-01-01T00:00:02Z","Action":"fail","Package":"pkg/example","Test":"TestBar"}`

	result := parseGoTestJSON(input)
	assert.Equal(t, "fail", result.Status)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.FailingTests, 1)
	assert.Equal(t, "TestBar", result.FailingTests[0].TestName)
	assert.Equal(t, "pkg/example", result.FailingTests[0].Package)
	assert.Contains(t, result.FailingTests[0].Output, "expected true, got false")
}

func TestParseGoTestJSON_Empty(t *testing.T) {
	result := parseGoTestJSON("")
	assert.Equal(t, "pass", result.Status)
	assert.Equal(t, 0, result.Total)
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "abc", truncateString("abc", 10))
	assert.Equal(t, "abc", truncateString("abc", 3))
	assert.Equal(t, "ab...", truncateString("abcdef", 5))
	assert.Equal(t, "a", truncateString("abcdef", 1))
}

func TestBuildCommand_AddsJSONFlag(t *testing.T) {
	ctx := context.Background()
	cmd := buildCommand(ctx, "go test ./...", true)
	assert.Contains(t, cmd.Args, "-json")
}

func TestBuildCommand_NoDuplicateJSONFlag(t *testing.T) {
	ctx := context.Background()
	cmd := buildCommand(ctx, "go test -json ./...", true)
	count := 0
	for _, arg := range cmd.Args {
		if arg == "-json" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestBuildCommand_NonGoTest(t *testing.T) {
	ctx := context.Background()
	cmd := buildCommand(ctx, "make test", false)
	assert.Equal(t, "make", cmd.Args[0])
	assert.NotContains(t, cmd.Args, "-json")
}

func TestRunTests_NonGoCommandFail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test command execution in short mode")
	}

	ctx := context.Background()
	result := runTests(ctx, "sh -c 'echo failed; exit 1'")

	assert.Equal(t, "fail", result.Status)
	assert.Contains(t, result.Output, "failed")
}

func TestHasJSONFlag(t *testing.T) {
	assert.True(t, hasJSONFlag([]string{"go", "test", "-json", "./..."}))
	assert.False(t, hasJSONFlag([]string{"go", "test", "./..."}))
}

func TestParseGoTestJSON_PackageLevelFail(t *testing.T) {
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"pkg/broken"}`

	result := parseGoTestJSON(input)
	assert.Equal(t, "fail", result.Status)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.FailingTests)
}

func TestParseGoTestJSON_TruncatedOutput(t *testing.T) {
	longOutput := strings.Repeat("x", 1000)
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"pkg/example","Test":"TestFoo","Output":"` + longOutput + `"}
{"Time":"2024-01-01T00:00:01Z","Action":"fail","Package":"pkg/example","Test":"TestFoo"}`

	result := parseGoTestJSON(input)
	assert.Equal(t, "fail", result.Status)
	assert.Len(t, result.FailingTests, 1)
	assert.LessOrEqual(t, len(result.FailingTests[0].Output), 503)
}
