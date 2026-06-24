// Package cli provides CLI delivery adapters for git-courer subcommands.
//
// This file implements the doctor subcommand: a thin adapter that runs the
// installer's RunDoctor diagnostics and prints a human-readable report so the
// user can see the health of every detected MCP client at a glance.
package cli

import (
	"fmt"

	"github.com/blak0p/git-courer/internal/installer"
)

// doctorFn is the function DoctorCommand.Run calls to obtain diagnostics.
// It is a package-level variable so tests can stub it without depending on
// real client detection.
var doctorFn = installer.RunDoctor

// DoctorCommand implements the doctor CLI subcommand.
//
// It runs the installer diagnostics and prints a human-readable report to
// stdout. All diagnostic logic lives in the installer package.
type DoctorCommand struct{}

// Run prints a human-readable doctor report on stdout.
func (c DoctorCommand) Run() error {
	diagnostics := doctorFn()

	if len(diagnostics) == 0 {
		fmt.Println("No MCP clients detected.")
		return nil
	}

	for _, d := range diagnostics {
		fmt.Printf("=== %s ===\n", d.ClientName)
		fmt.Printf("  Config: %s\n", d.ConfigPath)
		fmt.Printf("  MCP configured:       %s\n", statusLabel(d.MCPConfigured))
		fmt.Printf("  GIT_COURER.md present: %s\n", statusLabel(d.GitCourerMdPresent))
		fmt.Printf("  Hooks installed:       %s\n", hooksLabel(d.HooksStatus))
		fmt.Println()
	}
	return nil
}

func statusLabel(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}

func hooksLabel(status string) string {
	switch status {
	case "installed":
		return "yes"
	case "not_installed":
		return "no"
	default:
		return status
	}
}