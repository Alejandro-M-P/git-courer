package commit_analysis

import (
	"testing"
	"time"
)

// =============================================================================
// SINGLE COMMIT TESTS
// =============================================================================

func TestSingle_Feature(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/user.go": `package main
type User struct {
    ID   int
    Name string
}
func NewUser(name string) *User {
    return &User{ID: 1, Name: name}
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

	t.Logf("Time: %v", elapsed)
	t.Logf("Strategy: %s", analysis.Strategy)
	t.Logf("Groups: %d", len(analysis.Groups))

	if len(analysis.Groups) > 0 {
		for i, g := range analysis.Groups {
			t.Logf("  Group %d [%s]: %s", i+1, g.Type, g.Message)
		}
	}

	if analysis.Strategy != "single" {
		t.Errorf("Expected single strategy, got %s", analysis.Strategy)
	}
}

func TestSingle_BugFix(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/calculator.go": `package main
func Divide(a, b int) int {
    if b == 0 {
        return 0 // was: panic
    }
    return a / b
}`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Groups: %d", analysis.Strategy, len(analysis.Groups))

	if len(analysis.Groups) > 0 {
		t.Logf("Message: %s", analysis.Groups[0].Message)
		if analysis.Groups[0].Type == "fix" {
			t.Log("✅ Correctly identified as fix")
		}
	}
}

func TestSingle_Refactor(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"internal/parser.go": `package internal
// Refactored to use runes instead of indices
func Parse(s string) {
    for i, r := range s {
        _ = i
        println(string(r))
    }
}`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Groups: %d", analysis.Strategy, len(analysis.Groups))

	if len(analysis.Groups) > 0 && analysis.Groups[0].Type == "refactor" {
		t.Log("✅ Correctly identified as refactor")
	}
}

// =============================================================================
// MULTIPLE COMMIT TESTS (SPLITTING)
// =============================================================================

func TestSplit_MultipleCodeFiles(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/auth.go":   `package main\nfunc Auth() error { return nil }`,
		"src/user.go":   `package main\ntype User struct { ID int }`,
		"src/config.go": `package main\nconst Port = 8080`,
		"src/utils.go":  `package main\nfunc Add(a, b int) int { return a + b }`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Groups: %d", analysis.Strategy, len(analysis.Groups))

	if len(analysis.Groups) > 1 {
		t.Log("✅ Multiple groups generated")
		for i, g := range analysis.Groups {
			t.Logf("  Group %d [%s]: %d files", i+1, g.Type, len(g.Files))
		}
	}
}

func TestSplit_MixedTypes(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/auth.go":        `package main\nfunc Auth() error { return nil }`,
		"docs/api.md":        `# API Documentation`,
		"package.json":       `{"name": "test"}`,
		"tests/main_test.go": `package main\nfunc Test(t *testing.T) {}`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s, Groups: %d", analysis.Strategy, len(analysis.Groups))

	for i, g := range analysis.Groups {
		t.Logf("  Group %d [%s]: %v", i+1, g.Type, g.Files)
	}

	if analysis.Strategy == "split" && len(analysis.Groups) >= 2 {
		t.Log("✅ Correctly split into multiple commits")
	}
}

// =============================================================================
// CODE WITH PROBLEMS TESTS
// =============================================================================

func TestCodeWithSecrets_ExcludeSecretsKeepCode(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/user.go": `package main
type User struct {
    ID   int
    Name string
}`,
		"src/secrets.go": `package main
const API_KEY = "sk-real-secret"`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s", analysis.Strategy)
	t.Logf("Groups: %d", len(analysis.Groups))
	t.Logf("Excluded: %v", analysis.Excluded)

	if len(analysis.Excluded) > 0 {
		t.Log("✅ Correctly excluded secret files")
	}

	if len(analysis.Groups) > 0 {
		t.Logf("Code commit: %s", analysis.Groups[0].Message)
	}
}

func TestCodeWithDeps_ExcludeDepsKeepCode(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/user.go":  `package main\ntype User struct { ID int }`,
		"package.json": `{"dependencies": {"lodash": "^4.0.0"}}`,
		"go.sum":       `github.com/pkg/errors v0.9.1 h1:abc`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Strategy: %s", analysis.Strategy)
	t.Logf("Groups: %d", len(analysis.Groups))
	t.Logf("Excluded: %v", analysis.Excluded)
	t.Logf("Warnings: %v", analysis.Warnings)
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestEdge_EmptyDiff(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	analysis, err := adapter.AnalyzeAndPlanCommit([]string{}, "")
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Empty diff result: %+v", analysis)
}

func TestEdge_TrulyEmptyRepo(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{}

	dir, fileList := setupGitRepo(t, files)
	_ = dir

	diff := getStagedDiff(t, dir)

	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Empty repo result: Strategy=%s, Groups=%d", analysis.Strategy, len(analysis.Groups))
}

// =============================================================================
// SPEED TESTS
// =============================================================================

func TestSpeed_SingleFile(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/main.go": `package main
func main() { println("hello") }`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	iterations := 3
	var total time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
		total += time.Since(start)
		if err != nil {
			t.Fatalf("Iteration %d failed: %v", i+1, err)
		}
	}

	avg := total / time.Duration(iterations)
	t.Logf("Average time for single file: %v (over %d iterations)", avg, iterations)

	if avg > 10*time.Second {
		t.Errorf("Average time too slow: %v", avg)
	}
}

func TestSpeed_MultipleFiles(t *testing.T) {
	if !adapter.IsAvailable() {
		t.Skip("Ollama not available")
	}

	files := map[string]string{
		"src/a.go": `package main\nconst A = 1`,
		"src/b.go": `package main\nconst B = 2`,
		"src/c.go": `package main\nconst C = 3`,
		"src/d.go": `package main\nconst D = 4`,
		"src/e.go": `package main\nconst E = 5`,
	}

	dir, fileList := setupGitRepo(t, files)
	diff := getStagedDiff(t, dir)

	start := time.Now()
	analysis, err := adapter.AnalyzeAndPlanCommit(fileList, diff)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Time for 5 files: %v", elapsed)
	t.Logf("Strategy: %s, Groups: %d", analysis.Strategy, len(analysis.Groups))
}
