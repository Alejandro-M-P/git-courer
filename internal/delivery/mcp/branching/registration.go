package branching

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleBranch(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("branch",
			mcpgo.WithDescription(descriptions.DescBranch),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Branch operation to perform."), mcpgo.Enum("CREATE", "DELETE", "RENAME", "SWITCH", "LIST")),
			mcpgo.WithString("branch_name", mcpgo.Description("Name of the branch. Required for CREATE, DELETE, RENAME, SWITCH. Serves as glob/substring search pattern for LIST.")),
			mcpgo.WithString("new_branch_name", mcpgo.Description("New name for RENAME operation.")),
			mcpgo.WithBoolean("force", mcpgo.Description("Force branch creation or deletion even when checks would fail. Use with caution.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for DELETE. Without this, destructive operations are blocked.")),
			mcpgo.WithBoolean("switch", mcpgo.Description("If true, creates the branch and switches to it in one call. Auto-stashes dirty working tree.")),
			mcpgo.WithString("filter", mcpgo.Description("Filter branches by location: LOCAL, REMOTE, ALL. Used when command is LIST."), mcpgo.Enum("LOCAL", "REMOTE", "ALL")),
		),
		h.HandleBranch,
	)
}
