package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ============================================================================
// Platform Detection
// ============================================================================

func TestDetect_ReturnsCurrentPlatform(t *testing.T) {
	p := Detect()

	if p == nil {
		t.Fatal("Detect() returned nil")
	}
	if string(p.OS) != runtime.GOOS {
		t.Errorf("OS: got %q, want %q", p.OS, runtime.GOOS)
	}
	if p.Arch != runtime.GOARCH {
		t.Errorf("Arch: got %q, want %q", p.Arch, runtime.GOARCH)
	}
}

func TestPlatform_BinaryName(t *testing.T) {
	tests := []struct {
		platform *Platform
		want     string
	}{
		{&Platform{OS: "linux", Arch: "amd64"}, "git-courer-linux-amd64"},
		{&Platform{OS: "darwin", Arch: "arm64"}, "git-courer-darwin-arm64"},
		{&Platform{OS: "windows", Arch: "amd64"}, "git-courer-windows-amd64.exe"},
	}
	for _, tc := range tests {
		got := tc.platform.BinaryName()
		if got != tc.want {
			t.Errorf("BinaryName(%s/%s) = %q, want %q", tc.platform.OS, tc.platform.Arch, got, tc.want)
		}
	}
}

func TestPlatform_GitHubAsset(t *testing.T) {
	tests := []struct {
		platform *Platform
		want     string
	}{
		{&Platform{OS: "linux", Arch: "amd64"}, "git-courer_linux_amd64"},
		{&Platform{OS: "darwin", Arch: "arm64"}, "git-courer_darwin_arm64"},
		{&Platform{OS: "windows", Arch: "amd64"}, "git-courer_windows_amd64"},
	}
	for _, tc := range tests {
		got := tc.platform.GitHubAsset()
		if got != tc.want {
			t.Errorf("GitHubAsset(%s/%s) = %q, want %q", tc.platform.OS, tc.platform.Arch, got, tc.want)
		}
	}
}

func TestPlatform_String(t *testing.T) {
	p := &Platform{OS: "linux", Arch: "amd64"}
	got := p.String()
	if got != "linux/amd64" {
		t.Errorf("String() = %q, want %q", got, "linux/amd64")
	}
}

// ============================================================================
// MCP Client Config Generation
// ============================================================================

func TestMCPClients_AllHaveRequiredFields(t *testing.T) {
	clients := MCPClients()
	if len(clients) == 0 {
		t.Fatal("MCPClients() returned empty list")
	}
	for _, c := range clients {
		if c.Name == "" {
			t.Errorf("client has empty Name: %+v", c)
		}
		if c.Filename == "" {
			t.Errorf("client %q has empty Filename", c.Name)
		}
		if c.RootKey == "" && !c.IsArray {
			t.Errorf("client %q has empty RootKey and IsArray=false", c.Name)
		}
		if c.ConfigFn == nil {
			t.Errorf("client %q has nil ConfigFn", c.Name)
		}
		if c.Detect == nil {
			t.Errorf("client %q has nil Detect", c.Name)
		}
		// claude-desktop and some clients only support specific OSes (darwin/windows),
		// so paths may be nil on Linux — that's expected behavior.
		if len(c.Paths) == 0 && runtime.GOOS == "linux" && c.Name != "claude-desktop" {
			t.Errorf("client %q has no config paths on linux", c.Name)
		}
		if len(c.Paths) == 0 && runtime.GOOS != "linux" {
			t.Errorf("client %q has no config paths on %s", c.Name, runtime.GOOS)
		}
	}
}

func TestMCPClients_ConfigFn_ClaudeCode(t *testing.T) {
	binPath := "/usr/local/bin/git-courer"
	client := findClient(t, "claude-code")

	entry := client.ConfigFn(binPath)

	if entry["command"] != binPath {
		t.Errorf("command: got %v, want %q", entry["command"], binPath)
	}
	args, ok := entry["args"].([]string)
	if !ok {
		t.Fatalf("args is not []string: %T", entry["args"])
	}
	if len(args) != 1 || args[0] != "mcp" {
		t.Errorf("args: got %v, want [mcp]", args)
	}
}

func TestMCPClients_ConfigFn_OpenCode(t *testing.T) {
	client := findClient(t, "opencode")
	entry := client.ConfigFn("/usr/local/bin/git-courer")

	if entry["type"] != "local" {
		t.Errorf("type: got %v, want 'local'", entry["type"])
	}
	if entry["enabled"] != true {
		t.Errorf("enabled: got %v, want true", entry["enabled"])
	}
}

func TestMCPClients_ConfigFn_Continue_ArrayFormat(t *testing.T) {
	client := findClient(t, "continue")
	if !client.IsArray {
		t.Error("continue client should have IsArray=true")
	}
	entry := client.ConfigFn("/usr/local/bin/git-courer")
	if entry["name"] != "git-courer" {
		t.Errorf("name: got %v, want 'git-courer'", entry["name"])
	}
}

// ============================================================================
// configureObjectFormat — standard JSON merge
// ============================================================================

