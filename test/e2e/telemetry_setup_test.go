//go:build e2e

package e2e

import (
	"os"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/infra/telemetry"
)

func TestTelemetrySetup(t *testing.T) {
	// Ensure TELEMETRY=1 for this test
	os.Setenv("TELEMETRY", "1")
	defer os.Unsetenv("TELEMETRY")

	llm := requireOllama(t)

	_, ok := llm.(*telemetry.TelemetryDecorator)
	if !ok {
		t.Errorf("expected LLM to be *telemetry.TelemetryDecorator when TELEMETRY=1, got %T", llm)
	}
}
