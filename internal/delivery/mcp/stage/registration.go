package stage

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleStage(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleReset(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleStash(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("stage",
			mcpgo.WithDescription(descriptions.DescStage),
			mcpgo.WithDestructiveHintAnnotation(true),
		mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("Stage operation: RM (remove from index), RESTORE (unstage), CLEAN (remove untracked)."), mcpgo.Enum("RM", "RESTORE", "CLEAN")),
		mcpgo.WithString("target_paths", mcpgo.Description("File paths to operate on, space-separated. Required for RM and RESTORE.")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview the impact without executing. Returns what would change. Always use before confirming CLEAN.")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Required for CLEAN. Without this, CLEAN is blocked. Set to true only after reviewing the impact.")),
		),
		h.HandleStage,
	)
	s.AddTool(
		mcpgo.NewTool("reset",
			mcpgo.WithDescription(descriptions.DescReset),
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
			mcpgo.WithDescription(descriptions.DescStash),
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
