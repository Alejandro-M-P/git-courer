// Package cli_test verifies the restore CLI adapter.
package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestRestoreRun_PrintsCompletion verifies RestoreCommand.Run invokes the
// restore function and prints a completion message on stdout.
func TestRestoreRun_PrintsCompletion(t *testing.T) {
	cmd := RestoreCommand{}

	// Stub restoreFn so we don't touch real MCP client configs.
	originalRestoreFn := restoreFn
	defer func() { restoreFn = originalRestoreFn }()
	called := false
	restoreFn = func() error {
		called = true
		return nil
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = cmd.Run()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Error("restoreFn was not called by RestoreCommand.Run")
	}

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Fatalf("read pipe: %v", copyErr)
	}
	output := buf.String()
	if !strings.Contains(output, "Restore complete") {
		t.Errorf("output missing 'Restore complete' marker\noutput: %s", output)
	}
}

// TestRestoreRun_PropagatesError verifies RestoreCommand.Run surfaces the
// error returned by restoreFn to the caller.
func TestRestoreRun_PropagatesError(t *testing.T) {
	cmd := RestoreCommand{}

	originalRestoreFn := restoreFn
	defer func() { restoreFn = originalRestoreFn }()
	restoreFn = func() error {
		return errSentinel
	}

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Run()
	w.Close()
	os.Stdout = oldStdout

	if err != errSentinel {
		t.Errorf("Run error: got %v, want %v", err, errSentinel)
	}
}