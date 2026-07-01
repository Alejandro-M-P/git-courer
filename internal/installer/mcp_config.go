// Package installer provides installation and management for git-courer.
package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// gitCourerMdFilename is the golden-rules file written alongside each MCP
// client config so agents always see the status/diff/pr-review workflow.
const gitCourerMdFilename = "GIT_COURER.md"

const (
	promptBlockStartDelimiter = "<!-- git-courer start -->"
	promptBlockEndDelimiter   = "<!-- git-courer end -->"
)

// gitCourerMdContent is the golden rules content injected into instructions.
// Shared with hooks.go via GoldenRulesAdditionalContext.
const gitCourerMdContent = "# git-courer — Golden Rules\n" +
	"\n" +
	"git-courer is NOT a wrapper around git. Some tools do things raw git CANNOT.\n" +
	"Others are structured replacements that return JSON instead of human text.\n" +
	"\n" +
	"## Golden Rules — save tokens and prevent mistakes\n" +
	"\n" +
	"0. On session start (MANDATORY) → ALWAYS run `session start <branch_name>` to create your isolated worktree on a custom branch.\n" +
	"1. Immediately after start → run `session select <session_id>` to redirect all MCP operations to the session worktree.\n" +
	"2. Work location → perform all file reads, writes, and commands strictly inside the designated session worktree path.\n" +
	"3. Before any mutation → ALWAYS check `status` to know the repository state and identify active changes.\n" +
	"4. Before push or PR (or when verifying changes) → ALWAYS check `diff` + `review` to verify active diff checks.\n" +
	"5. Before PR → ALWAYS run `pr-review` to run all checks and verify changes in a single call."

// MCPClient represents an MCP client configuration.
type MCPClient struct {
	Name              string
	Filename          string       // config filename
	RootKey           string       // mcpServers, mcp, servers, context_servers
	IsArray           bool         // continue uses array format
	ConfigFormat      string       // "json" (default) or "toml"
	HooksConfig       *HooksConfig // nil = no hooks for this client
	ConfigFn          func(binPath string) map[string]interface{}
	PostInstallNotice func(binPath string) string // optional post-install warning check
	Paths             []string                    // possible config file paths for this platform
	Detect            func() bool
}

