// Package core implements the core domain MCP handlers for git operations.
// This file contains area/project detection and computation helpers.
package core

import (
	"encoding/json"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// areaRequiredResponse builds the JSON response for the area_required status.
// This response tells the agent that new directories need area assignments
// before proceeding with the commit.
func areaRequiredResponse(directories []string, existingAreas map[string][]string) string {
	type areaRequired struct {
		Status        string              `json:"status"`
		Message       string              `json:"message"`
		Directories   []string            `json:"directories"`
		ExistingAreas map[string][]string `json:"existing_areas"`
		Hint          string              `json:"hint"`
	}
	resp := areaRequired{
		Status:        "area_required",
		Message:       "New directories need area assignments for changelog organization.",
		Directories:   directories,
		ExistingAreas: existingAreas,
		Hint:          "Reply with: area_response {\"internal/infra/cfg/\": \"core\"}. Areas organize your changelog into meaningful sections.",
	}
	data, _ := json.Marshal(resp)
	return string(data)
}

// getStringAreaResponse extracts the area_response parameter from commit params.
// It accepts both map[string]string (structured) and string (JSON) formats.
// Returns nil if not provided or empty.
func getStringAreaResponse(params map[string]any) map[string]string {
	// Try map format first (structured parameter)
	if v, ok := params["area_response"]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			result := make(map[string]string, len(m))
			for k, val := range m {
				if s, ok := val.(string); ok {
					result[k] = s
				}
			}
			if len(result) > 0 {
				return result
			}
		}
		// Try map[string]string directly
		if m, ok := v.(map[string]string); ok {
			if len(m) > 0 {
				return m
			}
		}
		// Try JSON string format
		if s, ok := v.(string); ok && s != "" {
			var result map[string]string
			if err := json.Unmarshal([]byte(s), &result); err == nil && len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

// saveAreaResponse saves the provided area mappings to the project config.
// Append-only: adds new dir→area mappings to existing areas without overwriting.
func (h *Handler) saveAreaResponse(areas map[string]string) error {
	cfg, err := config.LoadProjectConfig(h.workDir)
	if err != nil {
		cfg = &config.ProjectConfig{
			Areas: make(map[string][]string),
		}
	}
	if cfg.Areas == nil {
		cfg.Areas = make(map[string][]string)
	}

	// Append-only: add new directory→area mappings without overwriting existing ones
	for dir, area := range areas {
		if existing, ok := cfg.Areas[area]; ok {
			// Check if this directory is already in the area's paths
			found := false
			for _, p := range existing {
				if p == dir {
					found = true
					break
				}
			}
			if !found {
				cfg.Areas[area] = append(cfg.Areas[area], dir)
			}
		} else {
			cfg.Areas[area] = []string{dir}
		}
	}

	return config.SaveProjectConfig(h.workDir, cfg)
}

// checkNewDirectories loads the project config and checks for directories
// that have no area mapping. Returns the list of new directories and the
// project config, or nil if no new directories are found or if areas are not configured.
func (h *Handler) checkNewDirectories() ([]string, *domain.ProjectConfig, error) {
	projectCfg, err := domain.LoadProjectConfig(h.workDir)
	if err != nil {
		// No config file — no areas configured, so no area question needed
		return nil, nil, nil
	}

	// If no areas are configured, there's nothing to check — all dirs would be "unassigned"
	// and asking about areas would be premature (the project hasn't been set up yet).
	if len(projectCfg.Areas) == 0 {
		return nil, projectCfg, nil
	}

	// Get changed files from git status
	status, err := h.git.Status()
	if err != nil {
		return nil, nil, fmt.Errorf("status check for new directories: %w", err)
	}

	var changedFiles []string
	for _, f := range status.Files {
		if f.Staged {
			changedFiles = append(changedFiles, f.Path)
		}
	}

	if len(changedFiles) == 0 {
		return nil, projectCfg, nil
	}

	newDirs := projectCfg.NewDirectories(changedFiles)
	if len(newDirs) == 0 {
		return nil, projectCfg, nil
	}

	return newDirs, projectCfg, nil
}