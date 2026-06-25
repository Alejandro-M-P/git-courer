package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
		// All remaining clients support Linux, so paths must be non-empty on every OS.
		if len(c.Paths) == 0 {
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

func TestConfigureMCP_CallsPostInstallNotice(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	calledWith := ""
	mockNotice := func(binPath string) string {
		calledWith = binPath
		return "warning: test post install notice"
	}

	client := &MCPClient{
		Name:     "test-client",
		Filename: "settings.json",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath}
		},
		Paths:             []string{configPath},
		Detect:            func() bool { return true },
		PostInstallNotice: mockNotice,
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	err = ConfigureMCP(client, "/usr/local/bin/git-courer")
	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	if calledWith != "/usr/local/bin/git-courer" {
		t.Errorf("PostInstallNotice not called with expected path: got %q", calledWith)
	}

	var buf [256]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])
	if !strings.Contains(output, "warning: test post install notice") {
		t.Errorf("expected warning printed to stderr, got: %q", output)
	}
}

func TestSetupClient_MultipleClients(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "config1.json")
	path2 := filepath.Join(dir, "config2.json")

	// Mock getMCPClients
	oldGetClients := getMCPClients
	defer func() { getMCPClients = oldGetClients }()

	called1 := false
	called2 := false

	getMCPClients = func() []*MCPClient {
		return []*MCPClient{
			{
				Name:     "duplicate-client",
				Filename: "config1.json",
				RootKey:  "mcpServers",
				ConfigFn: func(binPath string) map[string]interface{} {
					called1 = true
					return map[string]interface{}{"command": binPath}
				},
				Paths:  []string{path1},
				Detect: func() bool { return true },
			},
			{
				Name:     "duplicate-client",
				Filename: "config2.json",
				RootKey:  "mcpServers",
				ConfigFn: func(binPath string) map[string]interface{} {
					called2 = true
					return map[string]interface{}{"command": binPath}
				},
				Paths:  []string{path2},
				Detect: func() bool { return true },
			},
			{
				Name:     "duplicate-client",
				Filename: "config3.json",
				RootKey:  "mcpServers",
				ConfigFn: func(binPath string) map[string]interface{} {
					return map[string]interface{}{}
				},
				Paths:  []string{filepath.Join(dir, "config3.json")},
				Detect: func() bool { return false }, // should NOT be configured
			},
		}
	}

	err := SetupClient("duplicate-client", "/usr/local/bin/git-courer")
	if err != nil {
		t.Fatalf("SetupClient failed: %v", err)
	}

	if !called1 {
		t.Error("first duplicate client was not configured")
	}
	if !called2 {
		t.Error("second duplicate client was not configured")
	}
}

func TestSetupClient_NoDetectedClientsReturnsError(t *testing.T) {
	// Mock getMCPClients
	oldGetClients := getMCPClients
	defer func() { getMCPClients = oldGetClients }()

	getMCPClients = func() []*MCPClient {
		return []*MCPClient{
			{
				Name:   "some-client",
				Detect: func() bool { return false },
			},
		}
	}

	err := SetupClient("some-client", "/usr/local/bin/git-courer")
	if err == nil {
		t.Error("expected error when no clients are detected, got nil")
	}
}

func TestMCPClients_ConfigFn_Pi(t *testing.T) {
	binPath := "/usr/local/bin/git-courer"
	client := findClient(t, "pi")

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

func TestDetect_Pi(t *testing.T) {
	oldLookPath := lookPath
	oldOSStat := osStat
	defer func() {
		lookPath = oldLookPath
		osStat = oldOSStat
	}()

	tests := []struct {
		name        string
		lookPathErr error
		osStatErr   error
		wantDetect  bool
	}{
		{
			name:        "pi in PATH",
			lookPathErr: nil,
			osStatErr:   os.ErrNotExist,
			wantDetect:  true,
		},
		{
			name:        "pi agent dir exists",
			lookPathErr: os.ErrNotExist,
			osStatErr:   nil,
			wantDetect:  true,
		},
		{
			name:        "both exist",
			lookPathErr: nil,
			osStatErr:   nil,
			wantDetect:  true,
		},
		{
			name:        "neither exists",
			lookPathErr: os.ErrNotExist,
			osStatErr:   os.ErrNotExist,
			wantDetect:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lookPath = func(file string) (string, error) {
				if file == "pi" {
					return "/bin/pi", tc.lookPathErr
				}
				return "", os.ErrNotExist
			}
			osStat = func(name string) (os.FileInfo, error) {
				if strings.Contains(name, filepath.Join(".pi", "agent")) {
					return nil, tc.osStatErr
				}
				return nil, os.ErrNotExist
			}

			client := findClient(t, "pi")
			got := client.Detect()
			if got != tc.wantDetect {
				t.Errorf("Detect() = %v, want %v", got, tc.wantDetect)
			}
		})
	}
}

