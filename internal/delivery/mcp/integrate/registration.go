package integrate

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleIntegrate(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("integrate",
			mcpgo.WithDescription(descriptions.DescIntegrate),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Integration operation: MERGE (merge a branch), UPDATE (rebase onto a branch), PICK (cherry-pick a commit), CONTINUE (continue after resolving conflicts), ABORT (abort in-progress operation)."), mcpgo.Enum("MERGE", "UPDATE", "PICK", "CONTINUE", "ABORT")),
			mcpgo.WithString("branch_name", mcpgo.Description("Branch name. Required for MERGE and UPDATE.")),
			mcpgo.WithString("target_commit", mcpgo.Description("Hash of the commit to cherry-pick. Required for PICK.")),
			mcpgo.WithString("into_branch", mcpgo.Description("Optional: branch to switch to BEFORE merging. Used by MERGE.")),
			mcpgo.WithBoolean("delete_source", mcpgo.Description("If true, automatically deletes the source branch after successful merge. Used by MERGE.")),
			mcpgo.WithBoolean("push_after", mcpgo.Description("If true, automatically pushes changes to remote after successful merge. Used by MERGE.")),
			mcpgo.WithString("new_branch", mcpgo.Description("Name of a new branch to create and switch to after successful merge. Used by MERGE.")),
			mcpgo.WithString("onto", mcpgo.Description("New base to transplant commits onto. Used by UPDATE with --onto.")),
		),
		h.HandleIntegrate,
	)
}
