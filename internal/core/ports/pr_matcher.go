package ports

// PRMatcher matches branch names to their associated pull request numbers.
//
// This is a v2.8.0 interface contract stub. The real implementation is
// deferred; for now the no-op stub returns zero matches for every branch.
// Downstream code MUST treat an empty result as "no PR associated" and
// degrade gracefully.
type PRMatcher interface {
	// MatchBranch returns the PR numbers associated with the given branch.
	// Returns an empty (non-nil) slice when no PRs match. An error is
	// returned only on infrastructure failure (not on "no match").
	MatchBranch(branch string) ([]int, error)
}