package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerTools registers the four MCP tools on the server.
func registerTools(s *server.MCPServer, srv *Server) {
	s.AddTool(
		mcpgo.NewTool("git_read",
			mcpgo.WithDescription("Read-only git operations. Commands: READ_STATUS | READ_DIFF | READ_DIFF_STAGED | READ_LOG | READ_BRANCHES | READ_TAGS"),
			mcpgo.WithString("command", mcpgo.Description("READ_STATUS | READ_DIFF | READ_DIFF_STAGED | READ_LOG | READ_BRANCHES | READ_TAGS"), mcpgo.Required()),
		),
		srv.handleGitRead,
	)

	s.AddTool(
		mcpgo.NewTool("git_write",
			mcpgo.WithDescription("Direct write git operations (no LLM). Commands: ADD | CHECKOUT | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM"),
			mcpgo.WithString("command", mcpgo.Description("ADD | CHECKOUT | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM"), mcpgo.Required()),
			mcpgo.WithString("arg", mcpgo.Description("Path, branch name, or additional argument depending on command")),
		),
		srv.handleGitWrite,
	)

	s.AddTool(
		mcpgo.NewTool("git_write_review",
			mcpgo.WithDescription("Write git operations with optional confirmation. Three-phase protocol: {OP}_START → {OP}_APPLY | {OP}_ABORT. Ops: COMMIT, BRANCH_CREATE, BRANCH_DELETE, BRANCH_RENAME, MERGE, REBASE, REBASE_CONTINUE, REBASE_ABORT, RESET_HARD, CHERRY_PICK, REVERT, CLEAN, TAG_CREATE, TAG_DELETE, REMOTE_ADD, REMOTE_REMOVE, CLONE, INIT. Special: STATUS, SUMMARY."),
			mcpgo.WithString("command", mcpgo.Description("e.g. COMMIT_START | COMMIT_APPLY | BRANCH_CREATE_START | BRANCH_CREATE_APPLY | BRANCH_CREATE_ABORT"), mcpgo.Required()),
			mcpgo.WithString("instruction", mcpgo.Description("Natural language instruction for START phase (e.g. 'commit all changes' or 'crear rama para el login')")),
			mcpgo.WithString("branch", mcpgo.Description("Branch name (optional — LLM infers from instruction if absent)")),
			mcpgo.WithString("tag", mcpgo.Description("Tag name for tag operations")),
			mcpgo.WithString("commit", mcpgo.Description("Commit hash for cherry-pick / revert")),
			mcpgo.WithString("arg", mcpgo.Description("Additional argument (url for clone/remote_add, etc.)")),
			mcpgo.WithBoolean("preview", mcpgo.Description(fmt.Sprintf("If true, show preview before executing (default: %v)", srv.cfg.Validation.RequireConfirmation))),
		),
		srv.handleGitWriteReview,
	)
}

// handleGitRead routes read-only git operations directly to the git adapter.
func (s *Server) handleGitRead(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	command := req.GetString("command", "")
	if command == "" {
		return mcpgo.NewToolResultError("command is required"), nil
	}

	var result string
	var err error

	switch command {
	case "READ_STATUS":
		status, sErr := s.git.Status()
		if sErr != nil {
			return mcpgo.NewToolResultError("status failed: " + sErr.Error()), nil
		}
		result = formatStatus(status)
	case "READ_DIFF":
		result, err = s.git.Diff()
	case "READ_DIFF_STAGED":
		result, err = s.git.DiffStaged()
	case "READ_LOG":
		result, err = s.git.Log(20)
	case "READ_BRANCHES":
		current, _ := s.git.CurrentBranch()
		branches, bErr := s.git.ListBranches()
		if bErr != nil {
			return mcpgo.NewToolResultError("branches failed: " + bErr.Error()), nil
		}
		result = "Current: " + current + "\n\n" + branches
	case "READ_TAGS":
		tags, _ := s.git.ListTags()
		result = strings.Join(tags, "\n")
	default:
		return mcpgo.NewToolResultError("Unknown command: " + command), nil
	}

	if err != nil {
		return mcpgo.NewToolResultError(command + " failed: " + err.Error()), nil
	}
	return mcpgo.NewToolResultText(result), nil
}

