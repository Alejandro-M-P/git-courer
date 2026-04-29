// ReleaseService orchestrates the release workflow:
// get release intent → LLM interprets → generate changelog → create tag → create GitHub release.
package workflow

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// ReleaseServiceConfig holds tuneable values for the release service.
type ReleaseServiceConfig struct {
	ContextWindow       int    // LLM context window size
	MaxCommitsPerChunk  int    // max commits per chunk sent to LLM
	LogPath             string // path to release log file
	MaxLogLines         int    // circular buffer size for task.log
	BackgroundThreshold int // chunks above which run async
}

// DefaultReleaseServiceConfig returns sensible defaults derived from Ollama context window.
func DefaultReleaseServiceConfig(contextWindow, maxCommitsPerChunk, maxLogLines int, logPath string) ReleaseServiceConfig {
	return DefaultReleaseServiceConfigWithPaths(contextWindow, maxCommitsPerChunk, maxLogLines, logPath)
}

// DefaultReleaseServiceConfigWithPaths returns config with explicit log path.
func DefaultReleaseServiceConfigWithPaths(contextWindow, maxCommitsPerChunk, maxLogLines int, logPath string) ReleaseServiceConfig {
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
	git          ports.Git
	llm          ports.LLM
	logChunker   LogChunker
	taskLog      *releaseLogger
	cfg          ReleaseServiceConfig
	mu           sync.Mutex
	pendingState string
	pendingIntent    *domain.ReleaseIntent
	pendingChangelog string
}

// NewReleaseService creates a new ReleaseService.
func NewReleaseService(git ports.Git, llm ports.LLM, logChunker LogChunker, cfg ReleaseServiceConfig) *ReleaseService {
	return &ReleaseService{
		git:          git,
		llm:          llm,
		logChunker:   logChunker,
		taskLog:      newReleaseLogger(cfg.LogPath, cfg.MaxLogLines),
		cfg:          cfg,
		pendingState: "",
	}
}

// GetConfig returns the service configuration.
func (s *ReleaseService) GetConfig() ReleaseServiceConfig {
	return s.cfg
}

func (s *ReleaseService) setPendingState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingState = state
}

func (s *ReleaseService) SaveState(state string) {
	s.setPendingState(state)
}

func (s *ReleaseService) LoadState() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingState
}

func (s *ReleaseService) SaveIntent(intent *domain.ReleaseIntent) {
	s.setIntent(intent)
}

func (s *ReleaseService) setIntent(intent *domain.ReleaseIntent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingIntent = intent
}

func (s *ReleaseService) LoadIntent() (*domain.ReleaseIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingIntent == nil {
		return nil, fmt.Errorf("no release intent")
	}
	return s.pendingIntent, nil
}

func (s *ReleaseService) SaveChangelog(changelog string) {
	s.setChangelog(changelog)
}

func (s *ReleaseService) setChangelog(changelog string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingChangelog = changelog
}

func (s *ReleaseService) LoadChangelog() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingChangelog, nil
}

func (s *ReleaseService) ClearPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingState = ""
	s.pendingIntent = nil
	s.pendingChangelog = ""
}

// ReleaseResult holds the outcome of a release operation.
type ReleaseResult struct {
	Operation       string   `json:"operation"`
	TagName   string `json:"tag_name,omitempty"`
	Changelog string   `json:"changelog,omitempty"`
	Message   string   `json:"result,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Type      string   `json:"type"`
}

type preparedReleaseState struct {
	intent    *domain.ReleaseIntent
	commits   string
	chunks    []string
	changelog string
}

// Prepare gets release intent and commits since last tag.
// NO LLM - uses regex to parse instruction and calculates bump from commits.
// If userBump is provided, use it; otherwise calculate from commits.
// Returns the release intent, commits, and any warnings.
func (s *ReleaseService) Prepare(instruction string, userBump string) (*domain.ReleaseIntent, string, []string, error) {
	s.taskLog.logStart()

	// Get current releases for context
	releasesList, err := s.git.ListTags()
	if err != nil {
		releasesList = []string{}
	}

	// Get current branch
	currentBranch, _ := s.git.CurrentBranch()

	// Parse release intent from instruction using regex (NO LLM)
	intent := parseReleaseIntent(instruction, releasesList)

	s.taskLog.logIntent(intent.TagName, intent.VersionBump, currentBranch)

	// Get commits since last tag
	var commits string
	if intent.TagName != "" {
		// intent.TagName is the NEW tag to release. Use the previous tag as reference.
		prevTag := previousTag(releasesList, intent.TagName)
		if prevTag != "" {
			commits, err = s.git.CommitsFromTag(prevTag)
			if err != nil {
				s.taskLog.logError(fmt.Sprintf("failed to get commits from prev tag %s: %v", prevTag, err))
				commits, _ = s.git.LogFull(100)
			}
		} else {
			commits, _ = s.git.LogFull(100)
		}
	} else {
		// Use latest tag
		latestTag, err := s.git.LatestTag()
		if err != nil {
			s.taskLog.logError("no tags found, using all commits")
			commits, _ = s.git.LogFull(100)
		} else {
			commits, err = s.git.CommitsFromTag(latestTag)
			if err != nil {
				s.taskLog.logError(fmt.Sprintf("failed to get commits from tag %s: %v", latestTag, err))
				commits, _ = s.git.LogFull(100)
			}
		}
	}

	if commits == "" {
		s.taskLog.logError("no commits found")
		return intent, "", []string{"no commits found"}, fmt.Errorf("no commits found")
	}

	s.taskLog.logCommits(s.countLines(commits))

	// Calculate bump:
	// - If userBump provided → use it (user always has final say)
	// - Otherwise → Go calculates deterministically from commits
	var warnings []string
	goBump := domain.CalculateBump(strings.Split(commits, "\n"))
	actualBump := goBump
	if userBump != "" {
		// User explicitly requested a bump type - always use it
		actualBump = userBump
		warnings = append(warnings, fmt.Sprintf("bump type: usuario eligió %q", userBump))
	}

	// Apply the actual bump (skip if user specified version explicitly)
	if !intent.UserSpecifiedVersion && actualBump != "" {
		prevTag := previousTag(releasesList, intent.TagName)
		if prevTag != "" {
			if newTag, err := domain.BumpVersion(prevTag, actualBump); err == nil {
				intent.VersionBump = actualBump
				intent.TagName = newTag
			}
		}
	}

	return intent, commits, warnings, nil
}