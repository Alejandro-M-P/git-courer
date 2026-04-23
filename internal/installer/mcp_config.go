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

	_ "embed" // Required for //go:embed
)

//go:embed prompts/agent-instructions.md
var embeddedInstructions string

// readInstructions returns embedded instructions.
// Uses //go:embed so it works from any location (installed binary).
func readInstructions() ([]byte, error) {
	return []byte(embeddedInstructions), nil
}

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

// RuleFile represents an agent's instruction/rules file.
type RuleFile struct {
	Name     string
	Path    string
	Content string
}

// GetRuleFiles returns the rule files to create for all agents.
func GetRuleFiles(binPath string) ([]RuleFile, error) {
	home := homeDir()

	content, err := readInstructions()
	if err != nil {
		return nil, fmt.Errorf("failed to read instructions: %w", err)
	}

	strContent := string(content)

	return []RuleFile{
		{
			Name:     "claude-code",
			Path:    filepath.Join(home, "CLAUDE.md"),
			Content: strContent,
		},
		{
			Name:     "cursor",
			Path:    filepath.Join(home, ".cursorrules"),
			Content: strContent,
		},
		{
			Name:     "windsurf",
			Path:    filepath.Join(home, ".windsurfrules"),
			Content: strContent,
		},
		{
			Name:     "cline",
			Path:    filepath.Join(home, ".clinerules", "rules.md"),
			Content: strContent,
		},
		{
			Name:     "zed",
			Path:    filepath.Join(home, ".zed", "rules.md"),
			Content: strContent,
		},
		{
			Name:     "codex",
			Path:    filepath.Join(home, "AGENTS.md"),
			Content: strContent,
		},
		{
			Name:     "opencode-skill",
			Path:    filepath.Join(home, ".config", "opencode", "skills", "git-courer", "SKILL.md"),
			Content: strContent,
		},
		{
			Name:     "opencode-plugin",
			Path:    filepath.Join(home, ".config", "opencode", "plugins", "git-courer.js"),
			Content: getPluginJS(binPath),
		},
		// Gemini CLI support
		{
			Name:     "gemini",
			Path:    filepath.Join(home, ".gemini", "GEMINI.md"),
			Content: strContent,
		},
	}, nil
}

func getPluginJS(binPath string) string {
	return `// Git-Courer plugin for OpenCode
exports.gitCourer = async ({ project, client, $, directory, worktree }) => {
  return {
    "tool.execute.before": async (input, output) => {
      // git-courer context hooks
    }
  };
};
`
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

	if _, exists := rootMap["git-courer"]; exists {
		// Replace existing entry with updated config
		rootMap["git-courer"] = entry
	} else {
		rootMap["git-courer"] = entry
	}

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
