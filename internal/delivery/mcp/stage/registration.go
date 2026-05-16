package stage

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handlers interface {
	HandleStage(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleReset(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleStash(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("stage",
			mcpgo.WithDescription("Stage, unstage, restore, or clean files. Use ADD to stage changes, RM to remove, RESTORE to unstage, CLEAN to remove untracked files. Do NOT use for committing — use commit instead. CLEAN requires confirmed=true."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Stage operation: ADD (stage files), RM (remove from index), RESTORE (unstage), CLEAN (remove untracked)."), mcpgo.Enum("ADD", "RM", "RESTORE", "CLEAN")),
			mcpgo.WithString("target_paths", mcpgo.Description("File paths to operate on, space-separated. Required for ADD, RM, RESTORE.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the impact without executing. Returns what would change. Always use before confirming CLEAN.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for CLEAN. Without this, CLEAN is blocked. Set to true only after reviewing the impact.")),
		),
		h.HandleStage,
	)
	s.AddTool(
		mcpgo.NewTool("reset",
			mcpgo.WithDescription("Undo commits at different safety levels. SOFT moves HEAD only (safest). MIXED unstages too. HARD discards everything — requires confirmed=true. Use dry_run=true to preview. Do NOT use for amending the last commit — use amend instead."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Reset mode: SOFT (move HEAD), MIXED (unstage + move HEAD), HARD (discard everything)."), mcpgo.Enum("SOFT", "MIXED", "HARD")),
			mcpgo.WithString("target_commit", mcpgo.Description("Commit hash to reset to. Required.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for HARD. Without this, HARD reset is blocked.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the reset impact without executing. Use before confirming HARD.")),
		),
		h.HandleReset,
	)
	s.AddTool(
		mcpgo.NewTool("stash",
			mcpgo.WithDescription("Save, restore, or inspect stashed changes. SAVE stores working tree changes for later. POP restores them. SHOW previews stash content. Use before switching branches with dirty tree."),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Stash operation: SAVE (store changes), POP (restore changes), SHOW (view stash diff)."), mcpgo.Enum("SAVE", "POP", "SHOW")),
			mcpgo.WithString("commit_message", mcpgo.Description("Description for the stash entry. Used with SAVE to label the stash.")),
			mcpgo.WithString("stash_index", mcpgo.Description("Stash reference like stash@{0}. Used with POP or SHOW to specify which stash.")),
			mcpgo.WithBoolean("include_untracked", mcpgo.Description("If true, also stash untracked files when using SAVE. Use when you need a completely clean working tree.")),
			mcpgo.WithBoolean("diff", mcpgo.Description("If true, SHOW returns the diff content of the stash instead of just a summary.")),
		),
		h.HandleStash,
	)
}
