// Package installer provides installation and management for git-courer.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// MCPClient represents an MCP client configuration.
type MCPClient struct {
	Name     string
	Filename string // config filename
	RootKey  string // mcpServers, mcp, servers, context_servers
	IsArray  bool   // continue uses array format
	ConfigFn func(binPath string) map[string]interface{}
	Paths    []string // possible config file paths for this platform
	Detect   func() bool
}

// MCPServerConfig represents an MCP server entry.
type MCPServerConfig struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Type    string   `json:"type,omitempty"`
}

// DetectClients returns a list of detected MCP clients on the system.
func DetectClients() []*MCPClient {
	var detected []*MCPClient
	for _, client := range MCPClients() {
		if client.Detect() {
			detected = append(detected, client)
		}
	}
	return detected
}

// MCPClients returns all supported MCP clients.
func MCPClients() []*MCPClient {
	home := homeDir()
	osName := runtime.GOOS

	return []*MCPClient{
		{
			Name:     "opencode",
			Filename: "opencode.json",
			RootKey:  "mcp",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"type":    "local",
					"enabled": true,
					"command": []string{binPath, "mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".config/opencode/opencode.json"),
			},
			Detect: func() bool {
				if _, err := exec.LookPath("opencode"); err == nil {
					return true
				}
				_, err := os.Stat(filepath.Join(home, ".config/opencode"))
				return err == nil
			},
		},
		{
			Name:     "claude-code",
			Filename: ".mcp.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".claude.json"),
				".claude.json",
			},
			Detect: func() bool {
				_, err := exec.LookPath("claude")
				return err == nil
			},
		},
		{
			Name:     "cursor",
			Filename: "mcp.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".cursor/mcp.json"),
			},
			Detect: func() bool {
				if _, err := exec.LookPath("cursor"); err == nil {
					return true
				}
				_, err := os.Stat(filepath.Join(home, ".cursor"))
				return err == nil
			},
		},
		{
			Name:     "windsurf",
			Filename: "mcp_config.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".codeium/windsurf/mcp_config.json"),
			},
			Detect: func() bool {
				// Check for windsurf directory
				_, err := os.Stat(filepath.Join(home, ".codeium/windsurf"))
				return err == nil
			},
		},
		{
			Name:     "cline",
			Filename: "cline_mcp_settings.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: clinePaths(),
			Detect: func() bool {
				switch osName {
				case "darwin":
					_, err := os.Stat(filepath.Join(home, "Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev"))
					return err == nil
				case "windows":
					_, err := os.Stat(filepath.Join(os.Getenv("APPDATA"), "Code/User/globalStorage/saoudrizwan.claude-dev"))
					return err == nil
				default:
					_, err := os.Stat(filepath.Join(home, ".config/Code/User/globalStorage/saoudrizwan.claude-dev"))
					return err == nil
				}
			},
		},
		{
			Name:     "continue",
			Filename: "config.json",
			RootKey:  "mcpServers",
			IsArray:  true,
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"name":    "git-courer",
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".continue/config.json"),
			},
			Detect: func() bool {
				_, err := os.Stat(filepath.Join(home, ".continue"))
				return err == nil
			},
		},
		{
			Name:     "vscode",
			Filename: "mcp.json",
			RootKey:  "servers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: func() []string {
				switch osName {
				case "darwin":
					return []string{filepath.Join(home, "Library/Application Support/Code/User/mcp.json")}
				case "windows":
					return []string{filepath.Join(os.Getenv("APPDATA"), "Code/User/mcp.json")}
				default:
					return []string{filepath.Join(home, ".config/Code/User/mcp.json")}
				}
			}(),
			Detect: func() bool {
				if _, err := exec.LookPath("code"); err == nil {
					return true
				}
				_, err := exec.LookPath("code-insiders")
				return err == nil
			},
		},
		{
			Name:     "codex",
			Filename: "config.toml",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".codex/config.toml"),
			},
			Detect: func() bool {
				_, err := exec.LookPath("codex")
				return err == nil
			},
		},
		{
			Name:     "zed",
			Filename: "settings.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: func() []string {
				switch osName {
				case "darwin":
					return []string{filepath.Join(home, "Library/Application Support/Zed/settings.json")}
				case "windows":
					return []string{filepath.Join(os.Getenv("APPDATA"), "Zed/settings.json")}
				default:
					return []string{filepath.Join(home, ".config/zed/settings.json")}
				}
			}(),
			Detect: func() bool {
				if _, err := exec.LookPath("zed"); err == nil {
					return true
				}
				_, err := os.Stat(filepath.Join(home, ".config/zed"))
				return err == nil
			},
		},
		{
			Name:     "claude-desktop",
			Filename: "claude_desktop_config.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: func() []string {
				switch osName {
				case "darwin":
					return []string{filepath.Join(home, "Library/Application Support/Claude/claude_desktop_config.json")}
				case "windows":
					return []string{filepath.Join(os.Getenv("APPDATA"), "Claude/claude_desktop_config.json")}
				default:
					return []string{}
				}
			}(),
			Detect: func() bool {
				switch osName {
				case "darwin":
					_, err := os.Stat(filepath.Join(home, "Library/Application Support/Claude"))
					return err == nil
				case "windows":
					_, err := os.Stat(filepath.Join(os.Getenv("APPDATA"), "Claude"))
					return err == nil
				default:
					return false
				}
			},
		},
		{
			Name:     "roo-code",
			Filename: "cline_mcp_settings.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: rooCodePaths(),
			Detect: func() bool {
				switch osName {
				case "darwin":
					_, err := os.Stat(filepath.Join(home, "Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline"))
					return err == nil
				case "windows":
					_, err := os.Stat(filepath.Join(os.Getenv("APPDATA"), "Code/User/globalStorage/rooveterinaryinc.roo-cline"))
					return err == nil
				default:
					_, err := os.Stat(filepath.Join(home, ".config/Code/User/globalStorage/rooveterinaryinc.roo-cline"))
					return err == nil
				}
			},
		},
		// Gemini CLI support
		{
			Name:     "gemini",
			Filename: "settings.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".gemini/settings.json"),
			},
			Detect: func() bool {
				if _, err := exec.LookPath("gemini"); err == nil {
					return true
				}
				_, err := os.Stat(filepath.Join(home, ".gemini"))
				return err == nil
			},
		},
	}
}