// HooksConfig specifies hook installation for an MCP client.
//
// A client may use one or more of these hook storage strategies:
//   - HooksPath: a separate hooks file (Codex stores hooks in
//     "~/.codex/hooks.json" — managed via installHook/RemoveHook).
//   - SettingsPath: inline hooks inside a settings file (Claude Code stores
//     hooks inline in "~/.claude/settings.json" — managed via
//     installClaudeHooks/removeClaudeHooks).
//   - PermissionsPath: a separate settings.json holding declarative
//     permissions (Antigravity stores permissions in
//     "~/.gemini/antigravity-cli/settings.json" — managed via
//     installAntigravityPermissions/removeAntigravityPermissions).
//
// At most one strategy path is meaningful per client; empty string means the
// strategy is not used for this client.
type HooksConfig struct {
	HooksPath       string // e.g. "~/.codex/hooks.json" (Codex — separate file)
	SettingsPath    string // e.g. "~/.claude/settings.json" (Claude Code — inline hooks)
	PermissionsPath string // e.g. "~/.gemini/antigravity-cli/settings.json" (Antigravity — permissions file)
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
			HooksConfig: &HooksConfig{
				SettingsPath: filepath.Join(home, ".claude/settings.json"),
			},
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
			RootKey:      "mcp_servers",
			ConfigFormat: "toml",
			HooksConfig: &HooksConfig{
				HooksPath: filepath.Join(home, ".codex/hooks.json"),
			},
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
			HooksConfig: &HooksConfig{
				HooksPath:       filepath.Join(home, ".gemini/antigravity-cli/hooks.json"),
				PermissionsPath: filepath.Join(home, ".gemini/antigravity-cli/settings.json"),
			},
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
			// Already configured — still ensure prompt block is injected/updated.
			if err := ensurePromptBlock(client); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to ensure prompt block: %v\n", err)
			}
			// Delete old physical GIT_COURER.md if present
			_ = os.Remove(filepath.Join(filepath.Dir(configPath), gitCourerMdFilename))

			if client.Name == "opencode" {
				if policyErr := configureOpenCodePolicy(configPath); policyErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to apply OpenCode policy: %v\n", policyErr)
				}
			}
			// Also ensure hooks are installed (idempotent).
			if client.HooksConfig != nil {
				if client.HooksConfig.PermissionsPath != "" {
					// Antigravity-style client.
					if client.HooksConfig.HooksPath != "" {
						if hookErr := installAntigravityHooks(client.HooksConfig.HooksPath, binPath); hookErr != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to install Antigravity hooks: %v\n", hookErr)
						}
					}
					if permErr := installAntigravityPermissions(client.HooksConfig.PermissionsPath, binPath); permErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to install Antigravity permissions: %v\n", permErr)
					}
				} else {
					if client.HooksConfig.HooksPath != "" {
						if hookErr := installHook(client.HooksConfig.HooksPath, binPath); hookErr != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to install hooks: %v\n", hookErr)
						}
					}
					if client.HooksConfig.SettingsPath != "" {
						if hookErr := installClaudeHooks(client.HooksConfig.SettingsPath, binPath); hookErr != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to install Claude hooks: %v\n", hookErr)
						}
					}
				}
			}
			// Pi-style client: install @pi-lab/permissions deny rules into
			// ~/.pi/agent/settings.json. Pi has no lifecycle hooks config, so it
			// is handled here like opencode (policy-only).
			if client.Name == "pi" {
				if permErr := installPiPermissions(piPermissionsPath(), binPath); permErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to install Pi permissions: %v\n", permErr)
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
	} else {
		// Handle object format — pass ConfigFormat for TOML support
		err = configureObjectFormatWithFormat(configPath, client.RootKey, entry, client.ConfigFormat)
	}

	if err != nil {
		return err
	}

	// Ensure prompt block is injected/updated in target file.
	if err := ensurePromptBlock(client); err != nil {
		return err
	}
	// Delete old physical GIT_COURER.md if present
	_ = os.Remove(filepath.Join(filepath.Dir(configPath), gitCourerMdFilename))

	// Apply the OpenCode policy (permission.bash "git *": "deny" and
	// instructions AGENTS.md path). Idempotent merge.
	if client.Name == "opencode" {
		if policyErr := configureOpenCodePolicy(configPath); policyErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to apply OpenCode policy: %v\n", policyErr)
		}
	}

	// Install hooks for clients that have HooksConfig.
	if client.HooksConfig != nil {
		if client.HooksConfig.PermissionsPath != "" {
			// Antigravity-style client: separate hooks.json with the
			// run_command matcher (not Bash) plus a separate settings.json
			// for declarative permissions. The Antigravity hooks function
			// is distinct from installHook because of the matcher and the
			// PreInvocation event it adds.
			if client.HooksConfig.HooksPath != "" {
				if hookErr := installAntigravityHooks(client.HooksConfig.HooksPath, binPath); hookErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to install Antigravity hooks: %v\n", hookErr)
				}
			}
			if permErr := installAntigravityPermissions(client.HooksConfig.PermissionsPath, binPath); permErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to install Antigravity permissions: %v\n", permErr)
			}
		} else {
			// Codex / Claude Code style.
			if client.HooksConfig.HooksPath != "" {
				if hookErr := installHook(client.HooksConfig.HooksPath, binPath); hookErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to install hooks: %v\n", hookErr)
				}
			}
			if client.HooksConfig.SettingsPath != "" {
				if hookErr := installClaudeHooks(client.HooksConfig.SettingsPath, binPath); hookErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to install Claude hooks: %v\n", hookErr)
				}
			}
		}
	}

	// Pi-style client: install @pi-lab/permissions deny rules into
	// ~/.pi/agent/settings.json. Pi has no lifecycle hooks config, so it is
	// handled here like opencode (policy-only).
	if client.Name == "pi" {
		if permErr := installPiPermissions(piPermissionsPath(), binPath); permErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to install Pi permissions: %v\n", permErr)
		}
	}

	if client.PostInstallNotice != nil {
		if msg := client.PostInstallNotice(binPath); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
	}

	return nil
}

// GetInstructionsPath returns the path to the instructions file for the client,
// resolving relative to the resolved config file.
func (c *MCPClient) GetInstructionsPath() string {
	configPath := c.Paths[0]
	for _, path := range c.Paths {
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}
	dir := filepath.Dir(configPath)
	switch c.Name {
	case "claude-code":
		return filepath.Join(dir, ".claude/CLAUDE.md")
	case "antigravity":
		return filepath.Join(filepath.Dir(dir), "GEMINI.md")
	default:
		return filepath.Join(dir, "AGENTS.md")
	}
}

func injectOrUpdateBlock(filePath string, rulesContent string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create instructions dir: %w", err)
	}

	wrapped := promptBlockStartDelimiter + "\n" + rulesContent + "\n" + promptBlockEndDelimiter

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return os.WriteFile(filePath, []byte(wrapped), 0644)
	} else if err != nil {
		return fmt.Errorf("failed to read instructions file: %w", err)
	}

	content := string(data)
	startIdx := strings.Index(content, promptBlockStartDelimiter)
	endIdx := strings.Index(content, promptBlockEndDelimiter)

	if startIdx != -1 && endIdx != -1 && startIdx < endIdx {
		before := content[:startIdx]
		after := content[endIdx+len(promptBlockEndDelimiter):]
		newContent := before + wrapped + after
		return os.WriteFile(filePath, []byte(newContent), 0644)
	}

	newContent := content
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		newContent += "\n"
	}
	newContent += wrapped
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

