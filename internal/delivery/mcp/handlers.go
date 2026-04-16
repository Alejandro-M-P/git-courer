package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerTools registers the MCP tools on the server.
func registerTools(s *server.MCPServer, srv *Server) {
	s.AddTool(
		mcpgo.NewTool("git_read",
			mcpgo.WithDescription("Read-only git operations. Commands: READ_STATUS | READ_DIFF | READ_LOG | READ_BRANCHES"),
			mcpgo.WithString("command", mcpgo.Description("READ_STATUS | READ_DIFF | READ_LOG | READ_BRANCHES"), mcpgo.Required()),
		),
		srv.handleGitRead,
	)

	s.AddTool(
		mcpgo.NewTool("git_write",
			mcpgo.WithDescription("Direct write git operations (no LLM). Commands: ADD | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM"),
			mcpgo.WithString("command", mcpgo.Description("ADD | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM"), mcpgo.Required()),
			mcpgo.WithString("arg", mcpgo.Description("Path, branch name, or additional argument depending on command")),
		),
		srv.handleGitWrite,
	)

	s.AddTool(
		mcpgo.NewTool("git_write_review",
			mcpgo.WithDescription("Write git operations with confirmation. Three-phase protocol: {OP}_START → {OP}_APPLY | {OP}_ABORT. Ops: COMMIT, RELEASE, BRANCH_CREATE, BRANCH_DELETE, MERGE. Special: STATUS, SUMMARY."),
			mcpgo.WithString("command", mcpgo.Description("e.g. COMMIT_START | COMMIT_APPLY | BRANCH_CREATE_START | BRANCH_CREATE_APPLY | BRANCH_DELETE_START | MERGE_START"), mcpgo.Required()),
			mcpgo.WithString("instruction", mcpgo.Description("Natural language instruction for START phase")),
			mcpgo.WithString("branch", mcpgo.Description("Branch name")),
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
	case "READ_LOG":
		result, err = s.git.Log(20)
	case "READ_BRANCHES":
		current, _ := s.git.CurrentBranch()
		branches, bErr := s.git.ListBranches()
		if bErr != nil {
			return mcpgo.NewToolResultError("branches failed: " + bErr.Error()), nil
		}
		result = "Current: " + current + "\n\n" + branches
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

	// Special case: commit uses CommitService
	if op == "commit" {
		return s.handleCommitOperation(ctx, req, phase)
	}

	// Special case: release uses ReleaseService
	if op == "release" {
		return s.handleRelease(ctx, req, phase)
	}

	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")
		explicitArgs := extractExplicitArgs(req)

		s.setOpState(op, "processing")
		go func() {
			res, err := s.reviewWorkflow.Run(context.Background(), op, instruction, explicitArgs)
			if err != nil {
				s.setOpState(op, "error: "+err.Error())
				return
			}
			s.clearOpState(op)
			if res.Status == "completed" {
			}
		}()

		return mcpgo.NewToolResultText(processingJSON(
			op + " is being processed. Call " + strings.ToUpper(op) + "_APPLY when ready.",
		)), nil

	case "apply":
		if !s.reviewWorkflow.HasPendingPlan() {
			state := s.getOpState(op)
			switch {
			case strings.HasPrefix(state, "processing"):
				return mcpgo.NewToolResultText(processingJSON(op + " is still being processed. Try again in a moment.")), nil
			case strings.HasPrefix(state, "error:"):
				return mcpgo.NewToolResultError(op + " failed: " + strings.TrimPrefix(state, "error:")), nil
			default:
				return mcpgo.NewToolResultError("No pending " + op + " operation. Run " + strings.ToUpper(op) + "_START first."), nil
			}
		}
		res, err := s.applyWithBackup(op, false, func() (string, error) {
			r, applyErr := s.reviewWorkflow.Apply(context.Background())
			if applyErr != nil {
				return "", applyErr
			}
			return r.Output, nil
		})
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		return mcpgo.NewToolResultText(res), nil

	case "abort":
		s.clearOpState(op)
		if err := s.reviewWorkflow.Abort(); err != nil {
			return mcpgo.NewToolResultError("abort failed: " + err.Error()), nil
		}
		return mcpgo.NewToolResultText("Operation aborted"), nil

	default:
		return mcpgo.NewToolResultError("Unknown command: " + command + ". Use {OP}_START, {OP}_APPLY, or {OP}_ABORT"), nil
	}
}

// handleCommitOperation handles commit operations using CommitService.
func (s *Server) handleCommitOperation(_ context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	requireConfirmation := s.cfg.Validation.RequireConfirmation
	preview := req.GetBool("preview", requireConfirmation)

	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")

		if preview {
			var rejectedMessage string
			if s.commitConfirm.HasBlocker() {
				if existingPlan, _ := s.commitConfirm.ReadPlan(); existingPlan != nil {
					rejectedMessage = existingPlan.RejectedMessage
					if rejectedMessage == "" && len(existingPlan.Messages) > 0 {
						rejectedMessage = existingPlan.Messages[0]
					}
				}
			}
			s.llm.SetRetryContext(rejectedMessage)

			s.commitConfirm.RemoveBlocker()
			s.setOpState("commit", "processing")
			s.commitConfirm.CreateBlocker()

			go func() {
				messages, chunks, deleted, _, reasoning, err := s.commitSvc.PrepareCommit(instruction)
				if err != nil {
					s.setOpState("commit", "error: "+err.Error())
					s.commitConfirm.RemoveBlocker()
					return
				}

				chunkFiles := make([][]string, len(chunks))
				for i, c := range chunks {
					chunkFiles[i] = c.Files
				}

				preview := strings.Join(messages, "\n")
				plan := domain.OperationPlan{
					Operation:   "commit",
					Messages:    messages,
					Chunks:      chunkFiles,
					DeletedFiles: deleted,
					Instruction: instruction,
					Reasoning:   reasoning,
					Preview:     preview,
				}

				if err := s.commitConfirm.WritePlan(plan); err != nil {
					s.setOpState("commit", "error: "+err.Error())
					s.commitConfirm.RemoveBlocker()
					return
				}

				s.clearOpState("commit")
			}()

			return mcpgo.NewToolResultText(processingJSON(
				"Preparing commit... Wait 20-30s then show plan to user with COMMIT_APPLY.",
			)), nil
		}

		// Non-preview mode: execute directly, no plan goroutine
		s.setOpState("commit", "processing")
		go func() {
			result, err := s.commitSvc.Execute(instruction, false)
			if err != nil {
				s.setOpState("commit", "error: "+err.Error())
				return
			}
			_ = result
			s.clearOpState("commit")
			s.commitConfirm.DeletePlan()
		}()

		return mcpgo.NewToolResultText(processingJSON(
			fmt.Sprintf("Commit in progress. Check %q for progress.", s.cfg.Commit.LogPath),
		)), nil

	case "apply":
		if !s.commitConfirm.HasBlocker() {
			state := s.getOpState("commit")
			switch {
			case strings.HasPrefix(state, "processing"):
				return mcpgo.NewToolResultText(processingJSON("Commit preparation in progress. Try again in a moment.")), nil
			case strings.HasPrefix(state, "error:"):
				return mcpgo.NewToolResultError("Commit preparation failed:" + strings.TrimPrefix(state, "error:")), nil
			default:
				return mcpgo.NewToolResultError("No active commit plan. Run COMMIT_START first."), nil
			}
		}

		// Blocker exists but goroutine may still be running — check state first.
		state := s.getOpState("commit")
		if strings.HasPrefix(state, "processing") {
			return mcpgo.NewToolResultText(processingJSON("Commit preparation in progress. Try again in a moment.")), nil
		}
		if strings.HasPrefix(state, "error:") {
			s.commitConfirm.RemoveBlocker()
			return mcpgo.NewToolResultError("Commit preparation failed: " + strings.TrimPrefix(state, "error:")), nil
		}

		plan, err := s.commitConfirm.ReadPlan()
		if err != nil || plan == nil {
			s.commitConfirm.RemoveBlocker()
			return mcpgo.NewToolResultError("Failed to read plan. Run COMMIT_START again."), nil
		}

		instruction := req.GetString("instruction", "")

		if instruction != "" {
			result, err := s.applyWithBackup("commit", true, func() (string, error) {
				return s.commitSvc.Execute(instruction, false)
			})
			if err != nil {
				s.commitConfirm.RemoveBlocker()
				return mcpgo.NewToolResultError(err.Error()), nil
			}
			s.commitConfirm.DeletePlan()
			s.llm.ClearRetryContext()
			return mcpgo.NewToolResultText(result), nil
		}

		result, err := s.commitSvc.ExecuteFromPlan(plan.Messages, plan.Chunks, plan.DeletedFiles, plan.Instruction)
		if err != nil {
			s.commitConfirm.RemoveBlocker()
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		s.commitConfirm.DeletePlan()
		s.llm.ClearRetryContext()
		return mcpgo.NewToolResultText(result), nil

	case "abort":
		s.clearOpState("commit")
		s.commitConfirm.DeletePlan()
		s.llm.ClearRetryContext()
		return mcpgo.NewToolResultText("Commit plan aborted."), nil

	default:
		return mcpgo.NewToolResultError("Unknown commit phase: " + phase + ". Use COMMIT_START, COMMIT_APPLY, or COMMIT_ABORT"), nil
	}
}

// handleRelease handles release operations using ReleaseService.
func (s *Server) handleRelease(_ context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")
		if instruction == "" {
			instruction = "sacar versión"
		}

		s.mu.Lock()
		s.releaseIntent = nil
		s.releaseChangelog = ""
		s.mu.Unlock()

		s.releaseSvc.PrepareAndGenerateAsync(instruction)

		return mcpgo.NewToolResultText(processingJSON(
			"Preparing release... Wait 20-30s then show plan to user with RELEASE_APPLY.",
		)), nil

	case "apply":
		s.mu.Lock()
		cachedIntent := s.releaseIntent
		cachedChangelog := s.releaseChangelog
		s.mu.Unlock()

		if cachedIntent == nil {
			loaded, err := s.releaseSvc.LoadIntent()
			if err != nil {
				state := s.releaseSvc.LoadState()
				switch {
				case strings.HasPrefix(state, "processing"):
					time.Sleep(500 * time.Millisecond)
					if recheck := s.releaseSvc.LoadState(); strings.HasPrefix(recheck, "error:") {
						return mcpgo.NewToolResultError("Release preparation failed:" + strings.TrimPrefix(recheck, "error:")), nil
					}
					return mcpgo.NewToolResultText(processingJSON("Release is still being prepared. Try again in a moment.")), nil
				case strings.HasPrefix(state, "error:"):
					return mcpgo.NewToolResultError("Release preparation failed:" + strings.TrimPrefix(state, "error:")), nil
				default:
					return mcpgo.NewToolResultError("No active release. Run RELEASE_START first."), nil
				}
			}
			cachedIntent = loaded
			s.mu.Lock()
			s.releaseIntent = loaded
			s.mu.Unlock()
		}

		if cachedChangelog == "" {
			loaded, err := s.releaseSvc.LoadChangelog()
			if err != nil {
				return mcpgo.NewToolResultError("Failed to read changelog: " + err.Error()), nil
			}
			if loaded == "" {
				state := s.releaseSvc.LoadState()
				if strings.HasPrefix(state, "processing") {
					return mcpgo.NewToolResultText(processingJSON("Changelog is still being generated. Try again in a moment.")), nil
				}
			}
			cachedChangelog = loaded
			s.mu.Lock()
			s.releaseChangelog = loaded
			s.mu.Unlock()
		}

		createGitHubRelease := req.GetBool("create_github_release", false)
		tagResult, err := s.applyWithBackup("release", false, func() (string, error) {
			return s.releaseSvc.Execute(cachedIntent, cachedChangelog, createGitHubRelease)
		})
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		s.releaseSvc.DeletePendingFiles()
		s.mu.Lock()
		s.releaseIntent = nil
		s.releaseChangelog = ""
		s.mu.Unlock()

		return mcpgo.NewToolResultText(fmt.Sprintf("Release created: %s", tagResult)), nil

	case "abort":
		s.releaseSvc.DeleteState()
		s.mu.Lock()
		s.releaseIntent = nil
		s.releaseChangelog = ""
		s.mu.Unlock()
		return mcpgo.NewToolResultText("Release cancelled"), nil

	default:
		return mcpgo.NewToolResultError("Unknown release phase: " + phase + ". Use START, APPLY, or ABORT."), nil
	}
}

