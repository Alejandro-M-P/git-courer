package core

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleStatus(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleDiff(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleCommit(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleAmend(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRevert(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleCommitJobs(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("status",
			mcpgo.WithDescription(descriptions.DescStatus),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithString("filter", mcpgo.Description("File path pattern to filter results. Matches file paths containing this string.")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of file entries to return. Use with offset for pagination.")),
			mcpgo.WithNumber("offset", mcpgo.Description("Starting position for paginated results. Use with limit.")),
		),
		h.HandleStatus,
	)
	s.AddTool(
		mcpgo.NewTool("diff",
			mcpgo.WithDescription(descriptions.DescDiff),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithString("target_paths", mcpgo.Description("Space-separated file paths to diff. Empty string diffs entire working tree.")),
			mcpgo.WithBoolean("staged", mcpgo.Description("Show staged changes (git diff --cached) instead of unstaged. Use true to review what will be committed.")),
			mcpgo.WithString("branch", mcpgo.Description("Compare against a branch. Use '...' prefix for symmetric diff (e.g., '...main') or '..' for direct range.")),
			mcpgo.WithString("filter", mcpgo.Description("File path pattern to filter diff output. Matches paths containing this string.")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of diff lines to return. Use with offset for large diffs.")),
			mcpgo.WithNumber("offset", mcpgo.Description("Starting line offset for paginated diff results.")),
		),
		h.HandleDiff,
	)
	s.AddTool(
		mcpgo.NewTool("commit",
			mcpgo.WithDescription(descriptions.DescCommit),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Pipeline phase: PREVIEW (generate plan), APPLY (execute commits), ABORT (cancel job), REGENERATE (redo plan with feedback), STATUS (poll job state)."), mcpgo.Enum("PREVIEW", "APPLY", "ABORT", "REGENERATE", "STATUS")),
			mcpgo.WithString("why", mcpgo.Description("Custom explanation or justification for PREVIEW/REGENERATE to guide the commit message generation. Ignored by other commands.")),
			mcpgo.WithString("job_id", mcpgo.Description("Job ID for plumbing path. For STATUS: poll PREVIEW job. For APPLY: use plumbing commit path (CommitTree + UpdateRef) instead of porcelain, creating atomic commit from PREVIEW snapshot.")),
			mcpgo.WithString("feedback", mcpgo.Description("Feedback for REGENERATE — tell the pipeline what to change about the plan.")),
			mcpgo.WithBoolean("push_after", mcpgo.Description("Only for APPLY. If true, automatically pushes commits to remote after successful apply.")),
			mcpgo.WithString("area_response", mcpgo.Description("JSON mapping of directory paths to area names (e.g., {\"internal/infra/cfg/\": \"core\"}). Provided in response to area_required status from PREVIEW. Areas organize your changelog into meaningful sections.")),
		),
		h.HandleCommit,
	)
	s.AddTool(
		mcpgo.NewTool("amend",
			mcpgo.WithDescription(descriptions.DescAmend),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("commit_message", mcpgo.Description("New commit message. If omitted, the existing message is preserved.")),
			mcpgo.WithString("target_paths", mcpgo.Description("Space-separated file paths to include in the amend. Empty string amends with all staged changes.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required to execute. Without this, the operation is BLOCKED and does NOT run. Set to true only after reviewing changes.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the amend impact without executing. Returns what would change. Always use before confirming destructive operations.")),
		),
		h.HandleAmend,
	)
	s.AddTool(
		mcpgo.NewTool("revert",
			mcpgo.WithDescription(descriptions.DescRevert),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("target_commit", mcpgo.Required(), mcpgo.Description("Hash of the commit to revert. Required.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required to execute. Without this, the operation is BLOCKED and does NOT run. Set to true only after reviewing the revert preview.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the revert without executing. Returns what would change. Always use before confirming.")),
		),
		h.HandleRevert,
	)
	s.AddTool(
		mcpgo.NewTool("commit-jobs",
			mcpgo.WithDescription(descriptions.DescCommitJobs),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		h.HandleCommitJobs,
	)
}