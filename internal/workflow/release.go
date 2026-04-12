// ReleaseService orchestrates the release workflow:
// get release intent → LLM interprets → generate changelog → create tag → create GitHub release.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// ReleaseServiceConfig holds tuneable values for the release service.
type ReleaseServiceConfig struct {
	ContextWindow       int    // LLM context window size
	MaxCommitsPerChunk  int    // max commits per chunk sent to LLM
	LogPath             string // path to release log file
	MaxLogLines         int    // circular buffer size for task.log
	BackgroundThreshold int    // chunks above which run async
}

// DefaultReleaseServiceConfig returns sensible defaults derived from Ollama context window.
func DefaultReleaseServiceConfig(contextWindow, maxCommitsPerChunk, maxLogLines int, logPath string) ReleaseServiceConfig {
	cw := contextWindow
	if cw == 0 {
		cw = 4096
	}
	mcc := maxCommitsPerChunk
	if mcc == 0 {
		mcc = 20
	}
	return ReleaseServiceConfig{
		ContextWindow:       cw,
		MaxCommitsPerChunk:  mcc,
		LogPath:             logPath,
		MaxLogLines:         maxLogLines,
		BackgroundThreshold: 3,
	}
}

// LogChunker splits a list of commits into chunks for changelog generation.
type LogChunker interface {
	// Chunk splits commits into chunks.
	Chunk(commits string, maxPerChunk int) ([]string, error)
}

// ReleaseService handles the release workflow.
type ReleaseService struct {
	git        ports.Git
	llm        ports.LLM
	logChunker LogChunker
	taskLog    *releaseLogger
	cfg        ReleaseServiceConfig
}

// NewReleaseService creates a new ReleaseService.
func NewReleaseService(git ports.Git, llm ports.LLM, logChunker LogChunker, cfg ReleaseServiceConfig) *ReleaseService {
	return &ReleaseService{
		git:        git,
		llm:        llm,
		logChunker: logChunker,
		taskLog:    newReleaseLogger(cfg.LogPath, cfg.MaxLogLines),
		cfg:        cfg,
	}
}

