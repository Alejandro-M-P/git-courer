package screens

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestMCPSetupScreen_Dedup(t *testing.T) {
	// 1. Mock installer clients
	originalGetClients := getMCPClients
	originalConfigure := configureMCP
	originalFindBin := findBinaryPath
	defer func() {
		getMCPClients = originalGetClients
		configureMCP = originalConfigure
		findBinaryPath = originalFindBin
	}()

	mockClients := []*installer.MCPClient{
		{
			Name: "antigravity",
			Detect: func() bool { return true }, // detected CLI
		},
		{
			Name: "antigravity",
			Detect: func() bool { return false }, // undetected IDE
		},
		{
			Name: "pi",
			Detect: func() bool { return false }, // undetected pi
		},
		{
			Name: "cursor",
			Detect: func() bool { return true }, // detected cursor
		},
	}

	getMCPClients = func() []*installer.MCPClient {
		return mockClients
	}

	// 2. Instantiate screen
	screen := NewMCPSetupScreen(80)

	// 3. Verify deduplication in items
	items := screen.Items()
	// Expected: unique names: "antigravity", "pi", "cursor"
	if len(items) != 3 {
		t.Fatalf("expected 3 items after deduplication, got %d: %+v", len(items), items)
	}

	// Check mapping and aggregation
	// Item 0: "antigravity" (detected=true because one of them is true)
	if items[0].Name != "antigravity" {
		t.Errorf("item 0 name: got %q, want %q", items[0].Name, "antigravity")
	}
	if !items[0].Detected {
		t.Error("antigravity should be detected (aggregated)")
	}
	if !items[0].Selected {
		t.Error("antigravity should be selected (aggregated default)")
	}

	// Item 1: "pi" (detected=false)
	if items[1].Name != "pi" {
		t.Errorf("item 1 name: got %q, want %q", items[1].Name, "pi")
	}
	if items[1].Detected {
		t.Error("pi should not be detected")
	}

	// Item 2: "cursor" (detected=true)
	if items[2].Name != "cursor" {
		t.Errorf("item 2 name: got %q, want %q", items[2].Name, "cursor")
	}
	if !items[2].Detected {
		t.Error("cursor should be detected")
	}
}

func TestMCPSetupScreen_ConfigureClients(t *testing.T) {
	originalGetClients := getMCPClients
	originalConfigure := configureMCP
	originalFindBin := findBinaryPath
	defer func() {
		getMCPClients = originalGetClients
		configureMCP = originalConfigure
		findBinaryPath = originalFindBin
	}()

	var configured []*installer.MCPClient
	configureMCP = func(client *installer.MCPClient, binPath string) error {
		configured = append(configured, client)
		return nil
	}
	findBinaryPath = func() (string, error) {
		return "/mock/git-courer", nil
	}

	mockClients := []*installer.MCPClient{
		{
			Name: "antigravity",
			Detect: func() bool { return true }, // CLI, detected
		},
		{
			Name: "antigravity",
			Detect: func() bool { return false }, // IDE, not detected
		},
		{
			Name: "pi",
			Detect: func() bool { return false }, // pi, not detected
		},
		{
			Name: "cursor",
			Detect: func() bool { return true }, // cursor, detected
		},
	}
	getMCPClients = func() []*installer.MCPClient {
		return mockClients
	}

	// selection step: enter with default choices (detected ones selected)
	screen := NewMCPSetupScreen(80)

	// First enter key starts configuring (step 0 -> step 1)
	updated, _ := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s := updated.(*MCPSetupScreen)

	// Second enter key goes to step 2 (Done)
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s = updated.(*MCPSetupScreen)

	if !s.Done() {
		t.Error("expected screen to be done after configuring and second enter")
	}

	// We expect:
	// - only the detected antigravity entry to be configured (since it was selected and detected)
	// - cursor to be configured (since it was selected and detected)
	// - undetected antigravity IDE NOT to be configured
	// - pi NOT to be configured (not selected, not detected)
	if len(configured) != 2 {
		t.Fatalf("expected 2 client configurations, got %d", len(configured))
	}

	// Verify the clients configured are CLI and cursor
	hasCli := false
	hasIde := false
	hasCursor := false
	for _, c := range configured {
		if c.Name == "antigravity" {
			if c == mockClients[0] {
				hasCli = true
			} else if c == mockClients[1] {
				hasIde = true
			}
		} else if c.Name == "cursor" {
			hasCursor = true
		}
	}

	if !hasCli {
		t.Error("expected antigravity CLI to be configured")
	}
	if hasIde {
		t.Error("expected antigravity IDE not to be configured")
	}
	if !hasCursor {
		t.Error("expected cursor to be configured")
	}
}
