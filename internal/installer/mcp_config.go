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
	"strings"

	"github.com/BurntSushi/toml"
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
	HooksConfig       *HooksConfig // nil = no hooks for this client
	ConfigFn          func(binPath string) map[string]interface{}
	PostInstallNotice func(binPath string) string // optional post-install warning check
	Paths             []string                    // possible config file paths for this platform
	Detect            func() bool
}

// HooksConfig specifies hook installation for an MCP client.
//
// A client may use one of two hook storage strategies:
//   - HooksPath: a separate hooks file (Codex stores hooks in
//     "~/.codex/hooks.json" — managed via installHook/RemoveHook).
//   - SettingsPath: inline hooks inside a settings file (Claude Code stores
//     hooks inline in "~/.claude/settings.json" — managed via
//     installClaudeHooks/removeClaudeHooks).
//
// At most one path is set per client. Empty string means the strategy is not
// used for this client.
type HooksConfig struct {
	HooksPath    string // e.g. "~/.codex/hooks.json" (Codex — separate file)
	SettingsPath string // e.g. "~/.claude/settings.json" (Claude Code — inline hooks)
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
			// Also ensure hooks are installed (idempotent).
			if client.HooksConfig != nil {
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

	// Write GIT_COURER.md golden rules alongside the config (idempotent).
	ensureGitCourerMd(configPath)

	// Install hooks for clients that have HooksConfig.
	if client.HooksConfig != nil {
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
//   - permission.bash["git *"] = "ask" (preserving any existing keys; Go's
//     alphabetical map sort on json.MarshalIndent naturally places "git *"
//     after "*" for last-match-wins).
//   - instructions array includes the path to GIT_COURER.md in the same
//     directory as configPath (deduplicated; if instructions is a string it
//     is converted to an array preserving the original entry).
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

	rulesPath := filepath.Join(filepath.Dir(configPath), gitCourerMdFilename)

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

	// permission.bash["git *"] = "ask"
	perm, _ := config["permission"].(map[string]interface{})
	if perm == nil {
		perm = make(map[string]interface{})
		config["permission"] = perm
	}
	bash, _ := perm["bash"].(map[string]interface{})
	if bash == nil {
		bash = make(map[string]interface{})
		perm["bash"] = bash
	}
	bash["git *"] = "ask"

	// instructions array includes GIT_COURER.md path (dedup; convert string → array).
	switch raw := config["instructions"].(type) {
	case string:
		// Convert to array preserving the original string.
		arr := []interface{}{raw}
		if raw != rulesPath {
			arr = append(arr, rulesPath)
		}
		config["instructions"] = arr
	case []interface{}:
		// Dedupe: if rulesPath already present, leave as-is; else append.
		already := false
		for _, v := range raw {
			if s, ok := v.(string); ok && s == rulesPath {
				already = true
				break
			}
		}
		if !already {
			raw = append(raw, rulesPath)
			config["instructions"] = raw
		}
	default:
		// nil, missing, or malformed — create fresh array with the path.
		config["instructions"] = []interface{}{rulesPath}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal opencode.json: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

// removeOpenCodePolicy strips the git-courer policy entries from opencode.json:
//   - permission.bash["git *"] is removed (other keys preserved).
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

	rulesPath := filepath.Join(filepath.Dir(configPath), gitCourerMdFilename)

	// Detect whether any git-courer policy entry exists. If not, no-op.
	hasPolicy := false

	// Check permission.bash["git *"].
	if perm, ok := config["permission"].(map[string]interface{}); ok {
		if bash, ok := perm["bash"].(map[string]interface{}); ok {
			if _, exists := bash["git *"]; exists {
				hasPolicy = true
				delete(bash, "git *")
				// Keep the bash map even if empty? The spec says preserve other
				// entries. If empty, we still keep it (no harm; preserves shape).
			}
		}
	}

	// Check instructions for GIT_COURER.md path.
	if raw, ok := config["instructions"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			kept := arr[:0]
			removedAny := false
			for _, v := range arr {
				if s, ok := v.(string); ok && s == rulesPath {
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
