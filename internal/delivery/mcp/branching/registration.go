package branching

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleBranch(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleMerge(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRebase(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleTag(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleCherryPick(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("branch",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM", "SWITCH")),
			mcpgo.WithString("branch_name"),
			mcpgo.WithString("new_branch_name"),
			mcpgo.WithString("remote_name"),
			mcpgo.WithBoolean("force"),
			mcpgo.WithBoolean("confirmed"),
		),
		h.HandleBranch,
	)
	s.AddTool(
		mcpgo.NewTool("merge",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("branch_name", mcpgo.Required()),
			mcpgo.WithBoolean("abort"),
			mcpgo.WithBoolean("continue"),
		),
		h.HandleMerge,
	)
	s.AddTool(
		mcpgo.NewTool("rebase",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("branch_name", mcpgo.Required()),
			mcpgo.WithBoolean("abort"),
			mcpgo.WithBoolean("continue"),
		),
		h.HandleRebase,
	)
	s.AddTool(
		mcpgo.NewTool("tag",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("CREATE", "DELETE", "PUSH", "DELETE_REMOTE")),
			mcpgo.WithString("tag_name"),
			mcpgo.WithBoolean("confirmed"),
		),
		h.HandleTag,
	)
	s.AddTool(
		mcpgo.NewTool("cherry_pick",
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("target_commit", mcpgo.Required()),
		),
		h.HandleCherryPick,
	)
}
