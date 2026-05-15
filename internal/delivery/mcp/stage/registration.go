package stage

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleStage(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleReset(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleStash(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("stage",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("ADD", "RM", "RESTORE", "CLEAN")),
			mcpgo.WithString("target_paths"),
		),
		h.HandleStage,
	)
	s.AddTool(
		mcpgo.NewTool("reset",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("SOFT", "MIXED", "HARD")),
			mcpgo.WithString("target_commit"),
			mcpgo.WithBoolean("confirmed"),
		),
		h.HandleReset,
	)
	s.AddTool(
		mcpgo.NewTool("stash",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("SAVE", "POP", "SHOW")),
			mcpgo.WithBoolean("diff"),
		),
		h.HandleStash,
	)
}
