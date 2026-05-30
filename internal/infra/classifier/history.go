package classifier

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// PatternFrequency tracks commit type frequencies from git history to improve
// confidence scoring for recurring patterns in the repository.
type PatternFrequency struct {
	mu     sync.RWMutex
	counts map[string]int
	total  int
}

func newPatternFrequency() *PatternFrequency {
	return &PatternFrequency{counts: make(map[string]int)}
}

func (pf *PatternFrequency) record(commitType string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.counts[commitType]++
	pf.total++
}

// ConfidenceBoost returns a small boost (0.0–0.05) when the commit type is
// historically frequent in this repository (>30% → 0.05, >10% → 0.02).
func (pf *PatternFrequency) ConfidenceBoost(commitType string) float64 {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	if pf.total == 0 {
		return 0.0
	}
	freq := float64(pf.counts[commitType]) / float64(pf.total)
	if freq > 0.3 {
		return 0.05
	}
	if freq > 0.1 {
		return 0.02
	}
	return 0.0
}

// NewClassifier creates a Classifier with an optional Git provider for
// historical learning. Pass nil to skip history-based confidence boosting.
func NewClassifier(gitProvider ports.Git) *Classifier {
	return &Classifier{
		gitProvider: gitProvider,
		patternFreq: newPatternFrequency(),
		catalog:     domain.NewLanguageCatalog(nil, nil, nil),
	}
}

// ClassifierOption configures a Classifier during construction.
type ClassifierOption func(*Classifier)

// WithBinaryClassifier injects a BinaryClassifier for MOD_BODY_CALL delegation.
// When nil, MOD_BODY_CALL degrades gracefully to "fix".
func WithBinaryClassifier(bc ports.BinaryClassifier) ClassifierOption {
	return func(c *Classifier) { c.binaryClassifier = bc }
}

// WithPathTypes injects a path-prefix-to-commit-type map for Pillar 0.5 detection.
// When empty/nil, Pillar 0.5 is a no-op and classification falls through to
// weight-based Pillar 1.
func WithPathTypes(pathTypes map[string][]string) ClassifierOption {
	return func(c *Classifier) { c.pathTypes = pathTypes }
}

// NewClassifierWithCatalog creates a Classifier with an optional Git provider
// and a custom language catalog. Pass nil for gitProvider to skip history-based
// confidence boosting. Variadic options are applied in order.
func NewClassifierWithCatalog(gitProvider ports.Git, catalog *domain.LanguageCatalog, opts ...ClassifierOption) *Classifier {
	c := &Classifier{
		gitProvider: gitProvider,
		patternFreq: newPatternFrequency(),
		catalog:     catalog,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// logLineRegex parses lines from git log --pretty=format:%H|%an|%ad|%s
// and extracts the conventional commit type (e.g. feat, fix, chore).
var logLineRegex = regexp.MustCompile(`^[0-9a-f]+\|[^|]*\|[^|]*\|(\w+)!?:`)

// LearnFromHistory queries recent git log to build a commit type frequency
// table. The frequency is used in Classify to boost confidence for commit
// types that are common in this repository.
func (c *Classifier) LearnFromHistory() error {
	if c.gitProvider == nil {
		return nil
	}

	logOutput, err := c.gitProvider.Log(100, "")
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}
	if logOutput == "" {
		return nil
	}

	for _, line := range strings.Split(logOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := logLineRegex.FindStringSubmatch(line); len(m) > 1 {
			c.patternFreq.record(m[1])
		}
	}

	return nil
}
