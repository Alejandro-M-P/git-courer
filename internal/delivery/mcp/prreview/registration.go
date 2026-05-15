package prreview

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Handlers defines the interface for the pr_review tool handler.
type Handlers interface {
	HandlePRReview(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

// Register adds the pr_review tool to the MCP server.
func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("pr_review",
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithString("to"),
		),
		h.HandlePRReview,
	)
}
