package chunkers

import (
	"log/slog"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
)

// walkCFG counts control-flow keywords in src by simple word matching.
// Returns zero CFGCount if langName is empty or src is empty/nil.
func walkCFG(langName string, src []byte, cf data.ControlFlowCategory) domain.CFGCount {
	if langName == "" || len(src) == 0 {
		return domain.CFGCount{}
	}

	// Build lookup sets from ControlFlowCategory for O(1) matching.
	branchSet := make(map[string]bool, len(cf.Branch))
	for _, kw := range cf.Branch {
		branchSet[kw] = true
	}
	loopSet := make(map[string]bool, len(cf.Loop))
	for _, kw := range cf.Loop {
		loopSet[kw] = true
	}
	returnSet := make(map[string]bool, len(cf.Return))
	for _, kw := range cf.Return {
		returnSet[kw] = true
	}
	errorSet := make(map[string]bool, len(cf.Error))
	for _, kw := range cf.Error {
		errorSet[kw] = true
	}

	// If all sets are empty, no control-flow matching is possible.
	if len(branchSet) == 0 && len(loopSet) == 0 && len(returnSet) == 0 && len(errorSet) == 0 {
		return domain.CFGCount{}
	}

	// Tokenize source into words and match against keyword sets.
	var count domain.CFGCount
	for _, field := range strings.Fields(string(src)) {
		if branchSet[field] {
			count.Branch++
		}
		if loopSet[field] {
			count.Loop++
		}
		if returnSet[field] {
			count.Return++
		}
		if errorSet[field] {
			count.Error++
		}
	}

	return count
}

// ComputeCFGDiff tokenizes before/after source and counts keyword matches
// against the language's ControlFlowCategory data.
// For empty langName or empty ControlFlow, returns zero-value CFGDiff (no error).
// Degrades gracefully — returns zero CFGCount, never errors.
func ComputeCFGDiff(langName string, beforeSrc, afterSrc []byte, controlFlow data.ControlFlowCategory) domain.CFGDiff {
	if langName == "" {
		return domain.CFGDiff{}
	}

	// If ControlFlow is all empty, no matching is possible.
	if len(controlFlow.Branch) == 0 && len(controlFlow.Loop) == 0 &&
		len(controlFlow.Return) == 0 && len(controlFlow.Error) == 0 {
		return domain.CFGDiff{}
	}

	beforeCount := walkCFG(langName, beforeSrc, controlFlow)
	afterCount := walkCFG(langName, afterSrc, controlFlow)

	return domain.CFGDiff{
		Before: beforeCount,
		After:  afterCount,
	}
}

// ComputeEntityCFGDiff computes CFG diff for a single entity's body byte span
// instead of the whole file. It accepts separate byte spans for before and after
// sources because the same entity may appear at different byte offsets if other
// entities earlier in the file changed size.
// When both body spans are valid, it extracts the sub-slices and computes CFG
// on each independently. When either span is invalid (out of bounds, negative, or
// inverted), it falls back to file-level ComputeCFGDiff and emits a slog.Debug.
func ComputeEntityCFGDiff(langName string, beforeSrc, afterSrc []byte, beforeBodyStart, beforeBodyEnd, afterBodyStart, afterBodyEnd int, cf data.ControlFlowCategory) domain.CFGDiff {
	// Validate before body span
	validBefore := beforeBodyStart >= 0 && beforeBodyEnd > beforeBodyStart &&
		beforeBodyStart <= len(beforeSrc) && beforeBodyEnd <= len(beforeSrc)

	// Validate after body span
	validAfter := afterBodyStart >= 0 && afterBodyEnd > afterBodyStart &&
		afterBodyStart <= len(afterSrc) && afterBodyEnd <= len(afterSrc)

	if !validBefore || !validAfter {
		slog.Debug("per-entity CFG fallback: invalid byte span",
			"beforeBodyStart", beforeBodyStart, "beforeBodyEnd", beforeBodyEnd,
			"afterBodyStart", afterBodyStart, "afterBodyEnd", afterBodyEnd)
		return ComputeCFGDiff(langName, beforeSrc, afterSrc, cf)
	}

	// Extract entity body sub-slices from respective sources.
	beforeSlice := beforeSrc[beforeBodyStart:beforeBodyEnd]
	afterSlice := afterSrc[afterBodyStart:afterBodyEnd]

	beforeCount := walkCFG(langName, beforeSlice, cf)
	afterCount := walkCFG(langName, afterSlice, cf)

	return domain.CFGDiff{
		Before: beforeCount,
		After:  afterCount,
	}
}