func homeDir() string {
	u, _ := user.Current()
	return u.HomeDir
}

func clinePaths() []string {
	home := homeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{filepath.Join(home, "Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json")}
	case "windows":
		return []string{filepath.Join(os.Getenv("APPDATA"), "Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json")}
	default:
		return []string{filepath.Join(home, ".config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json")}
	}
}

func rooCodePaths() []string {
	home := homeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{filepath.Join(home, "Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/cline_mcp_settings.json")}
	case "windows":
		return []string{filepath.Join(os.Getenv("APPDATA"), "Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/cline_mcp_settings.json")}
	default:
		return []string{filepath.Join(home, ".config/Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/cline_mcp_settings.json")}
	}
}

// ConfigureMCP configures git-courer for the given MCP client.
func ConfigureMCP(client *MCPClient, binPath string) error {
	// Find existing config or use default path
	configPath := client.Paths[0]
	for _, path := range client.Paths {
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	entry := client.ConfigFn(binPath)

	// Check if already configured
	if data, err := os.ReadFile(configPath); err == nil {
		if containsGitCourer(string(data)) {
			return nil // Already configured
		}
	}

	// Handle array format (continue)
	if client.IsArray {
		return configureArrayFormat(configPath, entry)
	}

	// Handle object format
	return configureObjectFormat(configPath, client.RootKey, entry)
}

func containsGitCourer(data string) bool {
	return strings.Contains(data, "git-courer")
}

func configureObjectFormat(configPath, rootKey string, entry map[string]interface{}) error {
	// Use map[string]interface{} to preserve unknown user config
	config := make(map[string]interface{})

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &config)
	}

	// Get or create root key
	var rootMap map[string]interface{}
	if existing, ok := config[rootKey]; !ok {
		rootMap = make(map[string]interface{})
		config[rootKey] = rootMap
	} else {
		// Safe type check - avoid panic
		rootMap, ok = existing.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid config: %s is not an object", rootKey)
		}
	}

	rootMap["git-courer"] = entry

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

func configureArrayFormat(configPath string, entry map[string]interface{}) error {
	// Use map to preserve unknown user config (not just mcpServers)
	config := make(map[string]interface{})
	var servers []map[string]interface{}

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &config)
		// Preserve existing servers array
		if srv, ok := config["mcpServers"]; ok {
			if s, ok := srv.([]map[string]interface{}); ok {
				servers = s
			}
		}
	}

	// Check if already exists
	for i, srv := range servers {
		if name, ok := srv["name"]; ok && name == "git-courer" {
			// Replace existing entry with updated config
			servers[i] = entry
			config["mcpServers"] = servers
			data, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}
			return os.WriteFile(configPath, data, 0644)
		}
	}

	servers = append(servers, entry)
	config["mcpServers"] = servers

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

// ConfigureAllMCP configures git-courer for all detected MCP clients.
func ConfigureAllMCP(binPath string) (int, error) {
	clients := DetectClients()
	if len(clients) == 0 {
		return 0, fmt.Errorf("no MCP clients detected")
	}

	var configured int
	for _, client := range clients {
		if err := ConfigureMCP(client, binPath); err == nil {
			configured++
		}
	}

	return configured, nil
}

// SetupClient sets up git-courer for a specific client.
func SetupClient(clientName, binPath string) error {
	clients := MCPClients()
	for _, client := range clients {
		if client.Name == clientName {
			return ConfigureMCP(client, binPath)
		}
	}
	return fmt.Errorf("unknown client: %s", clientName)
}
