// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// BinaryName is the name of the binary.
	BinaryName = "git-courer"
	// Owner is the GitHub owner.
	Owner = "Alejandro-M-P"
	// Repo is the GitHub repository.
	Repo = "git-courer"
)

const (
	// PostInstallEnv is the env var that triggers post-install setup.
	PostInstallEnv = "GIT_COURER_POSTINSTALL"
)

// RunPostInstall runs setup after go install.
// Called when GIT_COURER_POSTINSTALL=1 is set.
func RunPostInstall() error {
	// Get current directory (or default to ".")
	projectDir := "."
	if len(os.Args) > 1 {
		projectDir = os.Args[1]
	}

	// Run setup
	return RunSetup(projectDir)
}

// RunSetup runs project setup.
func RunSetup(projectDir string) error {
	fmt.Println("Setting up git-courer...")

	// Setup project
	if err := SetupProject(projectDir); err != nil {
		return fmt.Errorf("project setup failed: %w", err)
	}
	fmt.Println("  ✓ Project configured")

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

// RunRemove removes git-courer from a project.
func RunRemove(projectDir string) error {
	fmt.Println("Removing git-courer...")

	if err := RemoveProject(projectDir); err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}

	fmt.Println("  ✓ Project cleaned")
	fmt.Println("\n✓ git-courer removed from project!")
	return nil
}

// RunUninstall performs global uninstall.
func RunUninstall() error {
	fmt.Println("Uninstalling git-courer...")

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
