package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) HandleSync(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", ""))

	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	validCommands := []string{"FETCH", "PULL", "PUSH", "AUTO"}
	valid := false
	for _, c := range validCommands {
		if command == c {
			valid = true
			break
		}
	}
	if !valid {
		hint := shared.SuggestCommand(command, validCommands)
		if hint != "" {
			return shared.JSONErrorResult("sync", fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult("sync", fmt.Errorf("unknown command: %s", command))
	}

	remote := shared.GetStringParam(params, "remote_name", "origin")
	branch := shared.GetStringParam(params, "branch", "")

	if command == "PUSH" || command == "AUTO" {
		cmdForSafety := "push"
		if result, err := shared.CheckSafetyGate(cmdForSafety, dryRun, confirmed); result != nil || err != nil {
			return result, err
		}
		if dryRun {
			impact, _ := shared.ComputeImpact(cmdForSafety, params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
	}

	var err error
	var result string

	// FETCH, PULL, PUSH don't stash by default, but CreateBackup writes to log
	_, _ = h.git.CreateBackup(command, domain.StashNone)

	switch command {
	case "FETCH":
		_, err = h.git.Fetch()
		result = shared.WriteResultJSON("FETCH", err == nil, "Fetched from remote")
	case "PULL":
		if branch != "" {
			_, err = h.git.PullFromBranch(remote, branch)
		} else {
			_, err = h.git.PullFrom(remote)
		}
		if err != nil && strings.Contains(err.Error(), "NO_UPSTREAM") {
			result = `{"error":"No upstream configured","hint":"Push first or specify remote/branch"}`
			err = nil
			break
		}
		result = shared.WriteHintedResultJSON("PULL", err == nil, "Pulled from "+remote, "consider calling diff to review merged changes")
	case "PUSH":
		if branch != "" {
			_, err = h.git.PushToBranch(remote, branch)
		} else {
			_, err = h.git.PushTo(remote)
		}
		if err == nil {
			// Non-blocking: push refs/courer/<branch> as sidecar
			if refBranch := branch; refBranch != "" {
				if _, refErr := h.git.PushToBranch("origin", "refs/courer/"+refBranch); refErr != nil {
					log.Printf("[WARN] sync: failed to push refs/courer/%s: %v", refBranch, refErr)
				}
			}
		}
		result = shared.WriteResultJSON("PUSH", err == nil, "Pushed to "+remote+" — changes are now on remote. Remember: call pr-review before creating a PR.")
		// branch=="" means no ref push sidecar; do not call CurrentBranch
	case "AUTO":
		var outputs []string
		// 1. Fetch
		if fOut, fErr := h.git.Fetch(); fErr != nil {
			return shared.JSONErrorResult("AUTO_FETCH", fErr)
		} else if fOut != "" {
			outputs = append(outputs, fOut)
		}
		// 2. Pull
		if pOut, pErr := h.git.Pull(); pErr != nil {
			// Pull can fail if no upstream, but AUTO should try to Push anyway if possible
			// but usually Pull is expected. Let's be strict but informative.
			if strings.Contains(pErr.Error(), "NO_UPSTREAM") {
				outputs = append(outputs, "[INFO] No upstream to pull from")
			} else {
				return shared.JSONErrorResult("AUTO_PULL", pErr)
			}
		} else if pOut != "" {
			outputs = append(outputs, pOut)
		}
		// 3. Push
		var pushedBranch string
		if uOut, uErr := h.git.Push(); uErr != nil {
			if strings.Contains(uErr.Error(), "NO_UPSTREAM") {
				outputs = append(outputs, "[INFO] No upstream to push to")
			} else {
				return shared.JSONErrorResult("AUTO_SYNC", uErr)
			}
		} else {
			if uOut != "" {
				outputs = append(outputs, uOut)
			}
			if currentBranch, branchErr := h.git.CurrentBranch(); branchErr == nil && currentBranch != "" {
				pushedBranch = currentBranch
			}
		}
		// Non-blocking: push refs/courer/<branch> as sidecar
		if pushedBranch != "" {
			if _, refErr := h.git.PushToBranch("origin", "refs/courer/"+pushedBranch); refErr != nil {
				log.Printf("[WARN] sync: failed to push refs/courer/%s: %v", pushedBranch, refErr)
			}
		}
		result = shared.WriteResultJSON("AUTO_SYNC", true, strings.Join(outputs, "\n"))
	}

	if err != nil {
		return shared.JSONErrorResult(command, err)
	}

	return mcpgo.NewToolResultText(result), nil
}
