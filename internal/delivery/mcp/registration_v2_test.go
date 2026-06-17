package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestToolRegistrationV2_Names(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	// We will refactor registerTools to include the new tools
	registerTools(mcpSrv, srv)

	wantTools := []string{
		"status", "diff", "commit",
		"branch", "stage", "stash",
		"history", "sync", "pr-review",
		"backup",
	}

	for _, name := range wantTools {
		t.Run(name, func(t *testing.T) {
			found := findTool(mcpSrv, name)
			if found == nil {
				t.Errorf("tool %q not found after registration", name)
			}
		})
	}
}
