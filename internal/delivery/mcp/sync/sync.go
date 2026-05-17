package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
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
		result = shared.WriteResultJSON("PUSH", err == nil, "Pushed to "+remote+" — changes are now on remote")
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
		if uOut, uErr := h.git.Push(); uErr != nil {
			if strings.Contains(uErr.Error(), "NO_UPSTREAM") {
				outputs = append(outputs, "[INFO] No upstream to push to")
			} else {
				return shared.JSONErrorResult("AUTO_PUSH", uErr)
			}
		} else if uOut != "" {
			outputs = append(outputs, uOut)
		}
		result = shared.WriteResultJSON("AUTO_SYNC", true, strings.Join(outputs, "\n"))
	}

	if err != nil {
		return shared.JSONErrorResult(command, err)
	}

	return mcpgo.NewToolResultText(result), nil
}
