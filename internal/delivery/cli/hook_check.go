// Package cli provides CLI delivery adapters for git-courer subcommands.
//
// This file implements the hook-check subcommand: a thin adapter that
// classifies a shell command via the gitcmd classifier and emits the Result
// as JSON on stdout so the calling agent can decide which tool to use.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blak0p/git-courer/internal/classifier/gitcmd"
)

// HookCheckCommand implements the hook-check CLI subcommand.
//
// It receives the shell command the agent is about to run, classifies it via
// gitcmd.Classify, and prints the classification Result as JSON to stdout.
// This is a thin adapter — all classification logic lives in the gitcmd
// package so it can be unit-tested independently.
type HookCheckCommand struct{}

// Run classifies args[0] and emits the Result as JSON on stdout.
// It returns an error if no command argument was provided.
func (c HookCheckCommand) Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: git-courer hook-check <command>")
	}
	command := args[0]
	if command == "" {
		return fmt.Errorf("usage: git-courer hook-check <command>")
	}

	result := gitcmd.Classify(command)

	// Emit as JSON — consistent with existing delivery patterns.
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return fmt.Errorf("hook-check: failed to encode result: %w", err)
	}
	return nil
}