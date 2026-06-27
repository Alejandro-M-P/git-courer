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
// The tool exposes a `command` enum with start, finish, status, and discard.
// Per-command parameters:
//   - start:   agent (required by handler), goal (required by handler)
//   - finish:  session_id (required by handler), agent (optional)
//   - status:  session_id (required by handler)
//   - discard: session_id (required by handler), confirmed (required by handler)
//
// Required-ness is enforced by the handlers (matching the branch package's
// validation pattern) rather than at the schema level, so the enum and param
// set stay discoverable without MCP-level required constraints.
func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("session",
			mcpgo.WithDescription(descriptions.DescSession),
			mcpgo.WithString("command", mcpgo.Required(),
				mcpgo.Description("Session operation to perform. `start` creates an isolated worktree + branch; `finish` merges the session branch into its base and cleans up; `status` reads session state; `discard` removes a session without merging."),
				mcpgo.Enum("start", "finish", "status", "discard")),
			mcpgo.WithString("agent", mcpgo.Description("Name of the agent that will own this session. Required for `start`.")),
			mcpgo.WithString("goal", mcpgo.Description("Short human-readable goal describing the work the session is for. Required for `start`.")),
			mcpgo.WithString("session_id", mcpgo.Description("Session identifier (the 8-hex-char id returned by `start`). Required for `finish`, `status`, and `discard`.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Set to true to authorize destructive operations. Required for `discard`.")),
		),
		h.HandleSession,
	)
}