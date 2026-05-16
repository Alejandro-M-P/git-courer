package prreview

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandlePRReview(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("pr-review",
			mcpgo.WithDescription("Pre-PR gate: runs tests, detects conflicts, shows diff stats, and checks branch divergence. Call BEFORE creating ANY PR — no exceptions. Returns test results, conflict files with AST-annotated hunks, and divergence info. Use instead of raw git diff + test commands."),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithString("to", mcpgo.Description("Target branch to compare against. Defaults to 'main'. Use 'develop' or feature branches as needed.")),
		),
		h.HandlePRReview,
	)
}