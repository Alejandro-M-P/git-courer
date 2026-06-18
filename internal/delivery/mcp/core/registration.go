package core

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleStatus(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleDiff(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleCommit(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
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
			mcpgo.WithString("branch", mcpgo.Description("Compare against a branch name directly (e.g., 'main'). Dot prefixes like '..' or '...' are not supported. Uses symmetric diff (branch...HEAD).")),
			mcpgo.WithString("filter", mcpgo.Description("File path pattern to filter diff output. Matches paths containing this string.")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of diff lines to return. Use with offset for large diffs.")),
			mcpgo.WithNumber("offset", mcpgo.Description("Starting line offset for paginated diff results.")),
			mcpgo.WithBoolean("include_untracked", mcpgo.Description("If true, include untracked file diffs alongside tracked changes. Useful to preview all changes before staging.")),
		),
		h.HandleDiff,
	)
	s.AddTool(
		mcpgo.NewTool("commit",
			mcpgo.WithDescription(descriptions.DescCommit),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Pipeline phase: PREVIEW (generate plan), APPLY (execute commits), STATUS (poll job state)."), mcpgo.Enum("PREVIEW", "APPLY", "STATUS")),
			mcpgo.WithString("why", mcpgo.Description("REQUIRED. The REAL reason this change exists — the problem, symptom, or limitation that motivated it. Do NOT describe what the code does (LLM reads the diff). Explain WHY it had to change. Ignored by non-PREVIEW commands.")),
			mcpgo.WithString("target_paths", mcpgo.Description("Space-separated file paths to stage before generating the preview plan. Use \".\" to stage all changes. Optional — defaults to full working tree.")),
			mcpgo.WithString("job_id", mcpgo.Description("Job ID for plumbing path. For STATUS: poll PREVIEW job. For APPLY: use plumbing commit path (CommitTree + UpdateRef) instead of porcelain, creating atomic commit from PREVIEW snapshot.")),
			mcpgo.WithBoolean("push_after", mcpgo.Description("Only for APPLY. If true, automatically pushes commits to remote after successful apply.")),
			mcpgo.WithString("type", mcpgo.Description("Only for APPLY. Optional commit type override. Overrides the prefix in the commit message. Valid: feat, fix, chore, docs, refactor, test, perf, style.")),
		),
		h.HandleCommit,
	)
}
