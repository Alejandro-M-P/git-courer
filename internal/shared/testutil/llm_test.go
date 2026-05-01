package testutil_test

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

func TestRequireOllama(t *testing.T) {
	// This should compile once Task 2.1 is done
	llm := testutil.RequireOllama(t)
	if llm == nil {
		t.Fatal("expected llm to be not nil")
	}
}

func TestGetTelemetryCollector(t *testing.T) {
	collector := testutil.GetTelemetryCollector()
	// It can be nil if TELEMETRY=0
	_ = collector
}
