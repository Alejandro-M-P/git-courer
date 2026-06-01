package prreview

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandlePRReview(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("pr-review",
			mcpgo.WithDescription(descriptions.DescPrReview),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithString("to", mcpgo.Description("Target branch to compare against. Defaults to 'main'. Use 'develop' or feature branches as needed.")),
		),
		h.HandlePRReview,
	)
}
