package session

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// descSession is the tool description used for PR 2. PR 3 will promote this to
// descriptions.DescSession and reference it from here, matching the pattern used
// by the branch package. Kept local for now so PR 2 stays self-contained and
// does not touch descriptions.go (PR 3 scope).
const descSession = `Isolated git worktree + branch per agent for parallel work. Only the ` + "`start`" + ` command is implemented: it creates a ` + "`courer/session-{id}`" + ` branch (atomic ` + "`git update-ref`" + `) and a linked worktree at ` + "`../git-courer-worktrees/{id}/`" + `, persisting session metadata. ` + "`finish`" + `, ` + "`status`" + `, and ` + "`discard`" + ` are declared but return "not implemented". The ` + "`agent`" + ` and ` + "`goal`" + ` parameters are required for ` + "`start`" + `.`

// Handlers is the interface a session handler must satisfy to be registered.
// Mirrors branch/registration.go's Handlers interface pattern.
type Handlers interface {
	HandleSession(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

// Register adds the `session` MCP tool to the server, wired to h.HandleSession.
// The tool exposes a `command` enum with start (implemented) plus finish, status,
// and discard (declared but returning "not implemented"). The agent and goal
// parameters are declared but not marked required at the schema level —
// handleStart enforces their presence and returns a structured error otherwise,
// matching the branch package's validation pattern.
func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("session",
			mcpgo.WithDescription(descSession),
			mcpgo.WithString("command", mcpgo.Required(),
				mcpgo.Description("Session operation to perform. Only `start` is implemented; `finish`, `status`, and `discard` return \"not implemented\"."),
				mcpgo.Enum("start", "finish", "status", "discard")),
			mcpgo.WithString("agent", mcpgo.Description("Name of the agent that will own this session. Required for `start`.")),
			mcpgo.WithString("goal", mcpgo.Description("Short human-readable goal describing the work the session is for. Required for `start`.")),
		),
		h.HandleSession,
	)
}
