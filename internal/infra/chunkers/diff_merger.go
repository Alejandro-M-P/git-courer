// Package chunkers diff_merger.go previously held the emoji-era annotation
// merge logic (MergeDiffIntoAnnotations, parseLabelGroups, parseLabelLine,
// rebuildAnnotatedDiff). That code was removed in Phase 2 of the
// chunker-prompt-quality change once the structured buildAnnotatedEntries path
// (in hunk_extractor.go) was fully wired through Annotate/AnnotateWithContent
// and the LLM adapters, and the manual e2e test was migrated to the new path.
//
// The reusable hunk-extraction helpers (hunkLinesForLabel, labelWindow,
// lineInWindow, trimContext, parseDiffHunks, buildAnnotatedEntries) now live
// in hunk_extractor.go. This file is kept as a marker so contributors find the
// rationale for the removal rather than a silently empty package file.
package chunkers