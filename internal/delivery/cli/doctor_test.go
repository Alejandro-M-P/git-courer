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
				HooksStatus:        "not_implemented",
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
	if !strings.Contains(output, "pending (SDDs 2-5)") {
		t.Errorf("output missing hooks status\noutput: %s", output)
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