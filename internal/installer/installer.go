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
//   - HooksStatus: the hook installation status — "installed" or "not_installed".
type ClientDiagnostic struct {
	ClientName         string
	ConfigPath         string
	MCPConfigured      bool
	GitCourerMdPresent bool
	HooksStatus        string
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

	// Remove hooks and GIT_COURER.md for each detected client before
	// restoring config backups.
	for _, client := range DetectClients() {
		configPath := client.Paths[0]
		for _, path := range client.Paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}

		// Remove hooks.json if this client has hooks configured.
		if client.HooksConfig != nil {
			if err := RemoveHook(client.HooksConfig.HooksPath); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ Failed to remove hooks: %v\n", err)
			} else {
				fmt.Printf("  ✓ Removed hooks: %s\n", client.HooksConfig.HooksPath)
			}
		}

		// Remove GIT_COURER.md.
		rulesPath := filepath.Join(filepath.Dir(configPath), gitCourerMdFilename)
		if err := os.Remove(rulesPath); err == nil {
			fmt.Printf("  ✓ Removed: %s\n", rulesPath)
		}

		restoreBackup(configPath)
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
		if client.HooksConfig != nil {
			d.HooksStatus = hooksStatus(client.HooksConfig.HooksPath)
		} else {
			d.HooksStatus = "not_installed"
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
