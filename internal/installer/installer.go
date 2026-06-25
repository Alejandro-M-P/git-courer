// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// BinaryName is the name of the binary.
	BinaryName = "git-courer"
	// Owner is the GitHub owner.
	Owner = "blak0p"
	// Repo is the GitHub repository.
	Repo = "git-courer"
)

const (
	// PostInstallEnv is the env var that triggers post-install setup.
	PostInstallEnv = "GIT_COURER_POSTINSTALL"
)

// ClientDiagnostic is the per-client diagnostic returned by RunDoctor.
//
// Fields:
//   - ClientName: the MCP client name (e.g. "claude-code").
//   - ConfigPath: the resolved config file path.
//   - MCPConfigured: true if the config file exists and contains git-courer.
//   - GitCourerMdPresent: true if GIT_COURER.md exists in the config directory.
//   - HooksStatus: the Codex hook installation status — "installed" or
//     "not_installed". Empty for clients without a HooksPath.
//   - ClaudeHooksStatus: the Claude Code hook installation status —
//     "installed", "not_installed", or "partial". Empty for clients without a
//     SettingsPath.
type ClientDiagnostic struct {
	ClientName         string
	ConfigPath         string
	MCPConfigured      bool
	GitCourerMdPresent bool
	HooksStatus        string
	ClaudeHooksStatus  string
}

// RunPostInstall runs setup after go install.
// Called when GIT_COURER_POSTINSTALL=1 is set.
func RunPostInstall() error {
	fmt.Println("Setting up git-courer (global config mode)...")

	// Detect binary path
	binPath, err := FindBinaryPath()
	if err != nil {
		binPath = BinaryName
	}

	// Setup MCP
	clients := DetectClients()
	if len(clients) == 0 {
		fmt.Println("  No MCP clients detected")
	} else {
		configured, err := ConfigureAllMCP(binPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  MCP setup: %v\n", err)
		} else {
			fmt.Printf("  %d MCP client(s) configured\n", configured)
		}
	}

	fmt.Println("\n✓ git-courer setup complete!")
	return nil
}

// RunUninstall performs global uninstall.
func RunUninstall() error {
	fmt.Println("Uninstalling git-courer...")

	// Remove hooks and GIT_COURER.md for each known client before
	// restoring config backups. Use MCPClients() (all known clients)
	// instead of DetectClients() (only currently-detected) so hooks
	// are cleaned up even if the client binary is no longer on PATH.
	for _, client := range MCPClients() {
		configPath := client.Paths[0]
		for _, path := range client.Paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}

		// Remove hooks if this client has hooks configured.
		// Codex uses a separate hooks.json (HooksPath); Claude Code stores
		// hooks inline in settings.json (SettingsPath). Both are cleaned up
		// here even if the client binary is no longer on PATH.
		if client.HooksConfig != nil {
			if client.HooksConfig.HooksPath != "" {
				if err := RemoveHook(client.HooksConfig.HooksPath); err != nil {
					fmt.Fprintf(os.Stderr, "  ⚠ Failed to remove hooks: %v\n", err)
				} else {
					fmt.Printf("  ✓ Removed hooks: %s\n", client.HooksConfig.HooksPath)
				}
			}
			if client.HooksConfig.SettingsPath != "" {
				if err := removeClaudeHooks(client.HooksConfig.SettingsPath); err != nil {
					fmt.Fprintf(os.Stderr, "  ⚠ Failed to remove Claude hooks: %v\n", err)
				} else {
					fmt.Printf("  ✓ Removed Claude hooks: %s\n", client.HooksConfig.SettingsPath)
				}
			}
		}

		// Remove GIT_COURER.md.
		rulesPath := filepath.Join(filepath.Dir(configPath), gitCourerMdFilename)
		if err := os.Remove(rulesPath); err == nil {
			fmt.Printf("  ✓ Removed: %s\n", rulesPath)
		}

		restoreBackup(configPath)

		// Strip git-courer policy entries (permission.bash "git *" and
		// instructions GIT_COURER.md path) from opencode.json. Called AFTER
		// restoreBackup: if a backup existed it already reverted everything
		// (policy included), and this becomes a no-op; if no backup existed
		// this strips the entries in place. Only applies to the opencode
		// client.
		if client.Name == "opencode" {
			if err := removeOpenCodePolicy(configPath); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ Failed to strip OpenCode policy: %v\n", err)
			}
		}
	}

	// Remove binary
	binPath, err := FindBinaryPath()
	if err != nil {
		fmt.Println("  ⚠ Binary not found")
	} else {
		if err := os.Remove(binPath); err != nil {
			return fmt.Errorf("failed to remove binary: %w", err)
		}
		fmt.Printf("  ✓ Removed: %s\n", binPath)
	}

	// Remove global config
	home, _ := os.UserHomeDir()
	globalConfig := filepath.Join(home, ".config/git-courer/config.yaml")
	if err := os.Remove(globalConfig); err == nil {
		fmt.Printf("  ✓ Removed config: %s\n", globalConfig)
	}

	// Remove MCP configs - TODO: ask user or remove all
	fmt.Println("\n✓ git-courer uninstalled!")
	return nil
}