func removeBlock(filePath string) error {
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read instructions file: %w", err)
	}

	content := string(data)
	startIdx := strings.Index(content, promptBlockStartDelimiter)
	endIdx := strings.Index(content, promptBlockEndDelimiter)

	if startIdx != -1 && endIdx != -1 && startIdx < endIdx {
		before := content[:startIdx]
		after := content[endIdx+len(promptBlockEndDelimiter):]
		if strings.HasSuffix(before, "\n") && strings.HasPrefix(after, "\n") {
			after = after[1:]
		}
		newContent := before + after
		return os.WriteFile(filePath, []byte(newContent), 0644)
	}

	return nil
}

func ensurePromptBlock(client *MCPClient) error {
	path := client.GetInstructionsPath()
	return injectOrUpdateBlock(path, gitCourerMdContent)
}

func RemovePromptBlock(client *MCPClient) error {
	path := client.GetInstructionsPath()
	return removeBlock(path)
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

// configureTomlFormat writes the MCP server entry as TOML with the format
// [rootKey."git-courer"]. It preserves any existing content in the file.
func configureTomlFormat(configPath, rootKey string, entry map[string]interface{}) error {
	// Read existing config to preserve user content.
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		if err := toml.Unmarshal(data, &existing); err != nil {
			// If we can't parse existing TOML, start fresh.
			existing = make(map[string]interface{})
		}
	}

	// Get or create the root key section.
	var rootMap map[string]interface{}
	if r, ok := existing[rootKey]; ok {
		if m, ok := r.(map[string]interface{}); ok {
			rootMap = m
		} else {
			rootMap = make(map[string]interface{})
		}
	} else {
		rootMap = make(map[string]interface{})
	}
	existing[rootKey] = rootMap

	rootMap["git-courer"] = entry

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(existing); err != nil {
		return fmt.Errorf("failed to encode TOML: %w", err)
	}
	return os.WriteFile(configPath, buf.Bytes(), 0644)
}

func configureObjectFormat(configPath, rootKey string, entry map[string]interface{}) error {
	return configureObjectFormatWithFormat(configPath, rootKey, entry, "json")
}

