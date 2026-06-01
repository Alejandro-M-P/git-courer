package workflow

import (
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

func TestResolveOwnerRepo(t *testing.T) {
	cases := []struct {
		name           string
		input          string
		expectedOwner  string
		expectedRepo   string
		expectedGitHub bool
	}{
		{
			name:           "https clean",
			input:          "https://github.com/owner/repo.git",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedGitHub: true,
		},
		{
			name:           "https no git",
			input:          "https://github.com/owner/repo",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedGitHub: true,
		},
		{
			name:           "https with token",
			input:          "https://token@github.com/owner/repo.git",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedGitHub: true,
		},
		{
			name:           "ssh classic",
			input:          "git@github.com:owner/repo.git",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedGitHub: true,
		},
		{
			name:           "ssh explicit port",
			input:          "ssh://git@github.com:22/owner/repo.git",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedGitHub: true,
		},
		{
			name:           "gitlab skip",
			input:          "https://gitlab.com/owner/repo.git",
			expectedOwner:  "",
			expectedRepo:   "",
			expectedGitHub: false,
		},
		{
			name:           "empty url",
			input:          "",
			expectedOwner:  "",
			expectedRepo:   "",
			expectedGitHub: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, isGitHub, err := resolveOwnerRepo(tc.input)
			if err != nil {
				t.Fatalf("resolveOwnerRepo(%q) returned unexpected error: %v", tc.input, err)
			}
			if owner != tc.expectedOwner {
				t.Errorf("resolveOwnerRepo(%q) owner = %q, want %q", tc.input, owner, tc.expectedOwner)
			}
			if repo != tc.expectedRepo {
				t.Errorf("resolveOwnerRepo(%q) repo = %q, want %q", tc.input, repo, tc.expectedRepo)
			}
			if isGitHub != tc.expectedGitHub {
				t.Errorf("resolveOwnerRepo(%q) isGitHub = %v, want %v", tc.input, isGitHub, tc.expectedGitHub)
			}
		})
	}
}

func TestDetectPRNumbers(t *testing.T) {
	cases := []struct {
		name     string
		commits  string
		expected []int
	}{
		{
			name:     "squash-merge single",
			commits:  "fix: resolve auth bug (#42)",
			expected: []int{42},
		},
		{
			name:     "squash-merge multiple",
			commits:  "feat: add login (#40)\nfix: resolve bug (#42)",
			expected: []int{40, 42},
		},
		{
			name:     "merge-commit",
			commits:  "Merge pull request #7 from feature/x",
			expected: []int{7},
		},
		{
			name:     "merge-commit inline",
			commits:  "1234abcd Merge pull request #55 from owner/main",
			expected: []int{55},
		},
		{
			name:     "deduplication",
			commits:  "feat: a (#3)\nfix: b (#3)",
			expected: []int{3},
		},
		{
			name:     "no PR refs",
			commits:  "feat: no pr reference here",
			expected: []int{},
		},
		{
			name:     "mixed PR and non-PR",
			commits:  "feat: something (#10)\nbuild: update deps\nMerge pull request #9 from fix/typo",
			expected: []int{10, 9},
		},
		{
			name:     "big PR number",
			commits:  "feat: stuff (#999999)",
			expected: []int{999999},
		},
		{
			name:     "edge case markdown link not PR ref",
			commits:  "docs: see [PR](#comparison) for details",
			expected: []int{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectPRNumbers(tc.commits)
			if len(got) != len(tc.expected) {
				t.Fatalf("detectPRNumbers(%q) = %v, want %v", tc.commits, got, tc.expected)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("detectPRNumbers(%q)[%d] = %d, want %d", tc.commits, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestMergeEnrichedCommits(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		enriched map[int][]domain.PRCommit
		want     string
	}{
		{
			name: "single PR replaced",
			raw:  "feat: add login (#10)",
			enriched: map[int][]domain.PRCommit{
				10: {
					{SHA: "abc", Message: "fix: auth bypass"},
					{SHA: "def", Message: "test: add auth tests"},
				},
			},
			want: "fix: auth bypass\ntest: add auth tests",
		},
		{
			name: "multi PR mixed with non-PR",
			raw:  "feat: add login (#10)\nfix: typo\nfeat: dashboard (#20)",
			enriched: map[int][]domain.PRCommit{
				10: {
					{SHA: "a1", Message: "fix: auth"},
				},
				20: {
					{SHA: "b1", Message: "feat: chart"},
					{SHA: "b2", Message: "feat: grid"},
				},
			},
			want: "fix: auth\nfix: typo\nfeat: chart\nfeat: grid",
		},
		{
			name:     "empty enrichment passthrough",
			raw:      "feat: no PR here\nfix: bug",
			enriched: map[int][]domain.PRCommit{},
			want:     "feat: no PR here\nfix: bug",
		},
		{
			name: "PR not resolved retains original",
			raw:  "feat: add login (#10)\nfix: typo (#99)",
			enriched: map[int][]domain.PRCommit{
				10: {{SHA: "a", Message: "fix: auth"}},
			},
			want: "fix: auth\nfix: typo (#99)",
		},
		{
			name: "merge commit format replaced",
			raw:  "Merge pull request #5 from feat/ui",
			enriched: map[int][]domain.PRCommit{
				5: {
					{SHA: "x", Message: "feat: button"},
				},
			},
			want: "feat: button",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeEnrichedCommits(tc.raw, tc.enriched)
			if got != tc.want {
				t.Errorf("mergeEnrichedCommits(%q) =\n%q\nwant:%q", tc.raw, got, tc.want)
			}
		})
	}
}