func TestPiPostInstallNotice(t *testing.T) {
	oldHomeDirFn := homeDirFn
	defer func() { homeDirFn = oldHomeDirFn }()

	tempHome := t.TempDir()
	homeDirFn = func() string { return tempHome }

	tests := []struct {
		name         string
		setupFunc    func()
		wantContains string
	}{
		{
			name: "settings.json with npm:pi-mcp-adapter",
			setupFunc: func() {
				dir := filepath.Join(tempHome, ".pi/agent")
				os.MkdirAll(dir, 0755)
				content := `{"packages": ["npm:pi-mcp-adapter"]}`
				os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0644)
			},
			wantContains: "",
		},
		{
			name: "settings.json without npm:pi-mcp-adapter",
			setupFunc: func() {
				dir := filepath.Join(tempHome, ".pi/agent")
				os.MkdirAll(dir, 0755)
				content := `{"packages": ["something-else"]}`
				os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0644)
			},
			wantContains: "pi Agent: extension 'pi-mcp-adapter' not installed. Run 'pi install npm:pi-mcp-adapter' and restart pi.",
		},
		{
			name: "missing settings.json file",
			setupFunc: func() {
				// Do nothing, file doesn't exist
			},
			wantContains: "pi Agent: extension 'pi-mcp-adapter' not found. Run 'pi install npm:pi-mcp-adapter' and restart pi.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear directory/file for each run since they use the same tempHome
			os.RemoveAll(filepath.Join(tempHome, ".pi"))

			tc.setupFunc()
			client := findClient(t, "pi")
			got := client.PostInstallNotice("/usr/local/bin/git-courer")
			if got != tc.wantContains {
				t.Errorf("PostInstallNotice() = %q, want %q", got, tc.wantContains)
			}
		})
	}
}

func TestDetect_AntigravityCLI(t *testing.T) {
	oldLookPath := lookPath
	defer func() {
		lookPath = oldLookPath
	}()

	tests := []struct {
		name        string
		lookPathErr error
		wantDetect  bool
	}{
		{
			name:        "agy in PATH",
			lookPathErr: nil,
			wantDetect:  true,
		},
		{
			name:        "agy not in PATH",
			lookPathErr: os.ErrNotExist,
			wantDetect:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lookPath = func(file string) (string, error) {
				if file == "agy" {
					return "/bin/agy", tc.lookPathErr
				}
				return "", os.ErrNotExist
			}

			client := findClient(t, "antigravity")
			got := client.Detect()
			if got != tc.wantDetect {
				t.Errorf("Detect() = %v, want %v", got, tc.wantDetect)
			}
		})
	}
}

// TestAntigravityClient_HasHooksConfig verifies the antigravity MCPClient
// entry has a non-nil HooksConfig with both HooksPath and PermissionsPath set.
func TestAntigravityClient_HasHooksConfig(t *testing.T) {
	client := findClient(t, "antigravity")
	if client.HooksConfig == nil {
		t.Fatal("antigravity client HooksConfig is nil")
	}
	if client.HooksConfig.HooksPath == "" {
		t.Error("antigravity HooksConfig.HooksPath is empty")
	}
	if client.HooksConfig.PermissionsPath == "" {
		t.Error("antigravity HooksConfig.PermissionsPath is empty")
	}
	if !strings.HasSuffix(client.HooksConfig.HooksPath, "hooks.json") {
		t.Errorf("HooksPath should end with hooks.json: got %q", client.HooksConfig.HooksPath)
	}
	if !strings.HasSuffix(client.HooksConfig.PermissionsPath, "settings.json") {
		t.Errorf("PermissionsPath should end with settings.json: got %q", client.HooksConfig.PermissionsPath)
	}
}

