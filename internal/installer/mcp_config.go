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
					"command": []string{binPath, "mcp"},
					"enabled": true,
				}
			},
			Paths: []string{
				filepath.Join(home, ".config/opencode/opencode.json"),
			},
			Detect: func() bool {
				_, err := exec.LookPath("opencode")
				return err == nil
			},
		},
		{
			Name:     "claude-code",
			Filename: "settings.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".claude/settings.json"),
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
				_, err := exec.LookPath("cursor")
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
			Paths: []string{
				filepath.Join(home, ".vscode/mcp.json"),
			},
			Detect: func() bool {
				// VS Code with Copilot
				_, err := exec.LookPath("code")
				return err == nil
			},
		},
		{
			Name:     "codex",
			Filename: "mcp.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".codex/mcp.json"),
			},
			Detect: func() bool {
				_, err := exec.LookPath("codex")
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
			Paths: claudeDesktopPaths(),
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

func claudeDesktopPaths() []string {
	home := homeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{filepath.Join(home, "Library/Application Support/Claude/claude_desktop_config.json")}
	case "windows":
		return []string{filepath.Join(os.Getenv("APPDATA"), "Claude/claude_desktop_config.json")}
	default:
		return nil
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
	var config map[string]interface{}

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &config)
	}

	// Get or create root key
	root := config[rootKey]
	if root == nil {
		root = make(map[string]interface{})
		config[rootKey] = root
	}

	rootMap := root.(map[string]interface{})
	if _, exists := rootMap["git-courer"]; exists {
		return nil // Already configured
	}

	rootMap["git-courer"] = entry

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

func configureArrayFormat(configPath string, entry map[string]interface{}) error {
	type ContinueConfig struct {
		MCPServers []map[string]interface{} `json:"mcpServers,omitempty"`
	}

	var cfg ContinueConfig

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &cfg)
	}

	// Check if already exists
	for _, srv := range cfg.MCPServers {
		if name, ok := srv["name"]; ok && name == "git-courer" {
			return nil
		}
	}

	cfg.MCPServers = append(cfg.MCPServers, entry)

	data, err := json.MarshalIndent(cfg, "", "  ")
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
		if err := ConfigureMCP(client, binPath); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ %s: %v\n", client.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s configured\n", client.Name)
		configured++
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
