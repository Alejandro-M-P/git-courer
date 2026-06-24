// Package cli_test verifies the doctor CLI adapter.
package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/installer"
)

// TestDoctorRun_PrintsDiagnostics verifies DoctorCommand.Run prints a
// human-readable report with client name, config path, and status fields.
func TestDoctorRun_PrintsDiagnostics(t *testing.T) {
	cmd := DoctorCommand{}

	// Mock the doctorFn so we don't depend on real client detection.
	originalDoctorFn := doctorFn
	defer func() { doctorFn = originalDoctorFn }()
	doctorFn = func() []installer.ClientDiagnostic {
		return []installer.ClientDiagnostic{
			{
				ClientName:         "test-client",
				ConfigPath:         "/tmp/config.json",
				MCPConfigured:      true,
				GitCourerMdPresent: true,
				// Use a real HooksStatus value that the installer now reports.
				HooksStatus: "installed",
			},
		}
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

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Fatalf("read pipe: %v", copyErr)
	}

	output := buf.String()
	if !strings.Contains(output, "test-client") {
		t.Errorf("output missing client name %q\noutput: %s", "test-client", output)
	}
	if !strings.Contains(output, "/tmp/config.json") {
		t.Errorf("output missing config path %q\noutput: %s", "/tmp/config.json", output)
	}
	if !strings.Contains(output, "yes") {
		t.Errorf("output missing 'yes' status markers\noutput: %s", output)
	}
}

// TestHooksLabel_Installed verifies hooksLabel maps the "installed" status
// reported by the installer to the human-readable "yes" label.
func TestHooksLabel_Installed(t *testing.T) {
	if got := hooksLabel("installed"); got != "yes" {
		t.Errorf("hooksLabel(%q): got %q, want %q", "installed", got, "yes")
	}
}

// TestHooksLabel_NotInstalled verifies hooksLabel maps the "not_installed"
// status to the human-readable "no" label.
func TestHooksLabel_NotInstalled(t *testing.T) {
	if got := hooksLabel("not_installed"); got != "no" {
		t.Errorf("hooksLabel(%q): got %q, want %q", "not_installed", got, "no")
	}
}

// TestHooksLabel_UnknownStatusPassthrough verifies hooksLabel returns an
// unrecognized status string as-is (defensive passthrough).
func TestHooksLabel_UnknownStatusPassthrough(t *testing.T) {
	if got := hooksLabel("something-else"); got != "something-else" {
		t.Errorf("hooksLabel(%q): got %q, want passthrough %q", "something-else", got, "something-else")
	}
}

// TestDoctorRun_NoClients verifies DoctorCommand.Run prints a friendly message
// when no clients are detected (not an error).
func TestDoctorRun_NoClients(t *testing.T) {
	cmd := DoctorCommand{}

	originalDoctorFn := doctorFn
	defer func() { doctorFn = originalDoctorFn }()
	doctorFn = func() []installer.ClientDiagnostic {
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

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Fatalf("read pipe: %v", copyErr)
	}

	output := strings.TrimSpace(buf.String())
	if !strings.Contains(output, "No MCP clients detected") {
		t.Errorf("expected friendly message, got: %q", output)
	}
}