// --- Backup helpers ---

// applyWithBackup wraps a destructive operation with automatic backup and restore.
//   - Before executing: creates a git ref + optional stash (copia de seguridad).
//   - On success: deletes the backup (no saturar el repo).
//   - On failure: auto-restores to the pre-operation state and notifies the caller.
//
// keepIndex=true stashes only unstaged changes, leaving staged files intact (for commit).
// If backup is disabled in config, fn runs directly without any backup logic.
func (s *Server) applyWithBackup(operation string, keepIndex bool, fn func() (string, error)) (string, error) {
	if !s.cfg.Backup.Enabled {
		return fn()
	}

	backup, bErr := s.git.CreateBackup(operation, keepIndex)
	if bErr != nil {
		log.Printf("⚠ backup creation failed for %s: %v — proceeding without backup", operation, bErr)
		return fn()
	}

	result, fnErr := fn()
	if fnErr != nil {
		// Operation failed — auto-restore and notify
		if rErr := s.git.RestoreBackup(backup); rErr != nil {
			if dErr := s.git.DeleteBackup(backup); dErr != nil {
				log.Printf("⚠ could not delete backup ref %s after restore failure: %v", backup.Ref, dErr)
			}
			return "", fmt.Errorf(
				"la operación '%s' falló y la restauración automática también falló.\n"+
					"  Error original: %v\n"+
					"  Error de restauración: %v\n"+
					"  Restauración manual: git reset --hard %s",
				operation, fnErr, rErr, backup.Ref,
			)
		}
		if dErr := s.git.DeleteBackup(backup); dErr != nil {
			log.Printf("⚠ could not delete backup ref %s after failed operation: %v", backup.Ref, dErr)
		}
		return "", fmt.Errorf(
			"⚠ la operación '%s' falló — el repo fue restaurado automáticamente al estado anterior.\n  Error: %v",
			operation, fnErr,
		)
	}

	// Success — remove the backup to keep the repo clean
	if err := s.git.DeleteBackup(backup); err != nil {
		log.Printf("⚠ could not delete backup ref %s: %v", backup.Ref, err)
	}
	return result, nil
}

// --- Helpers ---

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

func extractExplicitArgs(req mcpgo.CallToolRequest) map[string]string {
	args := make(map[string]string)
	for _, key := range []string{"branch", "arg"} {
		if v := req.GetString(key, ""); v != "" {
			args[key] = v
		}
	}
	if v, ok := args["arg"]; ok {
		args["url"] = v
		args["name"] = v
	}
	return args
}

func processingJSON(message string) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"status":  "processing",
		"message": message,
	})
	return string(resp)
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
