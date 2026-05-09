package classifier

import (
	"regexp"
	"strings"
)

// Detecta cambio de operadores: >, <, &&, ||, ==, !=, !, >=, <=.
// Si cambiaste los peajes del código, es un FIX.
func detectOperatorMutation(diff string) (string, float64) {
	if diff == "" {
		return "", 0.0
	}

	opPattern := regexp.MustCompile(`(>=|<=|!=|==|\|\||&&|[<>!])`)

	var before, after []string
	hasDeletions, hasAdditions := false, false

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			before = append(before, opPattern.FindAllString(line, -1)...)
			hasDeletions = true
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			after = append(after, opPattern.FindAllString(line, -1)...)
			hasAdditions = true
		}
	}

	// Must be a modification (both additions and deletions)
	if !hasDeletions || !hasAdditions {
		return "", 0.0
	}

	// Nada que comparar
	if len(before) == 0 && len(after) == 0 {
		return "", 0.0
	}

	// ¿Cambió algo?
	if len(before) != len(after) || !sameOps(before, after) {
		return "fix", 0.95
	}

	return "", 0.0
}

func sameOps(a, b []string) bool {
	count := make(map[string]int)
	for _, x := range a {
		count[x]++
	}
	for _, x := range b {
		count[x]--
		if count[x] < 0 {
			return false
		}
	}
	return true
}
