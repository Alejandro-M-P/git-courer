// Package installer provides installation and management for git-courer.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
)

// gitCourerMdFilename is the golden-rules file written alongside each MCP
// client config so agents always see the status/diff/pr-review workflow.
const gitCourerMdFilename = "GIT_COURER.md"

// gitCourerMdContent is the content written to GIT_COURER.md during MCP
// configuration. It distills the golden rules from the MCP server summary so
// agents loading the config directory see them without an MCP initialize.
const gitCourerMdContent = `# git-courer — Golden Rules

git-courer is NOT a wrapper around git. Some tools do things raw git CANNOT.
Others are structured replacements that return JSON instead of human text.

## Golden Rules — save tokens and prevent mistakes

1. BEFORE any mutation → status (know the repo state)
2. BEFORE push → diff + review
3. BEFORE PR → pr-review (all checks in one call)

## Tool map (use these instead of raw bash git)

| git command       | git-courer MCP tool | why |
|-------------------|---------------------|-----|
| git status        | status              | complete repo state in one call |
| git diff          | diff                | AST-labeled diffs — know WHAT changed |
| git commit        | commit              | LLM pipeline — atomic commits by dependency graph |
| git log/show/blame| history             | structured JSON, no pager hangs |
| git branch/switch | branch              | structured, auto-stash, safety gates |
| git merge/rebase  | integrate           | structured conflict detection |
| git revert/reset  | rewrite             | auto-backup before mutation |
| git stash         | stash               | structured JSON |
| git push/pull/fetch| sync               | PUSH is irreversible — safety gates |
| git add/restore   | stage               | structured, binary-file interception |

Source of truth: ` + descriptions.GitCourerSummary + `
`

// MCPClient represents an MCP client configuration.
type MCPClient struct {
	Name              string
	Filename          string // config filename
	RootKey           string // mcpServers, mcp, servers, context_servers
	IsArray           bool   // continue uses array format
	ConfigFormat      string // "json" (default) or "toml"
	ConfigFn          func(binPath string) map[string]interface{}
	PostInstallNotice func(binPath string) string // optional post-install warning check
	Paths             []string                    // possible config file paths for this platform
	Detect            func() bool
	HooksConfig       *HooksConfig // optional; non-nil for clients that support hooks (e.g. Codex)
}

// MCPServerConfig represents an MCP server entry.
type MCPServerConfig struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Type    string   `json:"type,omitempty"`
}

// HooksConfig describes where a client's hooks file lives and what format it
// uses. When non-nil on an MCPClient, ConfigureMCP installs the git-courer
// PreToolUse hook there (and RunDoctor reports its real status).
type HooksConfig struct {
	Path   string // e.g. "/home/u/.codex/hooks.json"
	Format string // "json"
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
			Name:         "codex",
			Filename:     "config.toml",
			ConfigFormat: "toml",
			RootKey:      "mcpServers",
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
			HooksConfig: &HooksConfig{
				Path:   filepath.Join(home, ".codex/hooks.json"),
				Format: "json",
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
			// Already configured — still ensure GIT_COURER.md exists (idempotent).
			ensureGitCourerMd(configPath)
			// Install hooks if the client supports them (idempotent no-op if
			// already installed). Even on the early-return path we must keep
			// hooks in sync with the configured client.
			if client.HooksConfig != nil {
				if err := installHook(client); err != nil {
					return fmt.Errorf("install hook for %s: %w", client.Name, err)
				}
			}
			return nil // Already configured
		}
	}

	// Backup existing config before mutation (only if it exists).
	if _, err := os.Stat(configPath); err == nil {
		if backupErr := backupConfig(configPath); backupErr != nil {
			return fmt.Errorf("failed to backup config: %w", backupErr)
		}
	}

	var err error
	// Handle array format (continue)
	if client.IsArray {
		err = configureArrayFormat(configPath, entry)
	} else if client.ConfigFormat == "toml" {
		err = configureTomlFormat(configPath, client.RootKey, entry)
	} else {
		// Handle object format (JSON)
		err = configureObjectFormat(configPath, client.RootKey, entry)
	}

	if err != nil {
		return err
	}

	// Write GIT_COURER.md golden rules alongside the config (idempotent).
	ensureGitCourerMd(configPath)

	// Install hooks if the client supports them (idempotent no-op if already
	// installed). Done AFTER the config is written so a failing hook install
	// does not leave a half-configured MCP entry without a rollback path.
	if client.HooksConfig != nil {
		if err := installHook(client); err != nil {
			return fmt.Errorf("install hook for %s: %w", client.Name, err)
		}
	}

	if client.PostInstallNotice != nil {
		if msg := client.PostInstallNotice(binPath); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
	}

	return nil
}