func TestConfigureObjectFormat_WritesCorrectStructure(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureObjectFormat(configPath, "mcpServers", entry); err != nil {
		t.Fatalf("configureObjectFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	mcpServers, ok := result["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers is not a map: %T", result["mcpServers"])
	}
	if _, exists := mcpServers["git-courer"]; !exists {
		t.Error("git-courer entry not found in mcpServers")
	}
}

func TestConfigureObjectFormat_MergesIntoExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	// Write existing config with another tool already configured.
	existing := `{"mcpServers":{"other-tool":{"command":"other","args":["serve"]}}}`
	os.WriteFile(configPath, []byte(existing), 0644)

	entry := map[string]interface{}{"command": "/usr/local/bin/git-courer", "args": []string{"mcp"}}
	if err := configureObjectFormat(configPath, "mcpServers", entry); err != nil {
		t.Fatalf("configureObjectFormat: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	mcpServers := result["mcpServers"].(map[string]interface{})
	if _, exists := mcpServers["other-tool"]; !exists {
		t.Error("existing other-tool entry was removed — merge broke existing config")
	}
	if _, exists := mcpServers["git-courer"]; !exists {
		t.Error("git-courer entry not added")
	}
}

func TestConfigureObjectFormat_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")
	entry := map[string]interface{}{"command": "/usr/local/bin/git-courer", "args": []string{"mcp"}}

	// Write twice.
	configureObjectFormat(configPath, "mcpServers", entry)
	configureObjectFormat(configPath, "mcpServers", entry)

	data, _ := os.ReadFile(configPath)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	mcpServers := result["mcpServers"].(map[string]interface{})
	count := 0
	for k := range mcpServers {
		if k == "git-courer" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 git-courer entry, got %d", count)
	}
}

// ============================================================================
// configureArrayFormat — Continue format
// ============================================================================

func TestConfigureArrayFormat_WritesEntry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	entry := map[string]interface{}{
		"name":    "git-courer",
		"command": "/usr/local/bin/git-courer",
	}

	if err := configureArrayFormat(configPath, entry); err != nil {
		t.Fatalf("configureArrayFormat: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var result struct {
		MCPServers []map[string]interface{} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(result.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(result.MCPServers))
	}
	if result.MCPServers[0]["name"] != "git-courer" {
		t.Errorf("name: got %v, want git-courer", result.MCPServers[0]["name"])
	}
}

func TestConfigureArrayFormat_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	entry := map[string]interface{}{"name": "git-courer", "command": "/usr/local/bin/git-courer"}

	configureArrayFormat(configPath, entry)
	configureArrayFormat(configPath, entry)

	data, _ := os.ReadFile(configPath)
	var result struct {
		MCPServers []map[string]interface{} `json:"mcpServers"`
	}
	json.Unmarshal(data, &result)

	count := 0
	for _, s := range result.MCPServers {
		if s["name"] == "git-courer" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 git-courer entry, got %d", count)
	}
}

// ============================================================================
// containsGitCourer
// ============================================================================

func TestContainsGitCourer(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`{"mcpServers":{"git-courer":{"command":"/usr/local/bin/git-courer"}}}`, true},
		{`{"mcpServers":{"other-tool":{"command":"other"}}}`, false},
		{``, false},
		{`git-courer`, true},
	}
	for _, tc := range tests {
		got := containsGitCourer(tc.input)
		if got != tc.want {
			t.Errorf("containsGitCourer(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// ============================================================================
// ConfigureMCP — full flow with temp config dir
// ============================================================================

func TestConfigureMCP_WritesConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	client := &MCPClient{
		Name:     "claude-code",
		Filename: "settings.json",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if !containsGitCourer(string(data)) {
		t.Errorf("config file does not contain git-courer entry:\n%s", data)
	}
}

func TestConfigureMCP_SkipsIfAlreadyConfigured(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	// Pre-populate with git-courer already configured.
	initial := `{"mcpServers":{"git-courer":{"command":"/usr/local/bin/git-courer","args":["mcp"]}}}`
	os.WriteFile(configPath, []byte(initial), 0644)

	client := &MCPClient{
		Name:    "claude-code",
		Paths:   []string{configPath},
		RootKey: "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath}
		},
		Detect: func() bool { return true },
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	// File content must be unchanged.
	data, _ := os.ReadFile(configPath)
	if string(data) != initial {
		t.Errorf("file was modified when it should have been left unchanged:\ngot:  %s\nwant: %s", data, initial)
	}
}

// TestFindBinaryPath_ReturnsErrorWhenNotInstalled verifica que FindBinaryPath
// devuelve un error descriptivo cuando git-courer no está instalado.
func TestFindBinaryPath_ReturnsErrorWhenNotInstalled(t *testing.T) {
	// In CI / test environments git-courer is not installed in the standard paths.
	// FindBinaryPath should return a descriptive error, not panic.
	_, err := FindBinaryPath()
	// It's OK if the binary IS found (developer machine with it installed).
	// What must never happen: a panic or an empty error message.
	if err != nil && err.Error() == "" {
		t.Error("FindBinaryPath() returned non-nil error with empty message")
	}
}

// ============================================================================
// Helpers
// ============================================================================

func findClient(t *testing.T, name string) *MCPClient {
	t.Helper()
	for _, c := range MCPClients() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("client %q not found in MCPClients()", name)
	return nil
}
