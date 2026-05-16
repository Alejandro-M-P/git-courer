package sync

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleSync(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRemotes(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("sync",
			mcpgo.WithDescription("Push, pull, or fetch from remote. PUSH is IRREVERSIBLE and requires confirmed=true. PULL and FETCH create a backup before executing. Use FETCH to check remote changes without merging (safer than PULL). Always call diff before pushing."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("PUSH (send commits to remote, irreversible), PULL (fetch and merge, creates backup), FETCH (download remote changes without merging, safest)."), mcpgo.Enum("PUSH", "PULL", "FETCH")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for PUSH. Without this, PUSH is BLOCKED. Set to true only after reviewing the diff with the user.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the sync impact without executing. Returns what would be pushed/pulled. Always use before PUSH.")),
			mcpgo.WithString("remote_name", mcpgo.Description("Remote repository name. Defaults to 'origin' if omitted.")),
		),
		h.HandleSync,
	)
	s.AddTool(
		mcpgo.NewTool("remotes",
			mcpgo.WithDescription("Manage remote repositories — ADD a new remote URL or REMOVE an existing one. REMOVE requires confirmed=true and is NOT undoable via backup."),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("ADD (create remote) or REMOVE (delete remote)."), mcpgo.Enum("ADD", "REMOVE")),
			mcpgo.WithString("remote_name", mcpgo.Description("Name of the remote (e.g., 'origin', 'upstream'). Required for both ADD and REMOVE.")),
			mcpgo.WithString("url", mcpgo.Description("Remote URL. Required for ADD. Ignored for REMOVE.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for REMOVE. Without this, the operation is BLOCKED and does NOT run.")),
		),
		h.HandleRemotes,
	)
}