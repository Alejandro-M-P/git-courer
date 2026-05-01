package telemetry

// TelemetryCollector defines the port for collecting telemetry data.
type TelemetryCollector interface {
	// RecordLLMCall records a single interaction with an LLM.
	RecordLLMCall(call LLMCall)
	// RecordMetric records a generic numeric metric.
	RecordMetric(name string, value float64, labels map[string]string)
	// Flush ensures all buffered telemetry is persisted.
	Flush() error
}

// ReportGenerator defines the port for generating telemetry reports.
type ReportGenerator interface {
	// GenerateReport processes telemetry data and produces a report.
	GenerateReport(telemetryDir string) error
}
