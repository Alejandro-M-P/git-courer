package utility

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleBackup(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("backup",
			mcpgo.WithDescription(descriptions.DescBackup),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Backup operation: RESTORE (undo mutation), LIST (show all)."), mcpgo.Enum("RESTORE", "LIST")),
			mcpgo.WithString("ref", mcpgo.Description("Backup reference for targeted RESTORE. When omitted, RESTORE defaults to the most recent backup.")),
		),
		h.HandleBackup,
	)
}
