package rewrite

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleRewrite(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("rewrite",
			mcpgo.WithDescription(descriptions.DescRewrite),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Rewrite operation: AMEND (fix last commit), REVERT (undo a commit), SOFT (move HEAD only), HARD (discard everything)."), mcpgo.Enum("AMEND", "REVERT", "SOFT", "HARD")),
			mcpgo.WithString("commit_message", mcpgo.Description("New commit message. If omitted, the existing message is preserved. Used by AMEND.")),
			mcpgo.WithString("target_paths", mcpgo.Description("Space-separated file paths to include in the amend. Empty string amends with all staged changes. Used by AMEND.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required to execute. Without this, the operation is BLOCKED and does NOT run. Set to true only after reviewing changes.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the impact without executing. Returns what would change. Always use before confirming destructive operations.")),
			mcpgo.WithString("target_commit", mcpgo.Description("Hash of the commit to revert/reset to. Required for REVERT, SOFT, and HARD.")),
		),
		h.HandleRewrite,
	)
}
