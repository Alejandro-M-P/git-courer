package utility

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleConfig(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleBackup(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleUndo(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("config",
			mcpgo.WithDescription("Read or update project configuration. GET returns all config (global and project-level). SET_TEST_COMMAND saves the project test command for release validation. SET_USER_NAME, SET_USER_EMAIL, and SET_SIGNING_KEY persist git identity settings in project-local config."),
			mcpgo.WithString("command", mcpgo.Description("Configuration operation. GET returns current config. SET_TEST_COMMAND saves a test command. SET_USER_NAME, SET_USER_EMAIL, SET_SIGNING_KEY persist git identity settings."), mcpgo.Enum("GET", "SET_TEST_COMMAND", "SET_USER_NAME", "SET_USER_EMAIL", "SET_SIGNING_KEY")),
			mcpgo.WithString("test_command", mcpgo.Description("Test command to save. Example: 'make test-ci'. Used by release to validate before tagging.")),
			mcpgo.WithString("value", mcpgo.Description("Value to set for SET_USER_NAME, SET_USER_EMAIL, or SET_SIGNING_KEY commands. Ignored for GET and SET_TEST_COMMAND.")),
		),
		h.HandleConfig,
	)
	s.AddTool(
		mcpgo.NewTool("backup",
			mcpgo.WithDescription("Manage git backups — CREATE, RESTORE, DELETE, or LIST. Every write operation auto-creates a backup. Use RESTORE to undo a mutation. Use LIST to see available backups with undoable indicators. DELETE removes a specific backup ref."),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Backup operation: CREATE (manual backup), RESTORE (undo mutation), DELETE (remove backup), LIST (show all)."), mcpgo.Enum("CREATE", "DELETE", "RESTORE", "LIST")),
			mcpgo.WithString("ref", mcpgo.Description("Backup reference for targeted RESTORE or DELETE. When omitted, RESTORE defaults to the most recent backup.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for DELETE. Without this, DELETE is BLOCKED. Set to true only after confirming with the user.")),
		),
		h.HandleBackup,
	)
	s.AddTool(
		mcpgo.NewTool("undo",
			mcpgo.WithDescription("Undo the most recent destructive git operation by restoring the latest backup. Shortcut for backup RESTORE without specifying a ref. WHEN to use: immediately after a mistaken amend, merge, rebase, or revert. CONSEQUENCES: resets HEAD to the pre-operation state."),
			mcpgo.WithString("ref", mcpgo.Description("Optional: specific backup ref to restore. When omitted, restores the most recent backup.")),
		),
		h.HandleUndo,
	)
}