package classifier

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// PatternFrequency tracks how often labels appear with specific commit types
// in historical commit data.
type PatternFrequency struct {
	mu          sync.RWMutex
	patterns    map[string]map[string]int // labelType -> commitType -> count
	totalCounts map[string]int           // commitType -> totalCounts
}

// NewPatternFrequency creates a new frequency tracker
type NewPatternFrequency struct {
	patterns    map[string]map[string]int
	totalCounts map[string]int
	mu          sync.RWMutex
}

// NewPatternFrequency creates a new frequency tracker
func NewPatternFrequency() *NewPatternFrequency {
	return &NewPatternFrequency{
		patterns:    make(map[string]map[string]int),
		totalCounts: make(map[string]int),
	}
}

// Add adds an observed pattern to the frequency database
func (pf *NewPatternFrequency) Add(labelType, commitType string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if _, exists := pf.patterns[labelType]; !exists {
		pf.patterns[labelType] = make(map[string]int)
	}
	pf.patterns[labelType][commitType]++
	pf.totalCounts[commitType]++
}

// ConfidenceBoost calculates a confidence boost factor (0.0-0.2) based on
// historical frequency of this label+commitType combination.
func (pf *NewPatternFrequency) ConfidenceBoost(labelType, commitType string) float64 {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	if pf.totalCounts[commitType] == 0 {
		return 0.0
	}

	if typeCounts, exists := pf.patterns[labelType]; exists {
		if count, exists := typeCounts[commitType]; exists {
			frequency := float64(count) / float64(pf.totalCounts[commitType])
			// Normalize frequency → confidence boost (0.0-0.2)
			return frequency * 0.2
		}
	}

	return 0.0
}

// NewClassifier creates a Classifier with Git provider for historical learning
func NewClassifier(gitProvider ports.Git) *Classifier {
	return &Classifier{
		gitProvider: gitProvider,
	}
}

// historyRegex matches commit messages with conventional commit format and extracts type
var historyRegex = regexp.MustCompile(`^(?:\w+!?)?:\s+(.+)$`)

// LearnFromHistory analyzes git logs to extract label-to-commit-type mappings
// and improve classification accuracy.
func (c *Classifier) LearnFromHistory() error {
	if c.gitProvider == nil {
		return nil // No git provider, skip learning for now
	}

	// Get recent commit history (last 100 commits)
	logOutput, err := c.gitProvider.Log(100, "", "")
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}

	// Parse log output to extract commit type patterns
	commits := strings.Split(logOutput, "\n\n")
	for _, commit := range commits {
		if strings.Contains(commit, "Merge") || strings.Contains(commit, "merge") {
			continue // Skip merge commits
		}

		// Extract commit type from conventional commit message
		lines := strings.Split(commit, "\n")
		if len(lines) < 3 {
			continue
		}

		commitMsg := lines[2] // Third line is typically the commit message
		if matches := historyRegex.FindStringSubmatch(commitMsg); len(matches) > 1 {
			commitType := strings.Split(matches[1], " ")[0] // First word after type:
			
			// For this prototype, we'll simulate label extraction
			// TODO: Extract actual AST labels from git diff for historical commits
			// For now, simulate some common patterns
			simulateLabelExtraction(context.Background(), commitType, commit)
		}
	}

	return nil
}

// simulateLabelExtraction simulates extracting labels from historical commits
// TODO: Replace with actual git diff analysis for historical commits
func simulateLabelExtraction(ctx context.Context, commitType, commitContent string) {
	// This is a placeholder - in real implementation, we would:
	// 1. Get the commit hash
	// 2. Use git show to get the diff  
	// 3. Parse AST annotations from the historical diff
	// 4. Record label-type mappings
	
	// For now, simulate some common patterns based on commit content
	patterns := map[string][]string{
		"feat":    {"NEW_FUNC", "NEW_TYPE"},
		"fix":     {"MOD_BODY", "MOD_SIG"},
		"refactor": {"MOD_TYPE", "DELETED_FUNC"},
		"chore":   {"CONFIG", "DEPS"},
		"docs":    {"DOCS"},
		"test":    {"TEST", "NEW_FUNC", "MOD_BODY"},
	}

	if labels, exists := patterns[commitType]; exists {
		for _, label := range labels {
			// In real implementation, check frequency counters here
			// patternFrequency.Add(label, commitType)
		}
	}
}