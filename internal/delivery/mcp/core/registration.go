package core

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleStatus(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleDiff(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleCommit(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleAmend(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRevert(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	// Descriptions will be moved to instructions.go v2
	s.AddTool(
		mcpgo.NewTool("status",
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
		),
		h.HandleStatus,
	)
	s.AddTool(
		mcpgo.NewTool("diff",
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithString("target_paths"),
			mcpgo.WithBoolean("staged"),
			mcpgo.WithString("branch"),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
		),
		h.HandleDiff,
	)
		s.AddTool(
			mcpgo.NewTool("commit",
				mcpgo.WithReadOnlyHintAnnotation(false),
				mcpgo.WithDestructiveHintAnnotation(true),
				mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("PREVIEW", "APPLY", "ABORT", "REGENERATE", "STATUS")),
				mcpgo.WithString("instruction"),
				mcpgo.WithString("job_id"),
				mcpgo.WithString("feedback"),
			),
			h.HandleCommit,
		)
	s.AddTool(
		mcpgo.NewTool("amend",
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("commit_message"),
			mcpgo.WithString("target_paths"),
			mcpgo.WithBoolean("confirmed"),
		),
		h.HandleAmend,
	)
	s.AddTool(
		mcpgo.NewTool("revert",
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("target_commit", mcpgo.Required()),
			mcpgo.WithBoolean("confirmed"),
		),
		h.HandleRevert,
	)
}