// handleGitWrite routes direct write operations to the git adapter (no LLM).
func (s *Server) handleGitWrite(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	command := req.GetString("command", "")
	if command == "" {
		return mcpgo.NewToolResultError("command is required"), nil
	}
	arg := req.GetString("arg", "")

	var result string
	var err error

	switch command {
	case "ADD":
		paths := []string{"."}
		if arg != "" {
			paths = strings.Fields(arg)
		}
		err = s.git.Add(paths)
		result = "Files added"
	case "RM":
		err = s.git.Remove(strings.Fields(arg))
		result = "Files removed"
	case "CHECKOUT":
		result, err = s.git.Checkout(arg)
		if result == "" {
			result = "Checked out: " + arg
		}
	case "SWITCH":
		err = s.git.Switch(arg)
		result = "Switched to: " + arg
	case "STASH":
		result, err = s.git.Stash()
		if result == "" {
			result = "Changes stashed"
		}
	case "STASH_POP":
		result, err = s.git.StashPop()
		if result == "" {
			result = "Stashed changes restored"
		}
	case "PUSH":
		result, err = s.git.Push()
	case "PULL":
		result, err = s.git.Pull()
	case "FETCH":
		result, err = s.git.Fetch()
	default:
		return mcpgo.NewToolResultError("Unknown command: " + command), nil
	}

	if err != nil {
		return mcpgo.NewToolResultError(command + " failed: " + err.Error()), nil
	}
	return mcpgo.NewToolResultText(result), nil
}

// handleGitWriteReview handles review operations using the three-phase workflow protocol.
// Commands follow the pattern: {OPERATION}_START | {OPERATION}_APPLY | {OPERATION}_ABORT
func (s *Server) handleGitWriteReview(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	command := req.GetString("command", "")
	if command == "" {
		return mcpgo.NewToolResultError("command is required"), nil
	}

	// Utility commands (no phase)
	switch command {
	case "STATUS":
		plan, err := s.reviewWorkflow.PlanStatus()
		if err != nil {
			return mcpgo.NewToolResultError("status failed: " + err.Error()), nil
		}
		return mcpgo.NewToolResultText(plan), nil
	case "SUMMARY":
		status, err := s.git.Status()
		if err != nil {
			return mcpgo.NewToolResultError("status failed: " + err.Error()), nil
		}
		return mcpgo.NewToolResultText(formatStatus(status)), nil
	}

	// Parse phase from command suffix
	op, phase := parseCommand(command)

	// Special case: commit uses CommitService instead of generic workflow
	if op == "commit" {
		return s.handleCommitOperation(ctx, req, phase)
	}

	// Special case: release uses ReleaseService instead of generic workflow
	if op == "release" {
		return s.handleRelease(ctx, req, phase)
	}

	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")
		explicitArgs := extractExplicitArgs(req)
		res, err := s.reviewWorkflow.Run(ctx, op, instruction, explicitArgs)
		if err != nil {
			return mcpgo.NewToolResultError(op + " failed: " + err.Error()), nil
		}
		if res.Status == "pending_approval" {
			resp, _ := json.Marshal(map[string]interface{}{
				"status":    res.Status,
				"preview":   res.Preview,
				"args":      res.Args,
				"operation": op,
			})
			return mcpgo.NewToolResultText(string(resp)), nil
		}
		return mcpgo.NewToolResultText(res.Output), nil

	case "apply":
		res, err := s.reviewWorkflow.Apply(ctx)
		if err != nil {
			return mcpgo.NewToolResultError("apply failed: " + err.Error()), nil
		}
		return mcpgo.NewToolResultText(res.Output), nil

	case "abort":
		if err := s.reviewWorkflow.Abort(); err != nil {
			return mcpgo.NewToolResultError("abort failed: " + err.Error()), nil
		}
		return mcpgo.NewToolResultText("Operation aborted"), nil

	default:
		return mcpgo.NewToolResultError("Unknown command: " + command + ". Use {OP}_START, {OP}_APPLY, or {OP}_ABORT"), nil
	}
}

