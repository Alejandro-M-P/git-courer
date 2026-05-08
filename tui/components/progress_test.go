package components

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/tui/styles"
)

func TestRenderProgress(t *testing.T) {
	tests := []struct {
		name          string
		steps         []string
		current       int
		wantContains  []string // substrings that MUST appear in output
		wantEmpty     bool     // if true, output must be empty string
	}{
		{
			name:     "empty steps list",
			steps:    []string{},
			current:  0,
			wantEmpty: true,
		},
		{
			name:    "first step selected",
			steps:   []string{"Welcome", "Config", "Review", "Finish"},
			current: 0,
			wantContains: []string{
				"[Welcome]",
				"Config",
				"Review",
				"Finish",
			},
		},
		{
			name:    "middle step selected",
			steps:   []string{"Welcome", "Config", "Review", "Finish"},
			current: 2,
			wantContains: []string{
				"Welcome",
				"Config",
				"[Review]",
				"Finish",
			},
		},
		{
			name:    "negative current index",
			steps:   []string{"A", "B"},
			current: -1,
			wantContains: []string{
				"A",
				"B",
			},
		},
		{
			name:    "current index overflow",
			steps:   []string{"A", "B"},
			current: 5,
			wantContains: []string{
				"A",
				"B",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderProgress(tt.steps, tt.current)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("RenderProgress(%v, %d) = %q, want empty string", tt.steps, tt.current, got)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("RenderProgress(%v, %d) = %q, want to contain %q", tt.steps, tt.current, got, want)
				}
			}
		})
	}
}

// TestRenderProgress_Style verifies that styles are applied correctly:
// - Current step uses SelectedStyle (bracketed)
// - Completed steps use SuccessStyle (✓ prefix)
// - Future steps use SubtextStyle (no prefix)
func TestRenderProgress_Style(t *testing.T) {
	steps := []string{"Welcome", "Config", "Review", "Finish"}

	// At current=0: first step bracketed, rest plain
	got := RenderProgress(steps, 0)
	if !strings.Contains(got, "[Welcome]") {
		t.Errorf("Current step should be bracketed; got: %s", got)
	}
	// Verify separator
	if !strings.Contains(got, " → ") {
		t.Errorf("Steps should be joined with ' → '; got: %s", got)
	}

	// At current=2: steps 0-1 completed with ✓ prefix
	got = RenderProgress(steps, 2)
	if !strings.Contains(got, "✓ Welcome") {
		t.Errorf("Completed step 0 should have ✓ prefix; got: %s", got)
	}
	if !strings.Contains(got, "✓ Config") {
		t.Errorf("Completed step 1 should have ✓ prefix; got: %s", got)
	}
	if !strings.Contains(got, "[Review]") {
		t.Errorf("Current step 2 should be bracketed; got: %s", got)
	}
}

// TestRenderProgress_NegativeAllFuture verifies negative current renders all as future.
func TestRenderProgress_NegativeAllFuture(t *testing.T) {
	steps := []string{"A", "B", "C"}
	got := RenderProgress(steps, -1)

	// No ✓ or [ markers should appear — all are future
	if strings.Contains(got, "✓") {
		t.Errorf("Negative current should render all steps as future (no ✓); got: %s", got)
	}
	if strings.Contains(got, "[") {
		t.Errorf("Negative current should render no step as current (no brackets); got: %s", got)
	}
	// Must still contain step names
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") || !strings.Contains(got, "C") {
		t.Errorf("All step names should appear; got: %s", got)
	}
}

// TestRenderProgress_OverflowAllCompleted verifies overflow renders all as completed.
func TestRenderProgress_OverflowAllCompleted(t *testing.T) {
	steps := []string{"A", "B"}
	got := RenderProgress(steps, 5)

	if !strings.Contains(got, "✓ A") {
		t.Errorf("Overflow should render all steps as completed (✓ A); got: %s", got)
	}
	if !strings.Contains(got, "✓ B") {
		t.Errorf("Overflow should render all steps as completed (✓ B); got: %s", got)
	}
	// No brackets for current — nothing is "current" when over
	if strings.Contains(got, "[") {
		t.Errorf("Overflow should not bracket any step; got: %s", got)
	}
}

// Verify rendering is identical to inline progressIndicator() behavior
// by checking style references match.
func TestRenderProgress_UsesCorrectStyles(t *testing.T) {
	// This test verifies the function produces styled output that is
	// consistent with the existing inline implementations.
	// Since styles inject ANSI codes, we just verify the styled text
	// contains the expected user-visible markers.

	steps := []string{"Welcome", "Config"}
	result := RenderProgress(steps, 1)

	if !strings.Contains(result, "✓ Welcome") {
		t.Errorf("Completed step should have checkmark; got: %s", result)
	}
	if !strings.Contains(result, "[Config]") {
		t.Errorf("Current step should be bracketed; got: %s", result)
	}
}

// Verify the separator is exactly "→" between steps
func TestRenderProgress_Separator(t *testing.T) {
	steps := []string{"A", "B", "C"}
	got := RenderProgress(steps, 1)

	// Strip ANSI codes and check separator count
	// There should be exactly len(steps)-1 " → " separators
	sepCount := strings.Count(got, " → ")
	if sepCount != len(steps)-1 {
		t.Errorf("Expected %d separators, got %d; output: %s", len(steps)-1, sepCount, got)
	}
}

// Verify single step renders correctly
func TestRenderProgress_SingleStep(t *testing.T) {
	got := RenderProgress([]string{"Only"}, 0)
	if !strings.Contains(got, "[Only]") {
		t.Errorf("Single current step should be bracketed; got: %s", got)
	}
	// No separator for single step
	if strings.Contains(got, " → ") {
		t.Errorf("Single step should have no separator; got: %s", got)
	}
}

// Suppress unused import warning (styles is used indirectly via RenderProgress)
var _ = styles.SelectedStyle