// restoreBackup restores configPath + ".bak" over configPath if a backup
// exists, then removes the backup. Silently no-ops if no backup exists.
func restoreBackup(configPath string) {
	bakPath := configPath + ".bak"
	data, err := os.ReadFile(bakPath)
	if err != nil {
		return // no backup — nothing to restore
	}
	if err := os.WriteFile(configPath, data, 0644); err == nil {
		fmt.Printf("  ✓ Restored backup: %s\n", configPath)
		_ = os.Remove(bakPath)
	}
}

// RunDoctor inspects each detected MCP client and reports diagnostic state:
// whether the MCP config exists and contains git-courer, whether
// GIT_COURER.md exists, and the hook installation status (stub until SDDs 2-5).
//
// It returns one ClientDiagnostic per detected client.
func RunDoctor() []ClientDiagnostic {
	clients := DetectClients()
	diagnostics := make([]ClientDiagnostic, 0, len(clients))

	for _, client := range clients {
		configPath := client.Paths[0]
		for _, path := range client.Paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}

		d := ClientDiagnostic{
			ClientName: client.Name,
			ConfigPath: configPath,
		}

		// Check hooks status if this client has hooks configured.
		// Codex (HooksPath) and Claude Code (SettingsPath) are mutually
		// exclusive per client — only the relevant status is populated.
		// Clients without any HooksConfig report "not_installed" for both so
		// the doctor report always shows a concrete hooks state.
		if client.HooksConfig != nil {
			if client.HooksConfig.HooksPath != "" {
				d.HooksStatus = hooksStatus(client.HooksConfig.HooksPath)
			} else {
				d.HooksStatus = "not_installed"
			}
			if client.HooksConfig.SettingsPath != "" {
				d.ClaudeHooksStatus = claudeHooksStatus(client.HooksConfig.SettingsPath)
			} else {
				d.ClaudeHooksStatus = "not_installed"
			}
		} else {
			d.HooksStatus = "not_installed"
			d.ClaudeHooksStatus = "not_installed"
		}

		if data, err := os.ReadFile(configPath); err == nil {
			d.MCPConfigured = strings.Contains(string(data), "git-courer")
		}

		rulesPath := filepath.Join(filepath.Dir(configPath), gitCourerMdFilename)
		if _, err := os.Stat(rulesPath); err == nil {
			d.GitCourerMdPresent = true
		}

		diagnostics = append(diagnostics, d)
	}

	return diagnostics
}

// RunUpdate checks for and applies updates.
func RunUpdate(force bool) error {
	hasUpdate, newVersion, err := CheckForUpdates()
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}

	if !hasUpdate && !force {
		fmt.Println("Already up to date!")
		return nil
	}

	if force {
		fmt.Printf("Updating to latest...\n")
	} else {
		fmt.Printf("Update available: %s\n", newVersion)
	}

	if err := DownloadUpdate(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println("✓ Updated to latest version!")
	return nil
}