// ensureGitCourerMd writes GIT_COURER.md in the same directory as configPath
// if it does not already exist. Idempotent — an existing file is preserved.
func ensureGitCourerMd(configPath string) {
	rulesPath := filepath.Join(filepath.Dir(configPath), gitCourerMdFilename)
	if _, err := os.Stat(rulesPath); err == nil {
		return // already exists — do not overwrite user content
	}
	_ = os.WriteFile(rulesPath, []byte(gitCourerMdContent), 0644)
}

// backupConfig copies configPath to configPath + ".bak" so RunUninstall can
// restore it. This is a simple read→write copy — atomicity is not required
// because the file is small and local.
func backupConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath+".bak", data, 0644)
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

// configureTomlFormat writes a TOML config file with the given rootKey and
// entry. It reads any existing TOML, merges the git-courer entry in, and
// writes back as valid TOML. This is needed for clients like Codex that
// expect TOML (config.toml) instead of JSON.
func configureTomlFormat(configPath, rootKey string, entry map[string]interface{}) error {
	// Read existing file content to preserve non-MCP sections.
	existing := ""
	if data, err := os.ReadFile(configPath); err == nil {
		existing = string(data)
	}

	// Build the git-courer section as TOML key-value pairs.
	var tomlLines []string
	tomlLines = append(tomlLines, fmt.Sprintf("[%s.git-courer]", rootKey))

	// Sort keys for deterministic output.
	var keys []string
	for k := range entry {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := entry[k]
		switch val := v.(type) {
		case string:
			tomlLines = append(tomlLines, fmt.Sprintf("%s = %q", k, val))
		case []string:
			// TOML inline array: ["a", "b"]
			parts := make([]string, len(val))
			for i, s := range val {
				parts[i] = fmt.Sprintf("%q", s)
			}
			tomlLines = append(tomlLines, fmt.Sprintf("%s = [%s]", k, strings.Join(parts, ", ")))
		case []interface{}:
			parts := make([]string, len(val))
			for i, s := range val {
				parts[i] = fmt.Sprintf("%q", s)
			}
			tomlLines = append(tomlLines, fmt.Sprintf("%s = [%s]", k, strings.Join(parts, ", ")))
		default:
			tomlLines = append(tomlLines, fmt.Sprintf("%s = %v", k, v))
		}
	}

	newSection := strings.Join(tomlLines, "\n")

	// If the file already has a [mcpServers.git-courer] section, replace it.
	if strings.Contains(existing, "[mcpServers.git-courer]") {
		// Find and replace the existing section.
		lines := strings.Split(existing, "\n")
		var result []string
		inSection := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "[mcpServers.git-courer]" {
				inSection = true
				result = append(result, newSection)
				continue
			}
			if inSection {
				// Skip lines until we hit a new section header or end.
				if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
					inSection = false
					result = append(result, line)
				}
				continue
			}
			result = append(result, line)
		}
		return os.WriteFile(configPath, []byte(strings.Join(result, "\n")+"\n"), 0644)
	}

	// No existing section — append.
	if existing != "" {
		newSection = "\n" + newSection
	}
	return os.WriteFile(configPath, []byte(existing+newSection+"\n"), 0644)
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