// TestAntigravityInstallUninstallCycle verifies the full install/uninstall
// cycle for the Antigravity client: ConfigureMCP creates hooks.json and
// settings.json with the expected entries, and the Antigravity-specific
// removal functions (removeAntigravityHooks + removeAntigravityPermissions)
// clean them up correctly.
func TestAntigravityInstallUninstallCycle(t *testing.T) {
	dir := t.TempDir()
	binPath := "/usr/local/bin/git-courer"

	hooksPath := filepath.Join(dir, "hooks.json")
	settingsPath := filepath.Join(dir, "settings.json")
	configPath := filepath.Join(dir, "mcp_config.json")

	client := &MCPClient{
		Name:     "antigravity",
		Filename: "mcp_config.json",
		RootKey:  "mcpServers",
		HooksConfig: &HooksConfig{
			HooksPath:       hooksPath,
			PermissionsPath: settingsPath,
		},
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	// Install.
	if err := ConfigureMCP(client, binPath); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	// Verify mcp_config.json contains git-courer.
	cfgData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("mcp_config.json not created: %v", err)
	}
	if !strings.Contains(string(cfgData), "git-courer") {
		t.Errorf("mcp_config.json missing git-courer entry:\n%s", cfgData)
	}

	// Verify hooks.json exists with PreToolUse(run_command) and PreInvocation.
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}
	if !strings.Contains(string(hooksData), "run_command") {
		t.Errorf("hooks.json missing run_command matcher:\n%s", hooksData)
	}
	if !strings.Contains(string(hooksData), "PreInvocation") {
		t.Errorf("hooks.json missing PreInvocation entry:\n%s", hooksData)
	}

	// Verify settings.json has the three permission entries.
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	settingsStr := string(settingsData)
	if !strings.Contains(settingsStr, "mcp(git-courer/*)") {
		t.Errorf("settings.json missing mcp(git-courer/*):\n%s", settingsData)
	}
	if !strings.Contains(settingsStr, "command(git *)") {
		t.Errorf("settings.json missing command(git *):\n%s", settingsData)
	}
	if !strings.Contains(settingsStr, "command(*)") {
		t.Errorf("settings.json missing command(*):\n%s", settingsData)
	}

	// Verify GIT_COURER.md was created.
	rulesPath := filepath.Join(dir, "GIT_COURER.md")
	if _, err := os.Stat(rulesPath); err != nil {
		t.Fatalf("GIT_COURER.md not created: %v", err)
	}

	// Uninstall the Antigravity hooks + permissions directly (the
	// RunUninstall wiring calls these same functions when PermissionsPath is
	// set). This verifies the removal half of the cycle without triggering
	// RunUninstall's binary/global-config side effects.
	if err := removeAntigravityHooks(hooksPath); err != nil {
		t.Fatalf("removeAntigravityHooks: %v", err)
	}
	if err := removeAntigravityPermissions(settingsPath); err != nil {
		t.Fatalf("removeAntigravityPermissions: %v", err)
	}

	// After removal, hooks.json should be gone (no pre-existing backup since
	// it was a fresh install).
	if _, err := os.Stat(hooksPath); err == nil {
		t.Error("hooks.json still exists after removeAntigravityHooks")
	}
	// settings.json should be gone too (no .gc.bak on fresh install).
	if _, err := os.Stat(settingsPath); err == nil {
		t.Error("settings.json still exists after removeAntigravityPermissions")
	}
}

// TestRunUninstall_CallsAntigravityRemoval verifies that RunUninstall calls
// removeAntigravityHooks and removeAntigravityPermissions for an
// Antigravity-style client (PermissionsPath set). It uses the getMCPClients
// override to inject a fake antigravity client pointing at a temp dir, and
// suppresses binary/global-config side effects by pointing HOME at a temp
// dir with no git-courer binary.
func TestRunUninstall_CallsAntigravityRemoval(t *testing.T) {
	dir := t.TempDir()
	binPath := "/nonexistent/git-courer"

	hooksPath := filepath.Join(dir, "hooks.json")
	settingsPath := filepath.Join(dir, "settings.json")
	configPath := filepath.Join(dir, "mcp_config.json")
	rulesPath := filepath.Join(dir, "GIT_COURER.md")

	// Seed files so the removal functions have something to remove.
	if err := installAntigravityHooks(hooksPath, binPath); err != nil {
		t.Fatalf("seed hooks: %v", err)
	}
	if err := installAntigravityPermissions(settingsPath, binPath); err != nil {
		t.Fatalf("seed permissions: %v", err)
	}
	if err := os.WriteFile(rulesPath, []byte("# rules"), 0644); err != nil {
		t.Fatalf("seed GIT_COURER.md: %v", err)
	}
	// Seed an mcp_config.json with git-courer so the "already configured" check
	// path and restoreBackup operate on a real file.
	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{"git-courer":{"command":"x"}}}`), 0644); err != nil {
		t.Fatalf("seed mcp_config.json: %v", err)
	}

	client := &MCPClient{
		Name:     "antigravity",
		Filename: "mcp_config.json",
		RootKey:  "mcpServers",
		HooksConfig: &HooksConfig{
			HooksPath:       hooksPath,
			PermissionsPath: settingsPath,
		},
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	// Mock getMCPClients so RunUninstall only touches our fake client.
	oldGetClients := getMCPClients
	defer func() { getMCPClients = oldGetClients }()
	getMCPClients = func() []*MCPClient { return []*MCPClient{client} }

	// Redirect HOME so the global config removal and binary removal do not
	// touch the real developer environment. FindBinaryPath scans HOME-based
	// paths and PATH; with HOME=dir and no git-courer there, it returns an
	// error and RunUninstall prints "Binary not found" without deleting
	// anything real.
	oldHomeEnv := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHomeEnv)
	// Also clear any PATH entries that could resolve a real git-courer during
	// the test — we only need the temp-dir seeded files removed.
	oldPathEnv := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", oldPathEnv)

	// Suppress stdout/stderr noise from RunUninstall.
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	uninstallErr := RunUninstall()
	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	// drain pipe to avoid blocking
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if err != nil || n == 0 {
			break
		}
	}

	if uninstallErr != nil {
		t.Fatalf("RunUninstall: %v", uninstallErr)
	}

	// GIT_COURER.md should be removed.
	if _, err := os.Stat(rulesPath); err == nil {
		t.Error("GIT_COURER.md still exists after uninstall")
	}
	// hooks.json should be gone.
	if _, err := os.Stat(hooksPath); err == nil {
		t.Error("hooks.json still exists after uninstall")
	}
	// settings.json should be gone.
	if _, err := os.Stat(settingsPath); err == nil {
		t.Error("settings.json still exists after uninstall")
	}
}