func configureObjectFormatWithFormat(configPath, rootKey string, entry map[string]interface{}, format string) error {
	if format == "toml" {
		return configureTomlFormat(configPath, rootKey, entry)
	}
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

// configureOpenCodePolicy merges the git-courer policy into opencode.json:
//   - permission.bash gains 23 granular "git {sub}": "deny" entries (one per
//     covered subcommand) plus a "git *": "ask" fallback. The 23 deny entries
//     are serialized BEFORE "git *": "ask" so OpenCode's last-match-wins
//     resolution prefers the specific deny over the wildcard ask.
//   - instructions array includes the path to GIT_COURER.md in the same
//     directory as configPath (deduplicated; if instructions is a string it
//     is converted to an array preserving the original entry).
//
// Ordering caveat: permission.bash is serialized as an ORDERED object
// (deny entries first, "git *" last) via a custom raw-JSON builder. A plain
// map[string]interface{} would sort keys alphabetically and place "git *"
// before "git add" (because '*' < 'a'), which breaks last-match-wins.
//
// Behavior:
//   - If opencode.json does not exist, a fresh config with the policy is
//     written.
//   - If opencode.json exists, it is backed up to configPath + ".bak" before
//     any mutation.
//   - If opencode.json exists but is unparseable JSON, it is backed up and a
//     fresh config with the policy is written.
//   - Idempotent: running twice produces byte-identical output.
func configureOpenCodePolicy(configPath string) error {
	// Ensure the config directory exists.
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	// Read existing config (if any) into a generic map to preserve unknown keys.
	config := make(map[string]interface{})
	fileExisted := false
	parsedOK := false
	if data, err := os.ReadFile(configPath); err == nil {
		fileExisted = true
		if jsonErr := json.Unmarshal(data, &config); jsonErr != nil {
			// Unparseable — back up the original bytes and start fresh so we
			// can still apply the policy without silently clobbering the
			// user's file.
			if backupErr := backupConfig(configPath); backupErr != nil {
				return fmt.Errorf("failed to backup unparseable config: %w", backupErr)
			}
			config = make(map[string]interface{})
		} else {
			parsedOK = true
		}
	}

	// Backup existing valid config before mutation (only if it parsed and we
	// have not already backed up the unparseable case above).
	if fileExisted && parsedOK {
		if backupErr := backupConfig(configPath); backupErr != nil {
			return fmt.Errorf("failed to backup config: %w", backupErr)
		}
	}

	// permission.bash: 23 granular "git {sub}": "deny" + "git *": "ask".
	// Existing non-git-courer bash keys are preserved and emitted AFTER the
	// git-courer deny entries but BEFORE the "git *" ask fallback, so a
	// user's own wildcard rules (e.g. "*": "allow") do not shadow the deny
	// entries. "git *" is always last for last-match-wins.
	perm, _ := config["permission"].(map[string]interface{})
	if perm == nil {
		perm = make(map[string]interface{})
		config["permission"] = perm
	}
	existingBash, _ := perm["bash"].(map[string]interface{})

	// Build the ordered list of bash key/value pairs.
	// 1. The 23 git-courer deny entries (deterministic order).
	// 2. Existing user bash keys that are NOT git-courer-owned (preserved).
	// 3. "git *": "ask" fallback (always last).
	orderedBash := buildOpenCodeBashOrdered(existingBash)
	// Replace the map with an ordered raw-JSON RawMessage so json.MarshalIndent
	// of the parent config preserves this order.
	perm["bash"] = json.RawMessage(orderedBash)

	// instructions array includes AGENTS.md path, and old GIT_COURER.md path is removed.
	agentsPath := filepath.Join(filepath.Dir(configPath), "AGENTS.md")

	switch raw := config["instructions"].(type) {
	case string:
		arr := []interface{}{}
		if filepath.Base(raw) != gitCourerMdFilename {
			arr = append(arr, raw)
		}
		if raw != agentsPath {
			arr = append(arr, agentsPath)
		} else if len(arr) == 0 {
			arr = append(arr, agentsPath)
		}
		config["instructions"] = arr
	case []interface{}:
		var kept []interface{}
		already := false
		for _, v := range raw {
			if s, ok := v.(string); ok {
				if filepath.Base(s) == gitCourerMdFilename {
					continue
				}
				if s == agentsPath {
					already = true
					kept = append(kept, v)
					continue
				}
			}
			kept = append(kept, v)
		}
		if !already {
			kept = append(kept, agentsPath)
		}
		config["instructions"] = kept
	default:
		config["instructions"] = []interface{}{agentsPath}
	}

	// Marshal the config, but force the bash RawMessage to be written inline
	// (json.MarshalIndent already inlines RawMessage). We must ensure the
	// RawMessage is pretty-printed consistently with the rest. Build the bash
	// object text already indented for two levels (permission → bash).
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal opencode.json: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

// openCodeDenySubcommands is the ordered list of the 23 git subcommands that
// git-courer covers with an MCP tool. Each becomes a "git {sub}": "deny"
// entry in permission.bash. The order matches the mcpTools coverage so the
// deny block reads predictably.
var openCodeDenySubcommands = []string{
	"status", "diff", "commit", "log", "branch", "merge", "rebase",
	"cherry-pick", "revert", "reset", "stash", "push", "pull", "fetch",
	"blame", "add", "restore", "clean", "rm", "switch", "checkout",
	"worktree", "reflog",
}

// openCodeBashOwnedKeys is the set of permission.bash keys owned by
// git-courer (the 23 deny entries plus the "git *" ask fallback). Used to
// identify and drop stale git-courer entries when rebuilding the ordered
// bash object and when stripping policy on uninstall.
var openCodeBashOwnedKeys = func() map[string]bool {
	m := make(map[string]bool, len(openCodeDenySubcommands)+1)
	for _, sub := range openCodeDenySubcommands {
		m["git "+sub] = true
	}
	m["git *"] = true
	return m
}()

// buildOpenCodeBashOrdered renders the permission.bash object as raw JSON
// bytes with a deterministic key order: the 23 git-courer deny entries first
// (in openCodeDenySubcommands order), then any existing user-owned bash keys
// (preserved in their original map iteration order), then "git *": "ask"
// last. This guarantees last-match-wins resolution in OpenCode regardless of
// Go's alphabetical map sorting on marshal.
//
// existing is the previous permission.bash map (may be nil). User-owned keys
// (anything not in openCodeBashOwnedKeys) are preserved; git-courer-owned keys
// are regenerated from scratch so re-runs are idempotent.
func buildOpenCodeBashOrdered(existing map[string]interface{}) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true

	writeKV := func(key, val string) {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		// Inline a 2-space-indented key:value pair; json.Marshal of the key
		// handles quoting/escaping, and the value is a simple string.
		k, _ := json.Marshal(key)
		v, _ := json.Marshal(val)
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}

	// 1. The 23 git-courer deny entries.
	for _, sub := range openCodeDenySubcommands {
		writeKV("git "+sub, "deny")
	}

	// 2. Existing user-owned bash keys (preserved, emitted in sorted key
	//    order for deterministic idempotent output — Go map iteration is
	//    randomized, so sorting is required).
	userKeys := make([]string, 0, len(existing))
	for k := range existing {
		if openCodeBashOwnedKeys[k] {
			continue // git-courer-owned — regenerated above
		}
		userKeys = append(userKeys, k)
	}
	sort.Strings(userKeys)
	for _, k := range userKeys {
		vBytes, err := json.Marshal(existing[k])
		if err != nil {
			continue // skip unmarshalable values
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false
		kBytes, _ := json.Marshal(k)
		buf.Write(kBytes)
		buf.WriteByte(':')
		buf.Write(vBytes)
	}

	// 3. "git *": "ask" fallback — always last for last-match-wins.
	writeKV("git *", "ask")

	buf.WriteByte('}')
	return buf.Bytes()
}

// removeOpenCodePolicy strips the git-courer policy entries from opencode.json:
//   - permission.bash: the 23 granular "git {sub}": "deny" entries AND the
//     "git *": "ask" fallback are removed (other user-owned keys preserved).
//   - GIT_COURER.md path is removed from the instructions array (other
//     entries preserved).
//
// Behavior:
//   - No-op (file not rewritten, modtime unchanged) if no git-courer policy
//     entries are present.
//   - If opencode.json is unparseable JSON AND no .bak exists, the file is
//     left untouched (do not risk clobbering user config).
//   - Idempotent: running N times yields the same end state.
func removeOpenCodePolicy(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No file — nothing to remove.
		return nil
	}

	config := make(map[string]interface{})
	if err := json.Unmarshal(data, &config); err != nil {
		// Unparseable — leave it alone rather than risk clobbering user config.
		return nil
	}

	// Detect whether any git-courer policy entry exists. If not, no-op.
	hasPolicy := false

	// Check permission.bash for any git-courer-owned key (the 23 deny entries
	// or "git *"). Note: bash may be a json.RawMessage after configure, so
	// handle both the map and RawMessage shapes.
	if perm, ok := config["permission"].(map[string]interface{}); ok {
		switch bashRaw := perm["bash"].(type) {
		case map[string]interface{}:
			for key := range bashRaw {
				if openCodeBashOwnedKeys[key] {
					hasPolicy = true
					delete(bashRaw, key)
				}
			}
		case json.RawMessage:
			// Decode the RawMessage into a map, strip owned keys, and reassign
			// as a plain map so the cleaned output is a normal object.
			var bashMap map[string]interface{}
			if err := json.Unmarshal(bashRaw, &bashMap); err == nil {
				for key := range bashMap {
					if openCodeBashOwnedKeys[key] {
						hasPolicy = true
						delete(bashMap, key)
					}
				}
				if len(bashMap) == 0 {
					delete(perm, "bash")
				} else {
					perm["bash"] = bashMap
				}
			}
		}
	}

	// Check instructions for GIT_COURER.md and AGENTS.md paths.
	agentsPath := filepath.Join(filepath.Dir(configPath), "AGENTS.md")

	if raw, ok := config["instructions"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			var kept []interface{}
			removedAny := false
			for _, v := range arr {
				if s, ok := v.(string); ok && (filepath.Base(s) == gitCourerMdFilename || s == agentsPath) {
					removedAny = true
					hasPolicy = true
					continue
				}
				kept = append(kept, v)
			}
			if removedAny {
				if len(kept) == 0 {
					delete(config, "instructions")
				} else {
					config["instructions"] = kept
				}
			}
		}
	}

	if !hasPolicy {
		// No policy entries — leave the file untouched.
		return nil
	}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cleaned opencode.json: %w", err)
	}
	return os.WriteFile(configPath, out, 0644)
}

