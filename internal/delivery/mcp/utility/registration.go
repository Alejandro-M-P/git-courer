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
	HandleUndo(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
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
	s.AddTool(
		mcpgo.NewTool("release",
			mcpgo.WithDescription("Semver releases from conventional commits. WHY: Automates version bump calculation and changelog generation from commit history. WHEN: Use when you are ready to publish a new version — not for regular commits. CONSEQUENCES: START computes the bump and previews the tag. APPLY creates and pushes the tag (destructive — not undoable via backup). ABORT discards the plan. REGENERATE revises the plan with feedback."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Release phase: START (calculate version bump and preview tag), APPLY (create and push tag — destructive), ABORT (discard plan), REGENERATE (revise plan with feedback)."), mcpgo.Enum("START", "APPLY", "ABORT", "REGENERATE")),
			mcpgo.WithString("instruction", mcpgo.Description("Natural language instruction for START or REGENERATE, e.g. 'bump minor' or 'release version 2.0.0'.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the release without creating a tag. Use before APPLY to verify the version and changelog.")),
			mcpgo.WithString("feedback", mcpgo.Description("Feedback for REGENERATE to revise the proposed changelog or version.")),
		),
		h.HandleRelease,
	)
}