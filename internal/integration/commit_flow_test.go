//go:build integration

package integration

import (
	"testing"
)

// TestCommitTwoPhaseFlowViaMCPHandlers verifies the full two‑phase commit via MCP handlers.
// Start (prepare) → plan with commit fields → Apply (execute) → commits in git.
func TestCommitTwoPhaseFlowViaMCPHandlers(t *testing.T) {
	// TODO: implement NewTempRepo or use mock
	t.Skip("NewTempRepo not implemented yet - test disabled")
}
