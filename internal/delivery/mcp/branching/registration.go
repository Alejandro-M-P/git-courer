package branching

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleBranch(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleMerge(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleRebase(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleTag(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleCherryPick(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("branch",
			mcpgo.WithDescription("Branch lifecycle — CREATE, DELETE, RENAME, REMOTE_DELETE, SET_UPSTREAM, UNSET_UPSTREAM, SWITCH. Use for branch management. Do NOT use for merging — use merge instead. DELETE and REMOTE_DELETE require confirmed=true — without it, the operation is blocked. SWITCH auto-stashes dirty tree."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Branch operation to perform."), mcpgo.Enum("CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM", "SWITCH")),
			mcpgo.WithString("branch_name", mcpgo.Description("Name of the branch. Required for CREATE, DELETE, RENAME, SWITCH.")),
			mcpgo.WithString("new_branch_name", mcpgo.Description("New name for RENAME operation.")),
			mcpgo.WithString("remote_name", mcpgo.Description("Remote repository name. Defaults to 'origin' if omitted. Used by SET_UPSTREAM, UNSET_UPSTREAM, REMOTE_DELETE.")),
			mcpgo.WithBoolean("force", mcpgo.Description("Force branch creation or deletion even when checks would fail. Use with caution.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for DELETE and REMOTE_DELETE. Without this, destructive operations are blocked.")),
		),
		h.HandleBranch,
	)
	s.AddTool(
		mcpgo.NewTool("merge",
			mcpgo.WithDescription("Merge a branch into the current branch (or into_branch) with conflict detection. After successful merge, delete_source:true removes the source branch, push_after:true pushes to remote, and new_branch:\"name\" creates and switches to a new branch. All composition steps only run if merge succeeds without conflicts."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("branch_name", mcpgo.Required(), mcpgo.Description("Branch to merge into the current branch.")),
			mcpgo.WithString("into_branch", mcpgo.Description("Optional: branch to switch to BEFORE merging.")),
			mcpgo.WithBoolean("abort", mcpgo.Description("Set to true to abort an in-progress merge.")),
			mcpgo.WithBoolean("continue", mcpgo.Description("Set to true to continue a merge after resolving conflicts. Run AFTER staging resolved files.")),
			mcpgo.WithBoolean("skip", mcpgo.Description("Set to true to skip the current conflicting commit.")),
			mcpgo.WithBoolean("delete_source", mcpgo.Description("If true, automatically deletes the source branch after successful merge.")),
			mcpgo.WithBoolean("push_after", mcpgo.Description("If true, automatically pushes changes to remote after successful merge.")),
			mcpgo.WithString("new_branch", mcpgo.Description("Name of a new branch to create and switch to after successful merge.")),
		),
		h.HandleMerge,
	)
	s.AddTool(
		mcpgo.NewTool("rebase",
			mcpgo.WithDescription("Rebase current branch onto a target branch. Same structured conflict output as merge. After resolving conflicts, call rebase with continue=true. Skip a conflicting commit with skip=true. Use --onto to transplant a branch onto a different base. Do NOT use for preserving merge history — use merge instead."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("branch_name", mcpgo.Required(), mcpgo.Description("Target branch to rebase onto.")),
			mcpgo.WithBoolean("abort", mcpgo.Description("Set to true to abort an in-progress rebase. Use when you want to cancel a rebase with conflicts.")),
			mcpgo.WithBoolean("continue", mcpgo.Description("Set to true to continue a rebase after resolving conflicts. Run AFTER staging resolved files.")),
			mcpgo.WithBoolean("skip", mcpgo.Description("Set to true to skip the current conflicting commit during a rebase. Use when a commit is not needed.")),
			mcpgo.WithString("onto", mcpgo.Description("New base to transplant commits onto. When provided, rebase moves commits between the branch and onto point. Example: rebase --onto feature branch moves commits after the fork point onto feature.")),
		),
		h.HandleRebase,
	)
	s.AddTool(
		mcpgo.NewTool("tag",
			mcpgo.WithDescription("Tag lifecycle — CREATE annotated tags, DELETE tags locally or remotely, PUSH tags. Use for version management. Do NOT use for commits. DELETE operations require confirmation."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Tag operation to perform."), mcpgo.Enum("CREATE", "DELETE", "PUSH", "DELETE_REMOTE")),
			mcpgo.WithString("tag_name", mcpgo.Description("Name of the tag. Required for all operations.")),
			mcpgo.WithString("commit_message", mcpgo.Description("Annotated tag message. Used with CREATE to add context to the tag.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for DELETE and DELETE_REMOTE. Without this, destructive operations are blocked.")),
		),
		h.HandleTag,
	)
	s.AddTool(
		mcpgo.NewTool("cherry_pick",
			mcpgo.WithDescription("Apply a specific commit onto the current branch. Use for selectively bringing changes from one branch to another. Do NOT use for full branch integration — use merge instead. Creates backup; undo with backup RESTORE."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("target_commit", mcpgo.Required(), mcpgo.Description("Hash of the commit to cherry-pick onto the current branch.")),
		),
		h.HandleCherryPick,
	)
}