// installClaudeHooks installs or updates git-courer hooks inside the Claude
// Code settings.json at settingsPath. Claude Code stores hooks inline in the
// "hooks" object (keyed by event name), so this merges git-courer entries
// into any existing hooks while preserving every non-git-courer hook and
// every other top-level settings key.
//
// Behavior:
//   - If settings.json does not exist, a fresh skeleton is created.
//   - If settings.json exists, it is backed up to settingsPath + ".bak"
//     before any mutation.
//   - The merged file is written atomically (temp file in the same
//     directory, then renamed) so a partial write never corrupts the real
//     settings file.
//   - Idempotent: running twice produces identical output (same matcher +
//     same git-courer command → skip; same matcher + changed git-courer
//     command → update in place, e.g. when the binary path changes).
func installClaudeHooks(settingsPath, binPath string) error {
	// Ensure the settings directory exists.
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("failed to create settings dir: %w", err)
	}

	// Read existing settings.json into a generic map to preserve every
	// top-level key (permissions, env, model, etc.) that we do not own.
	settings := make(map[string]interface{})
	fileExisted := false
	if data, err := os.ReadFile(settingsPath); err == nil {
		fileExisted = true
		// A parse failure means the file is not valid JSON. Rather than
		// clobbering user config, fall back to a fresh map but still back
		// up the original so the user can recover.
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			settings = make(map[string]interface{})
		}
	}

	// Backup existing settings.json before mutation (only if it exists).
	if fileExisted {
		if backupErr := backupClaudeSettings(settingsPath); backupErr != nil {
			return fmt.Errorf("failed to backup settings.json: %w", backupErr)
		}
	}

	// Build the git-courer hooks to install (exec form, no shell quoting).
	gitcourerHooks := map[string][]claudeHookEntry{
		"PreToolUse": {
			{
				Matcher: "Bash",
				Hooks: []claudeHookCmd{
					{Type: "command", Command: binPath + " hook-check", Args: []string{}},
				},
			},
		},
		"SessionStart": {
			{
				Matcher: "startup|resume",
				Hooks: []claudeHookCmd{
					{Type: "command", Command: binPath + " session-start-hook", Args: []string{}, Timeout: 10},
				},
			},
		},
		"SubagentStart": {
			{
				Matcher: "general-purpose|Explore|Plan",
				Hooks: []claudeHookCmd{
					{Type: "command", Command: binPath + " subagent-start-hook", Args: []string{}, Timeout: 10},
				},
			},
		},
		"UserPromptSubmit": {
			{
				Matcher: "",
				Hooks: []claudeHookCmd{
					{Type: "command", Command: binPath + " pre-invocation-hook", Args: []string{"UserPromptSubmit"}, Timeout: 10},
				},
			},
		},
	}

	// Decode the existing "hooks" object (if any) into the typed shape so
	// mergeClaudeHooks can match by matcher+command. Unknown sub-keys inside
	// "hooks" entries are dropped here, but every non-git-courer entry is
	// preserved at the entry level.
	existingHooks := make(map[string][]claudeHookEntry)
	if raw, ok := settings["hooks"]; ok {
		if rawMap, ok := raw.(map[string]interface{}); ok {
			for event, entries := range rawMap {
				entryList, ok := entries.([]interface{})
				if !ok {
					continue
				}
				for _, e := range entryList {
					em, ok := e.(map[string]interface{})
					if !ok {
						continue
					}
					var entry claudeHookEntry
					if m, ok := em["matcher"].(string); ok {
						entry.Matcher = m
					}
					if cmds, ok := em["hooks"].([]interface{}); ok {
						for _, c := range cmds {
							cm, ok := c.(map[string]interface{})
							if !ok {
								continue
							}
							var cmd claudeHookCmd
							if v, ok := cm["type"].(string); ok {
								cmd.Type = v
							}
							if v, ok := cm["command"].(string); ok {
								cmd.Command = v
							}
							if args, ok := cm["args"].([]interface{}); ok {
								for _, a := range args {
									if s, ok := a.(string); ok {
										cmd.Args = append(cmd.Args, s)
									}
								}
							}
							if v, ok := cm["timeout"].(float64); ok {
								cmd.Timeout = int(v)
							}
							entry.Hooks = append(entry.Hooks, cmd)
						}
					}
					existingHooks[event] = append(existingHooks[event], entry)
				}
			}
		}
	}

	merged := mergeClaudeHooks(existingHooks, gitcourerHooks)
	settings["hooks"] = merged

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings.json: %w", err)
	}

	// Write atomically: temp file in the same directory, then rename.
	return writeAtomic(settingsPath, data, 0644)
}

