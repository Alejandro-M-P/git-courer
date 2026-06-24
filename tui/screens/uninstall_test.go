// Package screens_test verifies the TUI uninstall screen.
package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/installer"
)

// TestRemoveFromJSONConfig_RemovesEntry verifies removeFromJSONConfig removes
// the git-courer entry from a JSON config file.
func TestRemoveFromJSONConfig_RemovesEntry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	config := `{"mcpServers":{"git-courer":{"command":"git-courer"},"other":{"command":"other"}}}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	screen := &UninstallScreen{}
	client := &installer.MCPClient{
		Name:    "test",
		RootKey: "mcpServers",
	}

	if err := screen.removeFromConfig(configPath, client); err != nil {
		t.Fatalf("removeFromConfig: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "git-courer") {
		t.Errorf("git-courer entry was not removed:\n%s", content)
	}
	if !strings.Contains(content, "other") {
		t.Errorf("other entry was removed but should be preserved:\n%s", content)
	}
}

// TestRemoveFromJSONConfig_RemovesEmptyRoot verifies removeFromJSONConfig
// removes the root key when it becomes empty.
func TestRemoveFromJSONConfig_RemovesEmptyRoot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	config := `{"mcpServers":{"git-courer":{"command":"git-courer"}}}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	screen := &UninstallScreen{}
	client := &installer.MCPClient{
		Name:    "test",
		RootKey: "mcpServers",
	}

	if err := screen.removeFromConfig(configPath, client); err != nil {
		t.Fatalf("removeFromConfig: %v", err)
	}

	// File should be removed since config is empty.
	if _, err := os.Stat(configPath); err == nil {
		data, _ := os.ReadFile(configPath)
		t.Errorf("config file should have been removed, but still exists:\n%s", data)
	}
}

// TestRemoveFromTOMLConfig_RemovesEntry verifies removeFromTOMLConfig removes
// the git-courer entry from a TOML config file.
func TestRemoveFromTOMLConfig_RemovesEntry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	config := `[mcp_servers]
  [mcp_servers."git-courer"]
  command = "git-courer"
  [mcp_servers."other"]
  command = "other"
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	screen := &UninstallScreen{}
	client := &installer.MCPClient{
		Name:         "test",
		RootKey:      "mcp_servers",
		ConfigFormat: "toml",
	}

	if err := screen.removeFromConfig(configPath, client); err != nil {
		t.Fatalf("removeFromConfig: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "git-courer") {
		t.Errorf("git-courer entry was not removed:\n%s", content)
	}
	if !strings.Contains(content, "other") {
		t.Errorf("other entry was removed but should be preserved:\n%s", content)
	}
}

// TestRemoveFromTOMLConfig_RemovesEmptyRoot verifies removeFromTOMLConfig
// removes the root key when it becomes empty.
func TestRemoveFromTOMLConfig_RemovesEmptyRoot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	config := `[mcp_servers]
  [mcp_servers."git-courer"]
  command = "git-courer"
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	screen := &UninstallScreen{}
	client := &installer.MCPClient{
		Name:         "test",
		RootKey:      "mcp_servers",
		ConfigFormat: "toml",
	}

	if err := screen.removeFromConfig(configPath, client); err != nil {
		t.Fatalf("removeFromConfig: %v", err)
	}

	// File should be removed since config is empty.
	if _, err := os.Stat(configPath); err == nil {
		data, _ := os.ReadFile(configPath)
		t.Errorf("config file should have been removed, but still exists:\n%s", data)
	}
}
