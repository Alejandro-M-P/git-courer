package sync

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleSync(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("sync",
			mcpgo.WithDescription(descriptions.DescSync),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("PUSH (send commits to remote, irreversible), PULL (fetch and merge, creates backup), FETCH (download remote changes without merging, safest)."), mcpgo.Enum("PUSH", "PULL", "FETCH")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for PUSH. Without this, PUSH is BLOCKED. Set to true only after reviewing the diff with the user.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the sync impact without executing. Returns what would be pushed/pulled. Always use before PUSH.")),
			mcpgo.WithString("remote_name", mcpgo.Description("Remote repository name. Defaults to 'origin' if omitted.")),
			mcpgo.WithString("branch", mcpgo.Description("Specific branch to push or pull. When omitted, operates on the current branch. Use for targeted sync operations.")),
		),
		h.HandleSync,
	)
}
