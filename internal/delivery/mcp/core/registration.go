package core

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleStatus(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleDiff(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleCommit(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleAmend(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRevert(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("status",
			mcpgo.WithDescription("Returns COMPLETE repo state in ONE call — branch, ahead/behind, staged, unstaged, untracked, conflicted files, stash count, in-progress operations, last commit. Call BEFORE any write operation to know repo state. Do NOT use raw git status — this replaces 5+ bash calls."),
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
			mcpgo.WithDescription("Annotated diff with AST labels in @@ headers — see WHAT changed at symbol level, not raw lines. Returns hunks labeled [NEW_FUNC], [MOD_SIG ⚠BREAKING], [DEPS], [DEL]. Paginated — no pager hangs. Call before pushing or creating a PR to review what will go up."),
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
			mcpgo.WithDescription("3-phase commit pipeline: PREVIEW parses AST and groups files by dependency graph into atomic commits, APPLY executes them. NOT just git add + commit — DiffChunker analyzes dependencies, Classifier labels each chunk (feat/fix/refactor/BREAKING). Workflow: 1) PREVIEW → get plan, 2) Review with user, 3) APPLY. If PREVIEW returns 'processing', poll STATUS with job_id."),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Pipeline phase: PREVIEW (generate plan), APPLY (execute commits), ABORT (cancel job), REGENERATE (redo plan with feedback), STATUS (poll job state)."), mcpgo.Enum("PREVIEW", "APPLY", "ABORT", "REGENERATE", "STATUS")),
			mcpgo.WithString("instruction", mcpgo.Description("Custom instruction for PREVIEW/REGENERATE to guide commit message style or focus. Ignored by other commands.")),
			mcpgo.WithString("job_id", mcpgo.Description("Job ID returned by PREVIEW or STATUS. Required for APPLY, ABORT, REGENERATE, and STATUS commands.")),
			mcpgo.WithString("feedback", mcpgo.Description("Feedback for REGENERATE — tell the pipeline what to change about the plan.")),
		),
		h.HandleCommit,
	)
	s.AddTool(
		mcpgo.NewTool("amend",
			mcpgo.WithDescription("Fix the last commit — change message, add files, or both. Use when the last commit needs fixing. Do NOT use for new changes (use commit instead). Creates backup BEFORE executing; undo with backup RESTORE. WITHOUT confirmed=true, the operation is BLOCKED and does NOT run."),
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
			mcpgo.WithDescription("Revert a commit by creating a new commit that undoes it. Creates backup BEFORE executing; undo with backup RESTORE. WITHOUT confirmed=true, the operation is BLOCKED and does NOT run. Use dry_run=true first to see what will be reverted."),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("target_commit", mcpgo.Required(), mcpgo.Description("Hash of the commit to revert. Required.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required to execute. Without this, the operation is BLOCKED and does NOT run. Set to true only after reviewing the revert preview.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the revert without executing. Returns what would change. Always use before confirming.")),
		),
		h.HandleRevert,
	)
}