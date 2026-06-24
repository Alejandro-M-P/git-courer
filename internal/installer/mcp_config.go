// Package installer provides installation and management for git-courer.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// MCPClient represents an MCP client configuration.
type MCPClient struct {
	Name              string
	Filename          string // config filename
	RootKey           string // mcpServers, mcp, servers, context_servers
	IsArray           bool   // continue uses array format
	ConfigFn          func(binPath string) map[string]interface{}
	PostInstallNotice func(binPath string) string // optional post-install warning check
	Paths             []string                    // possible config file paths for this platform
	Detect            func() bool
}

// MCPServerConfig represents an MCP server entry.
type MCPServerConfig struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Type    string   `json:"type,omitempty"`
}

var getMCPClients = MCPClients
var lookPath = exec.LookPath
var osStat = os.Stat
var homeDirFn = homeDir

// DetectClients returns a list of detected MCP clients on the system.
func DetectClients() []*MCPClient {
	var detected []*MCPClient
	for _, client := range getMCPClients() {
		if client.Detect() {
			detected = append(detected, client)
		}
	}
	return detected
}

// MCPClients returns all supported MCP clients.
func MCPClients() []*MCPClient {
	home := homeDirFn()

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
			Name:     "pi",
			Filename: "mcp.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			PostInstallNotice: func(binPath string) string {
				settingsPath := filepath.Join(homeDirFn(), ".pi/agent/settings.json")
				data, err := os.ReadFile(settingsPath)
				if err != nil {
					return "pi Agent: extension 'pi-mcp-adapter' not found. Run 'pi install npm:pi-mcp-adapter' and restart pi."
				}
				var settings struct {
					Packages []string `json:"packages"`
				}
				if err := json.Unmarshal(data, &settings); err != nil {
					return "pi Agent: could not read settings. Run 'pi install npm:pi-mcp-adapter' and restart pi."
				}
				for _, pkg := range settings.Packages {
					if pkg == "npm:pi-mcp-adapter" {
						return "" // installed
					}
				}
				return "pi Agent: extension 'pi-mcp-adapter' not installed. Run 'pi install npm:pi-mcp-adapter' and restart pi."
			},
			Paths: []string{
				filepath.Join(home, ".pi/agent/mcp.json"),
			},
			Detect: func() bool {
				if _, err := lookPath("pi"); err == nil {
					return true
				}
				_, err := osStat(filepath.Join(home, ".pi/agent"))
				return err == nil
			},
		},
		{
			Name:     "antigravity",
			Filename: "mcp_config.json",
			RootKey:  "mcpServers",
			ConfigFn: func(binPath string) map[string]interface{} {
				return map[string]interface{}{
					"command": binPath,
					"args":    []string{"mcp"},
				}
			},
			Paths: []string{
				filepath.Join(home, ".gemini/antigravity-cli/mcp_config.json"),
			},
			Detect: func() bool {
				_, err := lookPath("agy")
				return err == nil
			},
		},
	}
}

func homeDir() string {
	u, _ := user.Current()
	return u.HomeDir
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

	var err error
	// Handle array format (continue)
	if client.IsArray {
		err = configureArrayFormat(configPath, entry)
	} else {
		// Handle object format
		err = configureObjectFormat(configPath, client.RootKey, entry)
	}

	if err != nil {
		return err
	}

	if client.PostInstallNotice != nil {
		if msg := client.PostInstallNotice(binPath); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
	}

	return nil
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
	clients := getMCPClients()
	var configured bool
	for _, client := range clients {
		if client.Name == clientName && client.Detect() {
			if err := ConfigureMCP(client, binPath); err != nil {
				return err
			}
			configured = true
		}
	}
	if !configured {
		return fmt.Errorf("unknown client: %s", clientName)
	}
	return nil
}
