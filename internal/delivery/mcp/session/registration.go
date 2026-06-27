package session

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

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
			mcpgo.WithDescription(descriptions.DescSession),
			mcpgo.WithString("command", mcpgo.Required(),
				mcpgo.Description("Session operation to perform. Only `start` is implemented; `finish`, `status`, and `discard` return \"not implemented\"."),
				mcpgo.Enum("start", "finish", "status", "discard")),
			mcpgo.WithString("agent", mcpgo.Description("Name of the agent that will own this session. Required for `start`.")),
			mcpgo.WithString("goal", mcpgo.Description("Short human-readable goal describing the work the session is for. Required for `start`.")),
		),
		h.HandleSession,
	)
}
