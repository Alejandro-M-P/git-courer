// ReleaseService orchestrates the release workflow:
// get release intent → LLM interprets → generate changelog → create tag → create GitHub release.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// ReleaseServiceConfig holds tuneable values for the release service.
type ReleaseServiceConfig struct {
	ContextWindow      int    // LLM context window size
	MaxCommitsPerChunk int    // max commits per chunk sent to LLM
	LogPath            string // path to release log file
	MaxLogLines        int    // circular buffer size for task.log
	NumParallel        int    // max concurrent LLM calls (default: 1 = serial)
	Context            string // optional project context for prompt injection
	WorkDir            string // path to working directory
	ReleaseType        string // release type: "tag" or "github"
}

// DefaultReleaseServiceConfig is an alias of DefaultReleaseServiceConfigWithPaths.
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
		ContextWindow:      cw,
		MaxCommitsPerChunk: mcc,
		LogPath:            logPath,
		MaxLogLines:        maxLogLines,
		NumParallel:        1,
	}
}

// LogChunker splits a list of commits into chunks for changelog generation.
type LogChunker interface {
	// Chunk splits commits into chunks.
	Chunk(commits string, maxPerChunk int) ([]string, error)
}

// ReleaseService handles the release workflow.
type ReleaseService struct {
	git              ports.Git
	llm              ports.LLM
	logChunker       LogChunker
	githubAPI        ports.GitHubAPI // opt-in: nil means no PR enrichment
	cfg              ReleaseServiceConfig
	projectCfg       *domain.ProjectConfig // nil if init hasn't run
	mu               sync.Mutex
	pendingState     string
	pendingIntent    *domain.ReleaseIntent
	pendingChangelog string
	progressCb       func(done, total int)
	doneCb           func(changelog string)
	commitStore      ports.CommitStore // nil means no-op (no read/clear)
}

func (s *ReleaseService) SetProgressCallback(fn func(done, total int)) {
	s.mu.Lock()
	s.progressCb = fn
	s.mu.Unlock()
}

func (s *ReleaseService) SetDoneCallback(fn func(changelog string)) {
	s.mu.Lock()
	s.doneCb = fn
	s.mu.Unlock()
}

// SetContext sets the project context on the LLM adapter if it supports it.
func (s *ReleaseService) SetContext(context string) {
	if setter, ok := s.llm.(interface{ SetContext(string) }); ok {
		setter.SetContext(context)
	}
}

// NewReleaseService creates a new ReleaseService.
// githubAPI is optional — pass nil to disable PR enrichment.
// commitStore is optional — pass nil to disable commit metadata read/clear.
func NewReleaseService(git ports.Git, llm ports.LLM, logChunker LogChunker, cfg ReleaseServiceConfig, githubAPI ports.GitHubAPI, commitStore ports.CommitStore) *ReleaseService {
	if cfg.NumParallel <= 0 {
		cfg.NumParallel = 1
	}

	var projectCfg *domain.ProjectConfig
	if cfg.Context != "" {
		if setter, ok := llm.(interface{ SetContext(string) }); ok {
			setter.SetContext(cfg.Context)
		}
	} else {
		if loaded, err := domain.LoadProjectConfig("."); err == nil && loaded != nil {
			projectCfg = loaded
			if scopeCtx := loaded.FormatScopeContext(); scopeCtx != "" {
				if setter, ok := llm.(interface{ SetContext(string) }); ok {
					setter.SetContext(scopeCtx)
				}
			}
		}
	}
	return &ReleaseService{
		git:          git,
		llm:          llm,
		logChunker:   logChunker,
		githubAPI:    githubAPI,
		cfg:          cfg,
		projectCfg:   projectCfg,
		pendingState: "",
		commitStore:  commitStore,
	}
}

func sanitizeBranchName(name string) string {
	r := strings.ReplaceAll(name, "/", "-")
	for _, ch := range []string{"~", "^", ":", "\\", " "} {
		r = strings.ReplaceAll(r, ch, "")
	}
	for strings.Contains(r, "--") {
		r = strings.ReplaceAll(r, "--", "-")
	}
	r = strings.Trim(r, "-")
	if r == "" {
		return "HEAD"
	}
	return r
}

func (s *ReleaseService) getReleaseDir() (string, error) {
	if s.cfg.WorkDir == "" {
		return "", nil // Fallback to in-memory mode
	}
	currentBranch, err := s.git.CurrentBranch()
	var branchDir string
	if err == nil && currentBranch != "" {
		branchDir = filepath.Join("branches", sanitizeBranchName(currentBranch))
	}
	return filepath.Join(s.cfg.WorkDir, ".git-courer", branchDir), nil
}

func (s *ReleaseService) setPendingState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingState = state
}

func (s *ReleaseService) SaveState(state string) {
	s.setPendingState(state)
	dir, err := s.getReleaseDir()
	if err != nil || dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[WARN] ReleaseService: failed to create directory: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "release_state.txt"), []byte(state), 0o644); err != nil {
		log.Printf("[WARN] ReleaseService: failed to write state file: %v", err)
	}
}

func (s *ReleaseService) LoadState() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.getReleaseDir()
	if err != nil || dir == "" {
		return s.pendingState
	}
	data, err := os.ReadFile(filepath.Join(dir, "release_state.txt"))
	if err != nil {
		return ""
	}
	s.pendingState = string(data)
	return s.pendingState
}