// handleCommitOperation handles commit operations using CommitService.
func (s *Server) handleCommitOperation(ctx context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	requireConfirmation := s.cfg.Validation.RequireConfirmation
	preview := req.GetBool("preview", requireConfirmation)

	switch phase {
	case "start":
		if preview {
			isRetry := s.commitConfirm.HasBlocker()
			instruction := req.GetString("instruction", "")

			messages, chunks, warnings, reasoning, err := s.commitSvc.PrepareCommit(instruction)
			if err != nil {
				return mcpgo.NewToolResultError("failed to prepare commit: " + err.Error()), nil
			}

			var files []string
			seen := make(map[string]bool)
			for _, chunk := range chunks {
				for _, f := range chunk.Files {
					if !seen[f] {
						seen[f] = true
						files = append(files, f)
					}
				}
			}

			untracked, _ := s.git.ListUntracked()
			for _, f := range untracked {
				if !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}

			var rejectedMessage string
			if isRetry {
				existingPlan, _ := s.commitConfirm.ReadPlan()
				if existingPlan != nil {
					rejectedMessage = existingPlan.RejectedMessage
					if rejectedMessage == "" && len(existingPlan.Messages) > 0 {
						rejectedMessage = existingPlan.Messages[0]
					}
				}
			}
			s.llm.SetRetryContext(rejectedMessage)

			plan := domain.OperationPlan{
				Operation:       "commit",
				Preview:         strings.Join(messages, "\n"),
				CreatedAt:       time.Now().Unix(),
				Messages:        messages,
				Files:           files,
				RejectedMessage: rejectedMessage,
				Reasoning:       reasoning,
				Instruction:     instruction,
			}
			if err := s.commitConfirm.WritePlan(plan); err != nil {
				return mcpgo.NewToolResultError("failed to save plan: " + err.Error()), nil
			}
			if err := s.commitConfirm.CreateBlocker(); err != nil {
				return mcpgo.NewToolResultError("failed to create blocker: " + err.Error()), nil
			}

			resp, _ := json.Marshal(map[string]interface{}{
				"status":           "pending_approval",
				"message":          plan.Preview,
				"files":            files,
				"rejected_message": rejectedMessage,
				"num_commits":      len(messages),
				"warnings":         warnings,
			})
			return mcpgo.NewToolResultText(string(resp)), nil
		}

		instruction := req.GetString("instruction", "")
		result, err := s.commitSvc.Execute(instruction, false)
		if err != nil {
			return mcpgo.NewToolResultError("commit failed: " + err.Error()), nil
		}
		s.commitConfirm.DeletePlan()
		return mcpgo.NewToolResultText(result), nil

	case "apply":
		if !s.commitConfirm.HasBlocker() {
			return mcpgo.NewToolResultError("No active commit plan. Run COMMIT_START first."), nil
		}

		plan, err := s.commitConfirm.ReadPlan()
		if err != nil || plan == nil {
			s.commitConfirm.RemoveBlocker()
			return mcpgo.NewToolResultError("Failed to read plan. Run COMMIT_START again."), nil
		}

		instruction := req.GetString("instruction", "")

		if instruction != "" {
			result, err := s.commitSvc.Execute(instruction, false)
			if err != nil {
				s.commitConfirm.RemoveBlocker()
				return mcpgo.NewToolResultError("commit failed: " + err.Error()), nil
			}
			s.commitConfirm.DeletePlan()
			s.llm.ClearRetryContext()
			return mcpgo.NewToolResultText(result), nil
		}

		result, err := s.commitSvc.ExecuteFromPlan(plan.Messages, plan.Files, plan.Instruction)
		if err != nil {
			s.commitConfirm.RemoveBlocker()
			return mcpgo.NewToolResultError("commit failed: " + err.Error()), nil
		}

		s.commitConfirm.DeletePlan()
		s.llm.ClearRetryContext()
		return mcpgo.NewToolResultText(result), nil

	case "abort":
		s.commitConfirm.DeletePlan()
		s.llm.ClearRetryContext()
		return mcpgo.NewToolResultText("Commit plan aborted."), nil

	default:
		return mcpgo.NewToolResultError("Unknown commit phase: " + phase + ". Use COMMIT_START, COMMIT_APPLY, or COMMIT_ABORT"), nil
	}
}

// --- Helpers ---

// parseCommand splits "BRANCH_CREATE_START" into op="branch_create" and phase="start".
// Returns phase="unknown" if no valid phase suffix is found.
func parseCommand(command string) (op, phase string) {
	for _, suffix := range []string{"_START", "_APPLY", "_ABORT"} {
		if strings.HasSuffix(strings.ToUpper(command), suffix) {
			phase = strings.ToLower(suffix[1:])
			opRaw := command[:len(command)-len(suffix)]
			op = strings.ToLower(opRaw)
			return
		}
	}
	op = strings.ToLower(command)
	phase = "unknown"
	return
}