// ReleaseResult holds the outcome of a release operation.
type ReleaseResult struct {
	Operation       string   `json:"operation"`
	TagName         string   `json:"tag_name,omitempty"`
	Changelog       string   `json:"changelog,omitempty"`
	IsGitHubRelease bool     `json:"is_github_release,omitempty"`
	Message         string   `json:"result,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Type            string   `json:"type"`
}

type preparedReleaseState struct {
	intent    *domain.ReleaseIntent
	commits   string
	chunks    []string
	changelog string
}

// Prepare gets release intent and commits since last tag.
// Returns the release intent, commits, and any warnings.
func (s *ReleaseService) Prepare(instruction string) (*domain.ReleaseIntent, string, []string, error) {
	s.taskLog.logStart()

	// Get current releases for context
	releasesList, err := s.git.ListTags()
	if err != nil {
		releasesList = []string{}
	}
	releases := strings.Join(releasesList, "\n")

	// Detect branch flow
	branchFlow, _ := s.DetectBranchFlow()

	// Interpret release intent
	intent, err := s.llm.InterpretReleaseIntent(instruction, releases)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("failed to interpret release intent: %v", err))
		return nil, "", []string{err.Error()}, fmt.Errorf("failed to interpret release intent: %w", err)
	}

	s.taskLog.logIntent(intent.TagName, intent.VersionBump, branchFlow)

	// Get commits since last tag
	var commits string
	if intent.TagName != "" {
		// User specified a specific tag, get commits since then
		commits, err = s.git.CommitsFromTag(intent.TagName)
		if err != nil {
			s.taskLog.logError(fmt.Sprintf("failed to get commits from tag %s: %v", intent.TagName, err))
			// Try from latest tag
			latestTag, lterr := s.git.LatestTag()
			if lterr == nil && latestTag != "" {
				commits, _ = s.git.CommitsFromTag(latestTag)
			}
		}
	} else {
		// Use latest tag
		latestTag, err := s.git.LatestTag()
		if err != nil {
			s.taskLog.logError("no tags found, using all commits")
			commits, _ = s.git.Log(100)
		} else {
			commits, err = s.git.CommitsFromTag(latestTag)
			if err != nil {
				s.taskLog.logError(fmt.Sprintf("failed to get commits from tag %s: %v", latestTag, err))
				commits, _ = s.git.Log(100)
			}
		}
	}

	if commits == "" {
		s.taskLog.logError("no commits found")
		return intent, "", []string{"no commits found"}, fmt.Errorf("no commits found")
	}

	s.taskLog.logCommits(s.countLines(commits))

	return intent, commits, nil, nil
}

// Generate generates the changelog from commits.
// Returns the generated changelog and any warnings.
// Supports background processing for large numbers of commits.
func (s *ReleaseService) Generate(commits string) (string, []string, error) {
	s.taskLog.logStartChangelog()

	chunks, err := s.logChunker.Chunk(commits, s.cfg.MaxCommitsPerChunk)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("failed to chunk commits: %v", err))
		return "", []string{err.Error()}, fmt.Errorf("failed to chunk commits: %w", err)
	}

	s.taskLog.logChunks(len(chunks))

	if len(chunks) > s.cfg.BackgroundThreshold {
		return s.generateBackground(chunks)
	}

	return s.generateSync(chunks)
}

func (s *ReleaseService) generateSync(chunks []string) (string, []string, error) {
	s.taskLog.logStart()
	var warnings []string
	var changelogContent string
	var previousChangelog string

	// Create temp file for incremental changelog
	tmpFile := filepath.Join(os.TempDir(), "changelog-"+time.Now().Format("20060102150405")+".md")
	defer os.Remove(tmpFile)

	// Generate changelog for each chunk - writes to file
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChan := make(chan chunkChangelogResult, len(chunks))
	go func() {
		defer close(resultChan)
		for i, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}
			// GenerateChangelog returns changelog string
			result, err := s.llm.GenerateChangelog(chunk, previousChangelog, tmpFile)
			select {
			case <-ctx.Done():
				return
			case resultChan <- chunkChangelogResult{chunk: chunk, index: i, result: result, err: err}:
			}
			// Update previous for next iteration
			if result != "" {
				previousChangelog = result
			}
		}
	}()

	for r := range resultChan {
		if r.err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			continue
		}
	}

	// Build changelog by calling PolishChangelog with the raw chunks
	chunksForPolish := make([]string, len(chunks))
	copy(chunksForPolish, chunks)
	polished, err := s.llm.PolishChangelog(chunksForPolish)
	if err != nil {
		// If polish fails, use raw chunks as changelog
		changelogContent = strings.Join(chunks, "\n\n")
	} else {
		changelogContent = polished
	}

	s.taskLog.logChangelogDone(len(chunks))
	return changelogContent, warnings, nil
}

func (s *ReleaseService) generateBackground(chunks []string) (string, []string, error) {
	s.taskLog.logStart()
	s.taskLog.logProgress(0, len(chunks))

	go func() {
		var chunksProcessed int
		resultChan := make(chan chunkChangelogResult, len(chunks))
		go func() {
			for i, chunk := range chunks {
				result, err := s.llm.GenerateChangelog(chunk, "", "")
				resultChan <- chunkChangelogResult{chunk: chunk, index: i, result: result, err: err}
			}
			close(resultChan)
		}()

		var changelogContent string
		for r := range resultChan {
			if r.err != nil {
				s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
				continue
			}
			if r.result != "" {
				if changelogContent != "" {
					changelogContent += "\n\n"
				}
				changelogContent += r.result
			}
			chunksProcessed++
			s.taskLog.logProgress(chunksProcessed, len(chunks))
		}

		if chunksProcessed > 0 {
			s.taskLog.logChangelogDone(chunksProcessed)
		}
		s.taskLog.logDone()
	}()

	resp, _ := json.Marshal(map[string]any{
		"operation": "release", "type": "write", "state": "running",
		"message": fmt.Sprintf("Processing %d chunks in background. Check %q for progress.", len(chunks), s.cfg.LogPath),
	})
	return string(resp), nil, nil
}

// chunkChangelogResult holds the result of generating changelog for a chunk.
type chunkChangelogResult struct {
	chunk  string
	index  int
	result string // changelog generated for this chunk
	err    error
}

// BuildPreview formats the release preview for user confirmation.
// Returns a formatted string for preview.
func (s *ReleaseService) BuildPreview(intent *domain.ReleaseIntent, changelog string) string {
	var b strings.Builder
	b.WriteString("📦 Release Preview\n")
	b.WriteString("==================\n\n")
	b.WriteString(fmt.Sprintf("Tag: %s\n", intent.TagName))
	if intent.VersionBump != "" {
		b.WriteString(fmt.Sprintf("Version Bump: %s\n", intent.VersionBump))
	}
	b.WriteString("\n--- Changelog ---\n")
	b.WriteString(changelog)
	b.WriteString("\n\n")
	b.WriteString("Do you want to proceed?")
	return b.String()
}

// Execute creates the git tag and optional GitHub release.
// Includes security checks:
// - Validate tag with IsValidTagName
// - Check TagExists before creating
// - Check IsGHAuthenticated before gh release
// - Don't ignore gh errors
func (s *ReleaseService) Execute(intent *domain.ReleaseIntent, changelog string, createGHRelease bool) (string, error) {
	s.taskLog.logStart()

	// Security Check 1: Validate tag name
	if !domain.IsValidTagName(intent.TagName) {
		s.taskLog.logError(fmt.Sprintf("invalid tag name: %s", intent.TagName))
		return "", fmt.Errorf("invalid tag name: %s (must be semver like v1.0.0 or 1.0.0)", intent.TagName)
	}

	// Security Check 2: Check if tag already exists
	exists, err := s.git.TagExists(intent.TagName)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("failed to check tag existence: %v", err))
		return "", fmt.Errorf("failed to check tag existence: %w", err)
	}
	if exists {
		s.taskLog.logError(fmt.Sprintf("tag already exists: %s", intent.TagName))
		return "", fmt.Errorf("tag already exists: %s", intent.TagName)
	}

	// Create git tag
	_, err = s.git.Tag(intent.TagName)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("failed to create tag: %v", err))
		return "", fmt.Errorf("failed to create tag: %w", err)
	}
	s.taskLog.logTag(intent.TagName)

	// Optional GitHub release
	var ghResult string
	var isGHRelease bool
	if createGHRelease {
		// Security Check 3: Check GitHub authentication
		authenticated, err := s.git.IsGHAuthenticated()
		if err != nil {
			s.taskLog.logError(fmt.Sprintf("failed to check GH authentication: %v", err))
			// Don't ignore GH errors - fail the operation
			return "", fmt.Errorf("failed to check GH authentication: %w", err)
		}
		if !authenticated {
			s.taskLog.logError("GitHub CLI not authenticated")
			return "", fmt.Errorf("GitHub CLI not authenticated. Run 'gh auth login' first")
		}

		// Create GitHub release
		ghResult, err = s.git.CreateRelease(intent.TagName, changelog)
		if err != nil {
			s.taskLog.logError(fmt.Sprintf("failed to create GitHub release: %v", err))
			// Don't ignore gh errors - the tag was created, but GH release failed
			return "", fmt.Errorf("git tag created, but GitHub release failed: %w", err)
		}
		s.taskLog.logGHRelease(intent.TagName)
		isGHRelease = true
	}

	s.taskLog.logDone()

	result := ReleaseResult{
		Operation:       "release",
		TagName:         intent.TagName,
		Changelog:       changelog,
		IsGitHubRelease: isGHRelease,
		Type:            "write",
	}

	if ghResult != "" {
		result.Message = fmt.Sprintf("Tag %s created%s", intent.TagName, ghResult)
	} else {
		result.Message = fmt.Sprintf("Tag %s created", intent.TagName)
	}

	resp, _ := json.Marshal(result)
	return string(resp), nil
}

// DetectBranchFlow detects if the repository uses git flow.
// Returns the detected branch flow pattern: "gitflow", "trunk", or "unknown".
func (s *ReleaseService) DetectBranchFlow() (string, error) {
	branches, err := s.git.ListBranches()
	if err != nil {
		return "unknown", err
	}

	hasDevelop := strings.Contains(branches, "develop")
	hasDev := strings.Contains(branches, "dev")
	hasMain := strings.Contains(branches, "main")
	hasMaster := strings.Contains(branches, "master")

	// Git Flow: has develop and main/master
	if (hasDevelop || hasDev) && (hasMain || hasMaster) {
		return "gitflow", nil
	}

	// Trunk-based: only main/master
	if hasMain || hasMaster {
		return "trunk", nil
	}

	return "unknown", nil
}

func (s *ReleaseService) countLines(ss string) int {
	if ss == "" {
		return 0
	}
	return strings.Count(ss, "\n") + 1
}

// --- Release logger (circular buffer) ---

type releaseLogger struct {
	logPath     string
	maxLogLines int
}

func newReleaseLogger(logPath string, maxLogLines int) *releaseLogger {
	os.MkdirAll(filepath.Dir(logPath), 0755)
	return &releaseLogger{logPath: logPath, maxLogLines: maxLogLines}
}

func (l *releaseLogger) log(entryType, message string) {
	entry := fmt.Sprintf("%s [%s] %s", time.Now().Format("15:04:05"), entryType, message)
	lines, _ := l.readLines()
	lines = append(lines, entry)
	if len(lines) > l.maxLogLines {
		lines = lines[len(lines)-l.maxLogLines:]
	}
	l.writeLines(lines)
}

func (l *releaseLogger) readLines() ([]string, error) {
	data, err := os.ReadFile(l.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

func (l *releaseLogger) writeLines(lines []string) {
	os.WriteFile(l.logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func (l *releaseLogger) logStart() { l.log("START", "release task began") }
func (l *releaseLogger) logIntent(tag, bump, flow string) {
	l.log("INTENT", fmt.Sprintf("tag=%s bump=%s flow=%s", tag, bump, flow))
}
func (l *releaseLogger) logCommits(count int) {
	l.log("COMMITS", fmt.Sprintf("%d commits to process", count))
}
func (l *releaseLogger) logChunks(count int)    { l.log("CHUNKS", fmt.Sprintf("%d chunks", count)) }
func (l *releaseLogger) logStartChangelog()     { l.log("CHANGELOG", "generating changelog") }
func (l *releaseLogger) logChangelog(cl string) { l.log("CHANGELOG", cl) }
func (l *releaseLogger) logChangelogDone(count int) {
	l.log("CHANGELOG", fmt.Sprintf("done, %d sections", count))
}
func (l *releaseLogger) logProgress(done, total int) {
	l.log("PROGRESS", fmt.Sprintf("%d/%d chunks", done, total))
}
func (l *releaseLogger) logTag(tag string) { l.log("TAG", tag) }
func (l *releaseLogger) logGHRelease(tag string) {
	l.log("GH", fmt.Sprintf("release created for %s", tag))
}
func (l *releaseLogger) logError(msg string) { l.log("ERROR", msg) }
func (l *releaseLogger) logDone()            { l.log("DONE", "release completed") }
