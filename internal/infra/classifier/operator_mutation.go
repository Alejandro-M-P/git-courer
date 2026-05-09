package classifier

import (
	"regexp"
	"strings"
)

// operatorPattern matches logical/comparison operators in diff context.
// It looks for lines with + or - prefix containing operator changes.
var operatorPattern = regexp.MustCompile(`^[-+]\s*.*?(>|>=|<|<=|==|!=|&&|\|\||!)`)

// detectOperatorMutation scans a diff for changes to logical or comparison
// operators. If an operator mutation is detected, it signals a behavior-changing
// fix with high confidence.
//
// Returns ("fix", 0.95) if any operator mutation is detected, ("", 0.0) otherwise.
func detectOperatorMutation(diff string) (string, float64) {
	if diff == "" {
		return "", 0.0
	}

	// Lines that changed operators (added or removed)
	var removed []string
	var added []string

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed = append(removed, line)
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added = append(added, line)
		}
	}

	// Quick path: if no added or no removed lines, skip
	if len(removed) == 0 || len(added) == 0 {
		return "", 0.0
	}

	// Extract all operators from removed and added lines
	removedOps := extractOperators(removed)
	addedOps := extractOperators(added)

	// If operator sets differ, there's a mutation
	if operatorSetsDiffer(removedOps, addedOps) {
		return "fix", 0.95
	}

	return "", 0.0
}

// extractOperators finds all comparison/logical operators in lines.
func extractOperators(lines []string) []string {
	var ops []string
	for _, line := range lines {
		// Strip leading +/- prefix and whitespace
		stripped := strings.TrimSpace(line)
		if len(stripped) > 0 {
			stripped = stripped[1:] // remove leading + or -
		}
		stripped = strings.TrimSpace(stripped)

		// Match operators in order of specificity (>= before >, != before =, etc.)
		for _, op := range []string{
			">=", "<=", "!=", "==",
			">", "<",
			"&&", "||",
			"!",
		} {
			for strings.Contains(stripped, op) {
				ops = append(ops, op)
				stripped = strings.Replace(stripped, op, "", 1)
			}
		}
	}
	return ops
}

// operatorSetsDiffer returns true if two operator sets are different.
func operatorSetsDiffer(a, b []string) bool {
	if len(a) != len(b) {
		return true
	}
	// Simple count comparison: sort both and compare
	countA := make(map[string]int)
	countB := make(map[string]int)
	for _, op := range a {
		countA[op]++
	}
	for _, op := range b {
		countB[op]++
	}
	if len(countA) != len(countB) {
		return true
	}
	for op, cnt := range countA {
		if countB[op] != cnt {
			return true
		}
	}
	return false
}