// extractExplicitArgs collects named fields from the request into a map.
func extractExplicitArgs(req mcpgo.CallToolRequest) map[string]string {
	args := make(map[string]string)
	for _, key := range []string{"branch", "tag", "commit", "arg"} {
		if v := req.GetString(key, ""); v != "" {
			args[key] = v
		}
	}
	// Alias "arg" → "url" or "name" based on context (handlers pass full args to LLM anyway)
	if v, ok := args["arg"]; ok {
		args["url"] = v
		args["name"] = v
	}
	return args
}

// handleRelease handles release operations using ReleaseService.
func (s *Server) handleRelease(ctx context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")
		if instruction == "" {
			instruction = "sacar versión"
		}

		// Prepare release intent and commits
		intent, commitsChunk, warnings, err := s.releaseSvc.Prepare(instruction)
		if err != nil {
			return mcpgo.NewToolResultError("failed to prepare release: " + err.Error()), nil
		}

		// DEBUG: Log what we got
		fmt.Printf("[DEBUG] Prepare returned: intent=%+v, commitsChunk=%q, len=%d\n", intent.TagName, commitsChunk, len(commitsChunk))

		// If smart release mode, generate changelog
		changelog := ""
		if intent.IsRelease && commitsChunk != "" {
			fmt.Printf("[DEBUG] Calling Generate with %d chars of commits\n", len(commitsChunk))
			changelog, warnings, err = s.releaseSvc.Generate(commitsChunk)
			fmt.Printf("[DEBUG] Generate returned: changelog=%q, err=%v\n", changelog, err)
			if err != nil {
				warnings = append(warnings, err.Error())
			}
		} else {
			fmt.Printf("[DEBUG] Skipping Generate: IsRelease=%v, commitsChunk empty=%v\n", intent.IsRelease, commitsChunk == "")
		}

		// Build preview
		previewText := s.releaseSvc.BuildPreview(intent, changelog)
		if intent.MergePath != nil && len(intent.MergePath) > 0 {
			previewText += "\n\n### Merge Path\n"
			for _, m := range intent.MergePath {
				previewText += "- " + m + "\n"
			}
		}
		if len(warnings) > 0 {
			previewText += "\n\n### Warnings\n"
			for _, w := range warnings {
				previewText += "- " + w + "\n"
			}
		}

		// Store state
		s.releaseIntent = intent
		s.releaseChangelog = changelog

		resp, _ := json.Marshal(map[string]interface{}{
			"status":       "pending_approval",
			"preview":      previewText,
			"tag":          intent.TagName,
			"version_bump": intent.VersionBump,
			"is_release":   intent.IsRelease,
		})
		return mcpgo.NewToolResultText(string(resp)), nil

	case "apply":
		if s.releaseIntent == nil {
			return mcpgo.NewToolResultError("No active release. Run RELEASE_START first."), nil
		}

		createGitHubRelease := req.GetBool("create_github_release", false)
		tagResult, err := s.releaseSvc.Execute(s.releaseIntent, s.releaseChangelog, createGitHubRelease)
		if err != nil {
			return mcpgo.NewToolResultError("release failed: " + err.Error()), nil
		}

		s.releaseIntent = nil
		s.releaseChangelog = ""

		return mcpgo.NewToolResultText(fmt.Sprintf("Release created: %s", tagResult)), nil

	case "abort":
		s.releaseIntent = nil
		s.releaseChangelog = ""
		return mcpgo.NewToolResultText("Release cancelled"), nil

	default:
		return mcpgo.NewToolResultError("Unknown release phase: " + phase + ". Use START, APPLY, or ABORT."), nil
	}
}

// formatStatus formats domain.Status as a human-readable string.
func formatStatus(status domain.Status) string {
	if status.IsClean {
		return fmt.Sprintf("Branch: %s\nWorking tree clean", status.Branch)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Branch: %s\n", status.Branch))
	b.WriteString(fmt.Sprintf("Staged: %d  Modified: %d  Untracked: %d\n\n", status.Staged, status.Modified, status.Untracked))
	for _, f := range status.Files {
		b.WriteString(fmt.Sprintf("  %s  %s\n", f.Status, f.Path))
	}
	return b.String()
}
