// Package cli provides CLI delivery adapters for git-courer subcommands.
//
// This file implements the restore subcommand: a thin adapter that runs the
// installer's RunRestore to restore MCP client config backups, remove hooks,
// and remove GIT_COURER.md files. It follows the same pattern as DoctorCommand
// — all domain logic lives in the installer package; this adapter only wires
// the CLI boundary and prints the result.
package cli

import (
	"errors"
	"fmt"

	"github.com/blak0p/git-courer/internal/installer"
)

// restoreFn is the function RestoreCommand.Run calls to perform the restore.
// It is a package-level variable so tests can stub it without depending on
// real client detection or filesystem state.
var restoreFn = installer.RunRestore

// errSentinel is a stable error used by tests to verify RestoreCommand.Run
// propagates errors from restoreFn.
var errSentinel = errors.New("restore failed: sentinel")

// RestoreCommand implements the restore CLI subcommand.
//
// It restores .bak config backups for all detected MCP clients and removes
// the git-courer hook and GIT_COURER.md from each client. Unlike uninstall, it
// does NOT remove the binary or the global config — it is a config-only
// restore. All restore logic lives in the installer package.
type RestoreCommand struct{}

// Run executes the restore and prints a completion message on stdout. It
// returns the error from the installer if the restore fails.
func (c RestoreCommand) Run() error {
	if err := restoreFn(); err != nil {
		return err
	}
	fmt.Println("✓ Restore complete!")
	return nil
}