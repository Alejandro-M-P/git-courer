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
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("PUSH", "PULL")),
			mcpgo.WithBoolean("confirmed"),
		),
		h.HandleSync,
	)
	s.AddTool(
		mcpgo.NewTool("remotes",
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("ADD", "REMOVE")),
			mcpgo.WithString("remote_name"),
			mcpgo.WithString("url"),
			mcpgo.WithBoolean("confirmed"),
		),
		h.HandleRemotes,
	)
}
