package shared

import (
	"context"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SendProgress sends an MCP progress notification to the client if a
// ProgressToken is present in the request. This keeps the LLM engaged during
// long-running operations (commit PREVIEW, release START, etc.) and prevents
// it from falling back to bash.
//
// If the token is nil (client doesn't support progress), this is a no-op.
// The progress value should increase monotonically; total is optional.
// The message should be specific and human-readable: "Analyzing diff… (2/6)"
func SendProgress(ctx context.Context, srv *server.MCPServer, params map[string]any, progress float64, total float64, message string) {
	if srv == nil {
		return
	}
	token := extractProgressToken(params)
	if token == nil {
		return
	}
	var totalPtr *float64
	if total > 0 {
		totalPtr = &total
	}
	notification := mcpgo.NewProgressNotification(token, progress, totalPtr, &message)
	srv.SendNotificationToClient(
		ctx,
		"notifications/progress",
		map[string]any{
			"progressToken": notification.Params.ProgressToken,
			"progress":      notification.Params.Progress,
			"total":         notification.Params.Total,
			"message":       notification.Params.Message,
		},
	)
}

// extractProgressToken pulls the ProgressToken from the request's _meta field.
// The MCP spec sends it as params._meta.progressToken. When called from a
// CallToolRequest, the params come from req.Params.Arguments.
func extractProgressToken(params map[string]any) mcpgo.ProgressToken {
	if params == nil {
		return nil
	}
	// The token can be a string or number per the MCP spec.
	if token, ok := params["_meta.progressToken"]; ok {
		return token
	}
	// Some clients nest _meta as a map.
	if meta, ok := params["_meta"].(map[string]any); ok {
		if token, ok := meta["progressToken"]; ok {
			return token
		}
	}
	return nil
}

// ProgressSteps defines the standard steps for the commit pipeline.
// Use these constants to send consistent progress messages.
const (
	ProgressDiffParse     = 1 // "Parsing diff and building AST…"
	ProgressDepGraph      = 2 // "Building dependency graph…"
	ProgressClassify      = 3 // "Classifying chunks…"
	ProgressPlan          = 4 // "Generating commit plan…"
	ProgressCommitStart   = 5 // "Starting commit execution…"
	ProgressTotal         = 6 // Total steps in the commit pipeline
)

// CommitProgressMessage returns a human-readable progress message for each step.
func CommitProgressMessage(step int) string {
	switch step {
	case ProgressDiffParse:
		return "Parsing diff and building AST… (1/6)"
	case ProgressDepGraph:
		return "Building dependency graph… (2/6)"
	case ProgressClassify:
		return "Classifying chunks by type… (3/6)"
	case ProgressPlan:
		return "Generating commit plan… (4/6)"
	case ProgressCommitStart:
		return "Starting commit execution… (5/6)"
	default:
		return fmt.Sprintf("Processing… (%d/6)", step)
	}
}

// ReleaseProgressSteps defines the standard steps for the release pipeline.
const (
	ReleasePrepare    = 1 // "Preparing release metadata…"
	ReleaseTag        = 2 // "Creating git tag…"
	ReleasePush       = 3 // "Pushing tag to remote…"
	ReleaseGitHub     = 4 // "Creating GitHub release…"
	ReleaseTotal      = 4
)

// ReleaseProgressMessage returns a human-readable progress message for each step.
func ReleaseProgressMessage(step int) string {
	switch step {
	case ReleasePrepare:
		return "Preparing release metadata… (1/4)"
	case ReleaseTag:
		return "Creating git tag… (2/4)"
	case ReleasePush:
		return "Pushing tag to remote… (3/4)"
	case ReleaseGitHub:
		return "Creating GitHub release… (4/4)"
	default:
		return fmt.Sprintf("Processing… (%d/4)", step)
	}
}