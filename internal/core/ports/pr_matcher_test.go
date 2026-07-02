package ports

import "testing"

// stubPRMatcher is a no-op implementation of PRMatcher for testing.
type stubPRMatcher struct{}

func (m *stubPRMatcher) MatchBranch(branch string) ([]int, error) {
	return []int{}, nil
}

func TestPRMatcher_Stub_ReturnsEmpty(t *testing.T) {
	m := &stubPRMatcher{}

	result, err := m.MatchBranch("feature/any-branch")
	if err != nil {
		t.Fatalf("MatchBranch() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Error("MatchBranch() returned nil slice, want non-nil empty slice")
	}
	if len(result) != 0 {
		t.Errorf("MatchBranch() returned %d PRs, want 0", len(result))
	}
}

func TestPRMatcher_Stub_ReturnsEmptyForAllBranches(t *testing.T) {
	m := &stubPRMatcher{}

	branches := []string{"main", "feature/foo", "fix/bar", "chore/baz", ""}
	for _, branch := range branches {
		result, err := m.MatchBranch(branch)
		if err != nil {
			t.Errorf("MatchBranch(%q) returned error: %v", branch, err)
		}
		if len(result) != 0 {
			t.Errorf("MatchBranch(%q) returned %d PRs, want 0", branch, len(result))
		}
	}
}

// Compile-time check that stubPRMatcher implements PRMatcher.
var _ PRMatcher = (*stubPRMatcher)(nil)