// removeClaudeHooks strips every git-courer hook entry from the Claude Code
// settings.json at settingsPath. A hook command is removed if its command
// string contains "git-courer". Empty entries (matcher groups with no
// remaining hooks) and empty events are dropped so the file stays clean.
//
// Behavior:
//   - If no git-courer hooks are found, the call is a no-op (the file is not
//     rewritten and no backup is touched).
//   - If settingsPath + ".bak" exists, it is restored over settingsPath and
//     removed (same convention as RemoveHook for Codex).
//   - Otherwise the cleaned JSON is written atomically.
func removeClaudeHooks(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		// No settings.json — nothing to remove.
		return nil
	}

	settings := make(map[string]interface{})
	if err := json.Unmarshal(data, &settings); err != nil {
		// Unparseable settings.json — leave it alone rather than risk
		// clobbering user config.
		return nil
	}

	rawHooks, ok := settings["hooks"]
	if !ok {
		return nil // no hooks at all — nothing to remove
	}
	hooksMap, ok := rawHooks.(map[string]interface{})
	if !ok {
		return nil
	}

	// First pass: check if any git-courer hook is present. If not, no-op.
	anyGitCourer := false
	for _, entries := range hooksMap {
		entryList, ok := entries.([]interface{})
		if !ok {
			continue
		}
		for _, e := range entryList {
			em, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			cmds, ok := em["hooks"].([]interface{})
			if !ok {
				continue
			}
			for _, c := range cmds {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if cmd, ok := cm["command"].(string); ok && strings.Contains(cmd, "git-courer") {
					anyGitCourer = true
					break
				}
			}
			if anyGitCourer {
				break
			}
		}
		if anyGitCourer {
			break
		}
	}
	if !anyGitCourer {
		return nil // no git-courer hooks — leave the file untouched
	}

	// If a backup exists, restore it and we are done.
	bakPath := settingsPath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		bakData, err := os.ReadFile(bakPath)
		if err != nil {
			return fmt.Errorf("failed to read settings backup: %w", err)
		}
		if err := writeAtomic(settingsPath, bakData, 0644); err != nil {
			return fmt.Errorf("failed to restore settings backup: %w", err)
		}
		_ = os.Remove(bakPath)
		return nil
	}

	// No backup — strip git-courer hooks in place and write cleaned JSON.
	cleaned := make(map[string]interface{}, len(hooksMap))
	for event, entries := range hooksMap {
		entryList, ok := entries.([]interface{})
		if !ok {
			// Keep unknown-shape events untouched.
			cleaned[event] = entries
			continue
		}
		var keptEntries []interface{}
		for _, e := range entryList {
			em, ok := e.(map[string]interface{})
			if !ok {
				keptEntries = append(keptEntries, e)
				continue
			}
			cmds, ok := em["hooks"].([]interface{})
			if !ok {
				keptEntries = append(keptEntries, em)
				continue
			}
			var keptCmds []interface{}
			for _, c := range cmds {
				cm, ok := c.(map[string]interface{})
				if !ok {
					keptCmds = append(keptCmds, c)
					continue
				}
				if cmd, ok := cm["command"].(string); ok && strings.Contains(cmd, "git-courer") {
					continue // drop git-courer hook
				}
				keptCmds = append(keptCmds, c)
			}
			if len(keptCmds) == 0 {
				continue // drop the whole entry — no hooks left
			}
			em["hooks"] = keptCmds
			keptEntries = append(keptEntries, em)
		}
		if len(keptEntries) == 0 {
			continue // drop the event — no entries left
		}
		cleaned[event] = keptEntries
	}
	if len(cleaned) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = cleaned
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cleaned settings.json: %w", err)
	}
	return writeAtomic(settingsPath, out, 0644)
}

