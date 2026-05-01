package telemetry

import (
	"regexp"
	"strings"
)

var conventionalCommitRegex = regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9-]+\))?!?: .+$`)

// QualityEvaluator implements logic to score LLM responses.
type QualityEvaluator struct{}

// NewQualityEvaluator creates a new QualityEvaluator.
func NewQualityEvaluator() *QualityEvaluator {
	return &QualityEvaluator{}
}

// EvaluateCommitMessage scores a commit message based on conventional commit adherence.
func (e *QualityEvaluator) EvaluateCommitMessage(message string) QualityResult {
	lines := strings.Split(strings.TrimSpace(message), "\n")
	if len(lines) == 0 {
		return QualityResult{Score: 0, Summary: "Empty message"}
	}
	firstLine := strings.TrimSpace(lines[0])

	rules := []string{}
	score := 0.0
	summary := "Non-conventional commit message"

	if conventionalCommitRegex.MatchString(firstLine) {
		score = 1.0
		rules = append(rules, "conventional_commit")
		summary = "Valid conventional commit message"
	}

	return QualityResult{
		Score:   score,
		Rules:   rules,
		Summary: summary,
	}
}