func (s *ReleaseService) SaveIntent(intent *domain.ReleaseIntent) {
	s.setIntent(intent)
	dir, err := s.getReleaseDir()
	if err != nil || dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[WARN] ReleaseService: failed to create directory: %v", err)
		return
	}
	data, err := json.Marshal(intent)
	if err != nil {
		log.Printf("[WARN] ReleaseService: failed to marshal intent: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "release_intent.json"), data, 0o644); err != nil {
		log.Printf("[WARN] ReleaseService: failed to write intent file: %v", err)
	}
}

func (s *ReleaseService) setIntent(intent *domain.ReleaseIntent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingIntent = intent
}

func (s *ReleaseService) LoadIntent() (*domain.ReleaseIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.getReleaseDir()
	if err != nil || dir == "" {
		if s.pendingIntent == nil {
			return nil, fmt.Errorf("no release intent")
		}
		return s.pendingIntent, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "release_intent.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no pending release. Run 'gcourer release start' first")
		}
		return nil, fmt.Errorf("failed to read release intent: %w", err)
	}
	var intent domain.ReleaseIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal release intent: %w", err)
	}
	s.pendingIntent = &intent
	return &intent, nil
}

func (s *ReleaseService) SaveChangelog(changelog string) {
	s.setChangelog(changelog)
	dir, err := s.getReleaseDir()
	if err != nil || dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[WARN] ReleaseService: failed to create directory: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "release_changelog.md"), []byte(changelog), 0o644); err != nil {
		log.Printf("[WARN] ReleaseService: failed to write changelog file: %v", err)
	}
}

func (s *ReleaseService) setChangelog(changelog string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingChangelog = changelog
}

func (s *ReleaseService) LoadChangelog() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.getReleaseDir()
	if err != nil || dir == "" {
		return s.pendingChangelog, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "release_changelog.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read changelog: %w", err)
	}
	s.pendingChangelog = string(data)
	return s.pendingChangelog, nil
}

func (s *ReleaseService) ClearPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingState = ""
	s.pendingIntent = nil
	s.pendingChangelog = ""
	dir, err := s.getReleaseDir()
	if err != nil || dir == "" {
		return
	}
	_ = os.Remove(filepath.Join(dir, "release_intent.json"))
	_ = os.Remove(filepath.Join(dir, "release_changelog.md"))
	_ = os.Remove(filepath.Join(dir, "release_state.txt"))
}

// ReleaseResult holds the outcome of a release operation.
type ReleaseResult struct {
	Operation string   `json:"operation"`
	TagName   string   `json:"tag_name,omitempty"`
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
	s.mu.Lock()
	if s.progressCb != nil {
		s.progressCb(1, 4)
	}
	s.mu.Unlock()

	// Get current releases for context
	releasesList, err := s.git.ListTags()
	if err != nil {
		releasesList = []string{}
	}

	// Parse release intent from instruction using regex (NO LLM)
	intent := parseReleaseIntent(instruction, releasesList)


	// Get commits since last tag — prefer CommitStore if available
	var commits string
	var lastTag string // track the reference tag for error messages
	var fromStore bool
	if s.commitStore != nil {
		entries, storeErr := s.commitStore.Read()
		if storeErr == nil && len(entries) > 0 {
			msgLines := domain.Messages(entries)
			commits = strings.Join(msgLines, "\n")
			fromStore = true
			log.Printf("[DEBUG] Using %d CommitStore entries for release", len(entries))
		} else if storeErr != nil {
			log.Printf("[WARN] CommitStore.Read failed: %v (falling back to git)", storeErr)
		}
	}
	if !fromStore {
		if intent.TagName != "" {
			// intent.TagName is the NEW tag to release. Use the previous tag as reference.
			prevTag := previousTag(releasesList, intent.TagName)
			if prevTag != "" {
				lastTag = prevTag
				commits, err = s.git.CommitsFromTag(prevTag)
				if err != nil {
					commits, _ = s.git.LogFull(100)
				}
			} else {
				commits, _ = s.git.LogFull(100)
			}
		} else {
			// Use latest tag
			latestTag, err := s.git.LatestTag()
			if err != nil {
				commits, _ = s.git.LogFull(100)
			} else {
				lastTag = latestTag
				commits, err = s.git.CommitsFromTag(latestTag)
				if err != nil {
					commits, _ = s.git.LogFull(100)
				}
			}
		}

		// PR enrichment: only when NOT using CommitStore entries.
		// Store entries are already enriched during the commit cycle.
		if s.githubAPI != nil {
			prNumbers := detectPRNumbers(commits)
			if len(prNumbers) > 0 {
				remoteURL, _ := s.git.RemoteURL()
				owner, repo, isGitHub, _ := resolveOwnerRepo(remoteURL)
				if isGitHub {
					ctx := context.Background()
					enriched, err := s.githubAPI.FetchPRCommits(ctx, owner, repo, prNumbers)
					if err == nil {
						enrichedStr := mergeEnrichedCommits(commits, enriched)
						if enrichedStr != "" {
							commits = enrichedStr
						}
					} else {
						log.Printf("⚠ PR enrichment failed: %v (using raw commits)", err)
					}
				}
			}
		}
	}

	if commits == "" {
		return intent, "", []string{"no commits found"}, fmt.Errorf("no new commits since last tag (%s). Cannot create empty release. Make at least one commit first.", lastTag)
	}


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
