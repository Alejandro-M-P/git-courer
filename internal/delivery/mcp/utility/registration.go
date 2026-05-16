package utility

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleConfig(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleBackup(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRelease(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("config",
			mcpgo.WithDescription("Read or update project configuration. READ returns all config plus available models. SET_TEST_COMMAND saves the project test command for release validation."),
			mcpgo.WithString("command", mcpgo.Description("Configuration operation. SET_TEST_COMMAND saves a test command for release validation."), mcpgo.Enum("SET_TEST_COMMAND")),
			mcpgo.WithString("test_command", mcpgo.Description("Test command to save. Example: 'make test-ci'. Used by release to validate before tagging.")),
		),
		h.HandleConfig,
	)
	s.AddTool(
		mcpgo.NewTool("backup",
			mcpgo.WithDescription("Manage git backups — CREATE, RESTORE, DELETE, or LIST. Every write operation auto-creates a backup. Use RESTORE to undo a mutation. Use LIST to see available backups with undoable indicators. DELETE requires confirmed=true."),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Backup operation: CREATE (manual backup), RESTORE (undo mutation), DELETE (remove backup), LIST (show all)."), mcpgo.Enum("CREATE", "DELETE", "RESTORE", "LIST")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for DELETE. Without this, DELETE is BLOCKED. Set to true only after confirming with the user.")),
		),
		h.HandleBackup,
	)
	s.AddTool(
		mcpgo.NewTool("release",
			mcpgo.WithDescription("Semver releases from conventional commits. START calculates version bump. APPLY creates tag and pushes. ABORT cancels. REGENERATE revises with feedback. Use for version management, NOT for regular commits."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Release phase: START (calculate version), APPLY (create and push tag), ABORT (cancel), REGENERATE (revise with feedback)."), mcpgo.Enum("START", "APPLY", "ABORT", "REGENERATE")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the release without creating a tag. Use before APPLY to verify the version and changelog.")),
			mcpgo.WithString("feedback", mcpgo.Description("Feedback for REGENERATE to revise the proposed changelog or version.")),
		),
		h.HandleRelease,
	)
}