// backupClaudeSettings copies settingsPath to settingsPath + ".bak" so
// removeClaudeHooks can restore it. This mirrors backupConfig for MCP configs
// and installHook's backup for Codex hooks.
func backupClaudeSettings(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath+".bak", data, 0644)
}

// writeAtomic writes data to path via a temp file in the same directory
// followed by a rename, so a crash mid-write never leaves a truncated file.
// The temp file is created with the same permissions the final file should
// have. On Windows the rename replaces the destination atomically (os.Rename
// handles the replace semantics on every supported platform).
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-settings-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Always clean up the temp file if we bail before rename.
	defer func() {
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	return nil
}

// installAntigravityPermissions merges the git-courer permission entries into
// the Antigravity settings.json at settingsPath:
//   - permissions.allow gains "mcp(git-courer/*)"
//   - permissions.ask gains "command(git *)" and "command(*)"
//
// Existing non-git-courer permission keys and every other top-level settings
// key are preserved. Behavior:
//   - If settings.json does not exist, a fresh file is created (no backup).
//   - If settings.json exists, it is backed up to settingsPath + ".gc.bak"
//     before the first mutation. The backup is NOT overwritten on re-run.
//   - If settings.json exists but is unparseable JSON, it is backed up and a
//     fresh file with the git-courer permissions is written.
//   - Idempotent: running twice produces byte-identical settings.json.
func installAntigravityPermissions(settingsPath, binPath string) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("failed to create settings dir: %w", err)
	}

	// Read existing settings.json into a generic map to preserve every
	// top-level key we do not own.
	settings := make(map[string]interface{})
	fileExisted := false
	parsedOK := false
	if data, err := os.ReadFile(settingsPath); err == nil {
		fileExisted = true
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			// Unparseable — back up the original bytes and start fresh so we
			// can still apply the permissions without clobbering user config.
			if backupErr := backupAntigravitySettings(settingsPath, data); backupErr != nil {
				return fmt.Errorf("failed to backup unparseable settings: %w", backupErr)
			}
			settings = make(map[string]interface{})
		} else {
			parsedOK = true
		}
	}

	// Backup existing valid settings before mutation (only if it parsed and
	// no backup already exists from a prior install — preserves idempotency).
	if fileExisted && parsedOK {
		bakPath := settingsPath + ".gc.bak"
		if _, err := os.Stat(bakPath); os.IsNotExist(err) {
			if backupErr := backupAntigravitySettings(settingsPath, nil); backupErr != nil {
				return fmt.Errorf("failed to backup settings: %w", backupErr)
			}
		}
	}

	// permissions.allow, permissions.ask and permissions.block — merge git-courer entries.
	perm, _ := settings["permissions"].(map[string]interface{})
	if perm == nil {
		perm = make(map[string]interface{})
		settings["permissions"] = perm
	}

	// allow: mcp(git-courer/*)
	allow, _ := perm["allow"].([]interface{})
	allow = mergeStringEntry(allow, "mcp(git-courer/*)")
	perm["allow"] = allow

	// block: command(git *)
	block, _ := perm["block"].([]interface{})
	block = mergeStringEntry(block, "command(git *)")
	perm["block"] = block

	// ask: command(*)
	ask, _ := perm["ask"].([]interface{})
	ask = mergeStringEntry(ask, "command(*)")
	// Clean up command(git *) from ask if it was present there
	ask = filterStringEntries(ask, []string{"command(git *)"})
	if len(ask) == 0 {
		delete(perm, "ask")
	} else {
		perm["ask"] = ask
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings.json: %w", err)
	}
	return writeAtomic(settingsPath, data, 0644)
}

