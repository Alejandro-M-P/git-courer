package telemetry

import (
	"testing"
)

func TestTelemetryInterfaces(t *testing.T) {
	var _ interface {
		RecordLLMCall(call LLMCall)
		RecordMetric(name string, value float64, labels map[string]string)
		Flush() error
	} = (TelemetryCollector)(nil)

	var _ interface {
		GenerateReport(telemetryDir string) error
	} = (ReportGenerator)(nil)
}
