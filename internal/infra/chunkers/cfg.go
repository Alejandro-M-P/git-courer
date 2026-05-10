package chunkers

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
	gotreesitter "github.com/odvcencio/gotreesitter"
)

// walkCFG walks all nodes (named + unnamed) in src, counting matches against cf.
// Returns zero CFGCount if lang is nil or src is empty/nil.
func walkCFG(lang *gotreesitter.Language, src []byte, cf data.ControlFlowCategory) domain.CFGCount {
	if lang == nil || len(src) == 0 {
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

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return domain.CFGCount{}
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		return domain.CFGCount{}
	}

	var count domain.CFGCount
	walkAllNodes(root, lang, branchSet, loopSet, returnSet, errorSet, &count)
	return count
}

// walkAllNodes visits ALL children (named + unnamed) via node.Child(i),
// matching node.Type(lang) against the control-flow keyword sets.
func walkAllNodes(node *gotreesitter.Node, lang *gotreesitter.Language, branchSet, loopSet, returnSet, errorSet map[string]bool, count *domain.CFGCount) {
	nodeType := node.Type(lang)

	if branchSet[nodeType] {
		count.Branch++
	}
	if loopSet[nodeType] {
		count.Loop++
	}
	if returnSet[nodeType] {
		count.Return++
	}
	if errorSet[nodeType] {
		count.Error++
	}

	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			walkAllNodes(child, lang, branchSet, loopSet, returnSet, errorSet, count)
		}
	}
}

// ComputeCFGDiff parses before/after source with gotreesitter, walks ALL AST nodes
// (including unnamed keyword tokens), and counts matches against the language's
// ControlFlowCategory data. Returns CFGDiff with Before/After populated.
// For nil grammar or empty ControlFlow, returns zero-value CFGDiff (no error).
// Parse failures degrade gracefully — returns zero CFGCount, never errors.
func ComputeCFGDiff(lang *gotreesitter.Language, beforeSrc, afterSrc []byte, controlFlow data.ControlFlowCategory) domain.CFGDiff {
	if lang == nil {
		return domain.CFGDiff{}
	}

	// If ControlFlow is all empty, no matching is possible.
	if len(controlFlow.Branch) == 0 && len(controlFlow.Loop) == 0 &&
		len(controlFlow.Return) == 0 && len(controlFlow.Error) == 0 {
		return domain.CFGDiff{}
	}

	beforeCount := walkCFG(lang, beforeSrc, controlFlow)
	afterCount := walkCFG(lang, afterSrc, controlFlow)

	return domain.CFGDiff{
		Before: beforeCount,
		After:  afterCount,
	}
}