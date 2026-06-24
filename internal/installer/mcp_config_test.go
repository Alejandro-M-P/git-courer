// Package installer_test verifies MCP config operations including TOML format
// support and hook installation.
package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigureTomlFormat_WritesValidTOML verifies configureTomlFormat writes
// a valid TOML file with the correct [mcp_servers."git-courer"] section.
func TestConfigureTomlFormat_WritesValidTOML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureTomlFormat(configPath, "mcp_servers", entry); err != nil {
		t.Fatalf("configureTomlFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `[mcp_servers.git-courer]`) && !strings.Contains(content, `[mcp_servers."git-courer"]`) {
		t.Errorf("TOML missing section header [mcp_servers.git-courer]\ncontent:\n%s", content)
	}
	if !strings.Contains(content, `command = "/usr/local/bin/git-courer"`) {
		t.Errorf("TOML missing command field\ncontent:\n%s", content)
	}
	if !strings.Contains(content, `args = ["mcp"]`) {
		t.Errorf("TOML missing args field\ncontent:\n%s", content)
	}
}

// TestConfigureTomlFormat_PreservesExistingConfig verifies configureTomlFormat
// preserves existing content in the config file when adding git-courer.
func TestConfigureTomlFormat_PreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	existing := `[mcp_servers."other-tool"]
command = "other"
`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureTomlFormat(configPath, "mcp_servers", entry); err != nil {
		t.Fatalf("configureTomlFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `other-tool`) {
		t.Errorf("existing section was removed\ncontent:\n%s", content)
	}
	if !strings.Contains(content, `git-courer`) {
		t.Errorf("git-courer section missing\ncontent:\n%s", content)
	}
}

// TestConfigureObjectFormat_DelegatesToToml verifies configureObjectFormat
// delegates to configureTomlFormat when ConfigFormat is "toml".
func TestConfigureObjectFormat_DelegatesToToml(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureObjectFormatWithFormat(configPath, "mcp_servers", entry, "toml"); err != nil {
		t.Fatalf("configureObjectFormatWithFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `[mcp_servers.git-courer]`) && !strings.Contains(content, `[mcp_servers."git-courer"]`) {
		t.Errorf("expected TOML output, got:\n%s", content)
	}
}

// TestConfigureObjectFormat_WritesJSON verifies configureObjectFormat writes
// JSON when ConfigFormat is "json" (default).
func TestConfigureObjectFormat_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureObjectFormatWithFormat(configPath, "mcpServers", entry, "json"); err != nil {
		t.Fatalf("configureObjectFormatWithFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `"mcpServers"`) {
		t.Errorf("expected JSON output with mcpServers key, got:\n%s", content)
	}
	if !strings.Contains(content, `"git-courer"`) {
		t.Errorf("expected JSON output with git-courer entry, got:\n%s", content)
	}
}
