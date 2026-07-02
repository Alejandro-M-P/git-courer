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

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
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
	customMessage    string                // user instructions for changelog generation
	mu               sync.Mutex
	pendingState     string
	pendingIntent    *domain.ReleaseIntent
	pendingChangelog string
	pendingEntries   []domain.CommitEntry // stored by Prepare for stack grouping
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

// SetCustomMessage stores optional user instructions for changelog generation.
// The message is injected into the LLM prompt to guide tone, focus, or content.
func (s *ReleaseService) SetCustomMessage(msg string) {
	s.customMessage = msg
}

// NewReleaseService creates a new ReleaseService.
// githubAPI is optional — pass nil to disable PR enrichment.
// commitStore is optional — pass nil to disable commit metadata read/clear.
func NewReleaseService(git ports.Git, llm ports.LLM, logChunker LogChunker, cfg ReleaseServiceConfig, githubAPI ports.GitHubAPI, commitStore ports.CommitStore) *ReleaseService {
	cfg.NumParallel = 1

	var projectCfg *domain.ProjectConfig
	if cfg.Context != "" {
		if setter, ok := llm.(interface{ SetContext(string) }); ok {
			setter.SetContext(cfg.Context)
		}
	} else {
		workDir := cfg.WorkDir
		if workDir == "" {
			workDir = "."
		}
		if loaded, err := domain.LoadProjectConfig(workDir); err == nil && loaded != nil {
			projectCfg = loaded
			// FormatScopeContext now returns only description (no areas)
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

// migrationEntry matches the legacy JSON serialization of CommitEntry for metadata migration.
type migrationEntry struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// migrateOldMetadata merges branch entries from oldDir into newDir, deduplicating by SHA.
func migrateOldMetadata(oldDir, newDir string) error {
	oldBranches, err := os.ReadDir(oldDir)
	if err != nil {
		return err
	}
	for _, entry := range oldBranches {
		if !entry.IsDir() {
			continue
		}
		branchName := entry.Name()
		oldPath := filepath.Join(oldDir, branchName, "commits.json")
		newPath := filepath.Join(newDir, branchName, "commits.json")

		oldData, oldErr := os.ReadFile(oldPath)
		if oldErr != nil {
			if os.IsNotExist(oldErr) {
				continue
			}
			log.Printf("[WARN] migrate: cannot read %s: %v", oldPath, oldErr)
			continue
		}

		var oldEntries []migrationEntry
		if parseErr := json.Unmarshal(oldData, &oldEntries); parseErr != nil {
			log.Printf("[WARN] migrate: cannot parse %s: %v", oldPath, parseErr)
			continue
		}

		if mkErr := os.MkdirAll(filepath.Dir(newPath), 0o755); mkErr != nil {
			log.Printf("[WARN] migrate: cannot create %s: %v", filepath.Dir(newPath), mkErr)
			continue
		}

		newData, newErr := os.ReadFile(newPath)
		if os.IsNotExist(newErr) {
			if wErr := os.WriteFile(newPath, oldData, 0o644); wErr != nil {
				log.Printf("[WARN] migrate: cannot write %s: %v", newPath, wErr)
			}
			continue
		}
		if newErr != nil {
			log.Printf("[WARN] migrate: cannot read %s: %v", newPath, newErr)
			continue
		}

		var newEntries []migrationEntry
		if parseErr := json.Unmarshal(newData, &newEntries); parseErr != nil {
			log.Printf("[WARN] migrate: cannot parse %s: %v", newPath, parseErr)
			continue
		}

		seen := make(map[string]bool, len(newEntries))
		for _, je := range newEntries {
			seen[je.SHA] = true
		}
		for _, je := range oldEntries {
			if !seen[je.SHA] {
				newEntries = append(newEntries, je)
				seen[je.SHA] = true
			}
		}

		mergedData, marshalErr := json.MarshalIndent(newEntries, "", "  ")
		if marshalErr != nil {
			log.Printf("[WARN] migrate: cannot marshal %s: %v", branchName, marshalErr)
			continue
		}
		mergedData = append(mergedData, '\n')
		if wErr := os.WriteFile(newPath, mergedData, 0o644); wErr != nil {
			log.Printf("[WARN] migrate: cannot write %s: %v", newPath, wErr)
		}
	}
	return nil
}

// commitsFromRefs reads commit entries from refs/courer/* blobs.
// Returns empty string and nil error if no refs exist.
func (s *ReleaseService) commitsFromRefs() (string, error) {
	if s.git == nil {
		return "", nil
	}

	if _, fetchErr := s.git.Fetch(); fetchErr != nil {
		log.Printf("[WARN] release refs: fetch failed: %v", fetchErr)
	}

	refsRaw, err := s.git.ShowRef("refs/courer/*")
	if err != nil {
		return "", fmt.Errorf("show refs: %w", err)
	}
	if refsRaw == "" {
		return "", nil
	}

	seen := make(map[string]bool)
	var entries []domain.CommitEntry
	for _, line := range strings.Split(strings.TrimSpace(refsRaw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]

		blobContent, catErr := s.git.CatFile(ref, "")
		if catErr != nil {
			log.Printf("[WARN] release refs: cat-file %s failed: %v", ref, catErr)
			continue
		}
		var jEntries []struct {
			SHA     string `json:"sha"`
			Message string `json:"message"`
			Author  string `json:"author"`
			Date    string `json:"date"`
		}
		if parseErr := json.Unmarshal([]byte(blobContent), &jEntries); parseErr != nil {
			log.Printf("[WARN] release refs: parse blob %s failed: %v", ref, parseErr)
			continue
		}
		for _, je := range jEntries {
			if seen[je.SHA] {
				continue
			}
			entry, newErr := domain.NewCommitEntry(je.SHA, je.Message,
				domain.WithAuthor(je.Author),
				domain.WithDate(je.Date),
			)
			if newErr != nil {
				log.Printf("[WARN] release refs: invalid entry %s: %v", je.SHA, newErr)
				continue
			}
			entries = append(entries, entry)
			seen[je.SHA] = true
		}
	}

	if len(entries) == 0 {
		return "", nil
	}

	s.pendingEntries = entries
	msgLines := domain.Messages(entries)
	return strings.Join(msgLines, "\n"), nil
}

func (s *ReleaseService) getReleaseDir() (string, error) {
	if s.cfg.WorkDir == "" {
		return "", nil // Fallback to in-memory mode
	}
	return domain.ResolveMetadataDir(s.cfg.WorkDir), nil
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

	// Get commits since last tag — prefer refs/courer/*, then CommitStore, then git log
	var commits string
	var lastTag string // track the reference tag for error messages
	var fromStore bool

	// Migration: merge legacy .git-courer/branches/ into .git/git-courer/branches/ dedup by SHA
	if s.cfg.WorkDir != "" {
		oldBranchesDir := filepath.Join(s.cfg.WorkDir, ".git-courer", "branches")
		newBranchesDir := filepath.Join(domain.ResolveMetadataDir(s.cfg.WorkDir), "branches")
		if _, statErr := os.Stat(oldBranchesDir); statErr == nil {
			log.Printf("[INFO] ReleaseService: migrating metadata from %s to %s", oldBranchesDir, newBranchesDir)
			if mergeErr := migrateOldMetadata(oldBranchesDir, newBranchesDir); mergeErr != nil {
				log.Printf("[WARN] ReleaseService: metadata migration failed: %v (continuing)", mergeErr)
			} else {
				if removeErr := os.RemoveAll(oldBranchesDir); removeErr != nil {
					log.Printf("[WARN] ReleaseService: failed to remove old metadata: %v", removeErr)
				}
			}
		}
	}

	// Try refs/courer/* first (survives squash merge)
	if s.git != nil {
		if refCommits, err := s.commitsFromRefs(); err == nil && refCommits != "" {
			fromStore = true
			commits = refCommits
			log.Printf("[DEBUG] Using refs/courer/* entries for release")
		} else if err != nil {
			log.Printf("[WARN] Refs/courer/* read failed: %v (falling back to CommitStore)", err)
		}
	}

	if s.commitStore != nil && !fromStore {
		// Merge entries from branches/ (DEPRECATED) and workspace/ (new).
		// Deduplicate by SHA across both sources so the LLM sees each commit once.
		seen := make(map[string]bool)
		var deduped []domain.CommitEntry

		// 1. Read legacy branches/ stores (DEPRECATED — passive coexistence).
		branchEntries, allBranchesErr := s.commitStore.ReadAllBranches()
		if allBranchesErr == nil {
			for _, entries := range branchEntries {
				for _, entry := range entries {
					if !seen[entry.SHA()] {
						seen[entry.SHA()] = true
						deduped = append(deduped, entry)
					}
				}
			}
		} else {
			log.Printf("[WARN] ReadAllBranches failed: %v (continuing with workspace entries)", allBranchesErr)
		}

		// 2. Read workspace/ stores (new — session-keyed).
		workspaceEntries, wsErr := s.commitStore.ReadAllWorkspaces()
		if wsErr == nil {
			for _, entries := range workspaceEntries {
				for _, entry := range entries {
					if !seen[entry.SHA()] {
						seen[entry.SHA()] = true
						deduped = append(deduped, entry)
					}
				}
			}
		} else {
			log.Printf("[WARN] ReadAllWorkspaces failed: %v (continuing with branch entries)", wsErr)
		}

		if len(deduped) > 0 {
			msgLines := domain.Messages(deduped)
			commits = strings.Join(msgLines, "\n")
			fromStore = true
			s.pendingEntries = deduped // Store for stack grouping in Generate()
			log.Printf("[DEBUG] Using %d deduplicated CommitStore entries from branches + workspaces for release", len(deduped))
		}

		// If ReadAllBranches returned empty or wasn't available, fall back to Read (single branch)
		if !fromStore {
			entries, storeErr := s.commitStore.Read()
			if storeErr == nil && len(entries) > 0 {
				msgLines := domain.Messages(entries)
				commits = strings.Join(msgLines, "\n")
				fromStore = true
				s.pendingEntries = entries // Store for stack grouping in Generate()
				log.Printf("[DEBUG] Using %d CommitStore entries for release", len(entries))
			} else if storeErr != nil {
				log.Printf("[WARN] CommitStore.Read failed: %v (falling back to git)", storeErr)
			}
		}
	}
	if !fromStore {
		// Bug 4: Clear pendingEntries before git-history fallback to prevent stale entries from previous release
		s.pendingEntries = nil
		
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
