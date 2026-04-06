package commit_analysis

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	ollama "github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
)

const (
	ollamaHost  = "http://localhost:11434"
	modelName   = "qwen3.5:latest"
	testDataDir = "./test_data"
)

var adapter *ollama.Adapter

func init() {
	adapter = ollama.NewAdapter(ollamaHost, modelName, "")
}

func setupGitRepo(t *testing.T, files map[string]string) (string, []string) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = tmpDir
	cmd.Run()

	var fileList []string
	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
		fileList = append(fileList, path)
	}

	for _, f := range fileList {
		cmd := exec.Command("git", "add", f)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to stage %s: %v", f, err)
		}
	}

	return tmpDir, fileList
}

func getStagedDiff(t *testing.T, dir string) string {
	cmd := exec.Command("git", "diff", "--cached")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}
	return string(output)
}

// =============================================================================
// SECRETS DETECTION TESTS
// =============================================================================

func TestSecrets_APIKeys(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"config.go": `package main
const API_KEY = "sk-1234567890abcdef"
const DB_PASSWORD = "postgres://user:secret@localhost"`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)

	if analysis.Strategy == "skip" && len(analysis.Excluded) > 0 {
		t.Log("✅ Correctly detected and excluded secrets")
	} else {
		t.Logf("⚠️ Analysis result: %+v", analysis)
	}
}

func TestSecrets_GitHubTokens(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"auth.go": `package main
const GITHUB_TOKEN = "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
const OAUTH_SECRET = "oidc_secret_1234567890"`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)

	if len(analysis.Excluded) > 0 || analysis.Strategy == "skip" {
		t.Log("✅ Correctly detected token-like secrets")
	}
}

func TestSecrets_AWSKeys(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"aws.go": `package main
const AWS_ACCESS_KEY = "AKIAIOSFODNN7EXAMPLE"
const AWS_SECRET = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)
}

func TestSecrets_EnvFile(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		".env": `DATABASE_URL=postgres://user:secret123@localhost:5432/mydb
API_KEY=sk-1234567890abcdef1234567890abcdef
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	start := time.Now()
	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Time: %v, Strategy: %s", elapsed, analysis.Strategy)
	t.Logf("Excluded: %v", analysis.Excluded)
	t.Logf("Warnings: %v", analysis.Warnings)

	if len(analysis.Excluded) > 0 {
		t.Log("✅ .env file excluded")
	}
}

// =============================================================================
// DEPENDENCIES DETECTION TESTS
// =============================================================================

func TestDeps_NodeModules(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"node_modules/lodash.js":        `// lodash v4.17.21`,
		"node_modules/express/index.js": `// express framework`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)

	if analysis.Strategy == "skip" {
		t.Log("✅ Correctly identified deps-only scenario")
	}
}

func TestDeps_GoVendor(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"vendor/github.com/pkg/errors": `package errors`,
		"vendor/gopkg.in/yaml.v3":      `package yaml`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)
}

func TestDeps_LockFiles(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"package-lock.json": `{"dependencies": {"lodash": "^4.0.0"}}`,
		"yarn.lock":         `# yarn lockfile`,
		"Cargo.lock":        `# Cargo lockfile`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)
}

// =============================================================================
// BINARY FILES DETECTION TESTS
// =============================================================================

func TestBinaries_Images(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"images/logo.png":   "\x89PNG\r\n\x1a\n", // PNG header
		"images/icon.jpg":   "\xff\xd8\xff",      // JPEG header
		"images/banner.gif": "GIF89a",            // GIF header
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)
}

func TestBinaries_Documents(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"docs/manual.pdf":        "%PDF-1.4",
		"docs/presentation.pptx": "PK\x03\x04", // ZIP-based
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)
}

func TestBinaries_Executables(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"bin/app":       "\x7fELF",  // Linux ELF
		"build/app.exe": "\x4d\x5a", // Windows PE
		"libmath.so":    "ELF",      // Shared library
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Excluded: %v", analysis.Strategy, analysis.Excluded)
}

// =============================================================================
// EXPORT TEST RESULTS
// =============================================================================

func TestExport_ResultsToJSON(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/user.go": `package main
type User struct {
    ID    int
    Name  string
    Email string
}
func NewUser(name, email string) *User {
    return &User{ID: 1, Name: name, Email: email}
}`,
		"src/auth.go": `package main
import "errors"
func Auth(user, pass string) error {
    if user == "" || pass == "" {
        return errors.New("empty credentials")
    }
    return nil
}`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	start := time.Now()
	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	result := map[string]interface{}{
		"test_name":   "secrets_detection",
		"elapsed":     elapsed.String(),
		"elapsed_sec": elapsed.Seconds(),
		"file_count":  len(fileList),
		"files":       fileList,
		"diff_length": len(diff),
		"analysis":    analysis,
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	os.MkdirAll(testDataDir, 0755)
	os.WriteFile(filepath.Join(testDataDir, "secrets_test.json"), jsonBytes, 0644)

	t.Logf("Results exported to %s/secrets_test.json", testDataDir)
	t.Logf("\n%s", string(jsonBytes))
}
