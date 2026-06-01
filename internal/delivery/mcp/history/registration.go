package history

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

type Handlers interface {
	HandleHistory(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
	HandleBlame(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
}

func Register(s *server.MCPServer, h Handlers) {
	s.AddTool(
		mcpgo.NewTool("history",
			mcpgo.WithDescription(descriptions.DescHistory),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("LOG for commit history, REFLOG for operation history and recovery."), mcpgo.Enum("LOG", "REFLOG")),
			mcpgo.WithString("target_commit", mcpgo.Description("Starting commit hash or reference. Defaults to HEAD.")),
			mcpgo.WithString("target_paths", mcpgo.Description("Space-separated file paths to filter history by. Only shows commits affecting these files.")),
			mcpgo.WithString("pattern", mcpgo.Description("Search pattern to filter commit messages. Matches commits containing this string.")),
			mcpgo.WithString("filter", mcpgo.Description("Filter results by path pattern. Matches file paths containing this string.")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of entries to return. Use with offset for pagination.")),
			mcpgo.WithNumber("offset", mcpgo.Description("Starting position for paginated results.")),
		),
		h.HandleHistory,
	)

	s.AddTool(
		mcpgo.NewTool("blame",
			mcpgo.WithDescription(descriptions.DescBlame),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("target_paths", mcpgo.Required(), mcpgo.Description("Path to the file to blame. Required.")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of blame entries to return.")),
			mcpgo.WithNumber("offset", mcpgo.Description("Starting line offset for paginated results.")),
		),
		h.HandleBlame,
	)
}
