package utility

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleConfig(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleBackup(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRelease(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("config",
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		h.HandleConfig,
	)
	s.AddTool(
		mcpgo.NewTool("backup",
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("CREATE", "DELETE", "RESTORE", "LIST")),
		),
		h.HandleBackup,
	)
	s.AddTool(
		mcpgo.NewTool("release",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("START", "APPLY", "ABORT", "REGENERATE")),
			mcpgo.WithBoolean("dry_run"),
			mcpgo.WithString("feedback"),
		),
		h.HandleRelease,
	)
}