package telemetry

import "time"

// LLMCall represents a single interaction with an LLM provider.
type LLMCall struct {
	Timestamp time.Time     `json:"timestamp"`
	Operation string        `json:"operation"` // e.g., "GenerateCommitMessage", "DecideCommit"
	Model     string        `json:"model"`
	Latency   time.Duration `json:"latency"`
	Prompt    string        `json:"prompt"`
	Response  string        `json:"response"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}

// Metric represents a generic telemetry metric.
type Metric struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels,omitempty"`
}

// QualityResult represents the output of a quality evaluation.
type QualityResult struct {
	Score   float64  `json:"score"`   // 0.0 to 1.0
	Rules   []string `json:"rules"`   // Names of rules that were evaluated/passed
	Summary string   `json:"summary"` // Human-readable summary of the evaluation
}
