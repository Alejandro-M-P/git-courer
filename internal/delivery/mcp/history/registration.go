package history

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	descHistory = "Show commit history, branches, tags, merge-base, and reflog. Structured JSON, pagination, and filtering."
	descBlame   = "Show line-by-line attribution for a specific file (who changed what and when)."
)

type Handlers interface {
	HandleHistory(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleBlame(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("history",
			mcpgo.WithDescription(descHistory),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("LOG", "REFLOG")),
			mcpgo.WithString("target_commit"),
			mcpgo.WithString("target_paths"),
			mcpgo.WithString("pattern"),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
		),
		h.HandleHistory,
	)

	s.AddTool(
		mcpgo.NewTool("blame",
			mcpgo.WithDescription(descBlame),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("target_paths", mcpgo.Required(), mcpgo.Description("Path to the file to blame")),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
		),
		h.HandleBlame,
	)
}