// removeAntigravityPermissions strips the git-courer permission entries from
// the Antigravity settings.json at settingsPath. Behavior:
//   - If settingsPath + ".gc.bak" exists, it is restored over settingsPath and
//     the .bak file is removed.
//   - Otherwise the git-courer permission entries are removed in place,
//     preserving non-git-courer keys.
//   - Idempotent: running twice does not error.
func removeAntigravityPermissions(settingsPath string) error {
	bakPath := settingsPath + ".gc.bak"
	if _, err := os.Stat(bakPath); err == nil {
		bakData, err := os.ReadFile(bakPath)
		if err != nil {
			return fmt.Errorf("failed to read settings backup: %w", err)
		}
		if err := writeAtomic(settingsPath, bakData, 0644); err != nil {
			return fmt.Errorf("failed to restore settings backup: %w", err)
		}
		_ = os.Remove(bakPath)
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		// No file — nothing to remove.
		return nil
	}

	settings := make(map[string]interface{})
	if err := json.Unmarshal(data, &settings); err != nil {
		// Unparseable — leave it alone rather than risk clobbering user config.
		return nil
	}

	perm, ok := settings["permissions"].(map[string]interface{})
	if !ok {
		return nil // no permissions section — nothing to strip
	}

	changed := false
	if allow, ok := perm["allow"].([]interface{}); ok {
		filtered := filterStringEntries(allow, []string{"mcp(git-courer/*)"})
		if len(filtered) != len(allow) {
			changed = true
			if len(filtered) == 0 {
				delete(perm, "allow")
			} else {
				perm["allow"] = filtered
			}
		}
	}
	if ask, ok := perm["ask"].([]interface{}); ok {
		filtered := filterStringEntries(ask, []string{"command(git *)", "command(*)"})
		if len(filtered) != len(ask) {
			changed = true
			if len(filtered) == 0 {
				delete(perm, "ask")
			} else {
				perm["ask"] = filtered
			}
		}
	}
	if block, ok := perm["block"].([]interface{}); ok {
		filtered := filterStringEntries(block, []string{"command(git *)"})
		if len(filtered) != len(block) {
			changed = true
			if len(filtered) == 0 {
				delete(perm, "block")
			} else {
				perm["block"] = filtered
			}
		}
	}

	if !changed {
		return nil // no git-courer entries — leave the file untouched
	}

	// If the permissions map is now empty (all entries were git-courer-owned
	// and got dropped), drop the permissions key too.
	if len(perm) == 0 {
		delete(settings, "permissions")
	}

	// If the settings map is now empty (all keys were git-courer-owned and got
	// dropped), delete the file entirely rather than leaving an empty {}.
	if len(settings) == 0 {
		_ = os.Remove(settingsPath)
		return nil
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cleaned settings.json: %w", err)
	}
	// If the marshaled output is just an empty object, delete the file.
	if string(out) == "{}" {
		_ = os.Remove(settingsPath)
		return nil
	}
	return writeAtomic(settingsPath, out, 0644)
}

// backupAntigravitySettings copies settingsPath to settingsPath + ".gc.bak"
// so removeAntigravityPermissions can restore it. If extra is non-nil it is
// written instead of reading the file (used for the unparseable case).
func backupAntigravitySettings(settingsPath string, extra []byte) error {
	var data []byte
	var err error
	if extra != nil {
		data = extra
	} else {
		data, err = os.ReadFile(settingsPath)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(settingsPath+".gc.bak", data, 0644)
}

// mergeStringEntry appends s to slice if it is not already present, returning
// the possibly-extended slice. Idempotent: re-adding a present entry is a no-op.
func mergeStringEntry(slice []interface{}, s string) []interface{} {
	for _, v := range slice {
		if str, ok := v.(string); ok && str == s {
			return slice // already present
		}
	}
	return append(slice, s)
}

// filterStringEntries returns slice with every entry equal to any of removed
// excluded, preserving order and non-string entries.
func filterStringEntries(slice []interface{}, removed []string) []interface{} {
	out := make([]interface{}, 0, len(slice))
	for _, v := range slice {
		if s, ok := v.(string); ok {
			drop := false
			for _, r := range removed {
				if s == r {
					drop = true
					break
				}
			}
			if drop {
				continue
			}
		}
		out = append(out, v)
	}
	return out
}
