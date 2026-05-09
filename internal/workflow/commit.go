// CommitService orchestrates the commit workflow:
// status → LLM decides what to stage → security check → chunk diff → LLM messages → git commit(s).
package workflow

import (
	"fmt"
	"log"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	gitadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/infra/classifier"
)

// CommitServiceConfig holds tuneable values for the commit service.
type CommitServiceConfig struct {
	ChunkSize   int    // max chars per diff chunk sent to LLM
	MaxLogLines int    // circular buffer size for task.log
	LogPath     string // path to task log file
	NumParallel int    // max concurrent LLM calls (default: 1 = serial)
	Context     string // optional project context for prompt injection
}

// DefaultCommitServiceConfig returns sensible defaults derived from Ollama context window.
func DefaultCommitServiceConfig(contextWindow, maxLogLines int, logPath string) CommitServiceConfig {
	cw := contextWindow
	if cw == 0 {
		cw = 4096
	}
	chunkSize := cw / 2
	if chunkSize > 6000 {
		chunkSize = 6000
	}
	return CommitServiceConfig{
		ChunkSize:   chunkSize,
		MaxLogLines: maxLogLines,
		LogPath:     logPath,
		NumParallel: 1,
	}
}

// CommitService handles the commit workflow.
type CommitService struct {
	git              ports.Git
	llm              ports.LLM
	chunker          ports.DiffChunker
	annotator        ports.ChunkAnnotator
	classifier       ports.MessageClassifier
	contentProvider  ports.ContentProvider
	security         ports.SecurityService
	taskLog          *taskLogger
	cfg              CommitServiceConfig
	projectCfg       *domain.ProjectConfig // nil if init hasn't run
}

// SetContext sets the project context on the LLM adapter if it supports it.
func (s *CommitService) SetContext(context string) {
	if setter, ok := s.llm.(interface{ SetContext(string) }); ok {
		setter.SetContext(context)
	}
}

// NewCommitService creates a new CommitService.
func NewCommitService(git ports.Git, llm ports.LLM, chunker ports.DiffChunker, security ports.SecurityService, cfg CommitServiceConfig) *CommitService {
	if cfg.NumParallel <= 0 {
		cfg.NumParallel = 1
	}

	// Load project config: inject scope context into LLM and store for per-chunk scope resolution.
	var projectCfg *domain.ProjectConfig
	if cfg.Context == "" {
		if loaded, err := domain.LoadProjectConfig("."); err == nil && loaded != nil {
			projectCfg = loaded
			if scopeCtx := loaded.FormatScopeContext(); scopeCtx != "" {
				if setter, ok := llm.(interface{ SetContext(string) }); ok {
					setter.SetContext(scopeCtx)
				}
			}
		}
	} else {
		if setter, ok := llm.(interface{ SetContext(string) }); ok {
			setter.SetContext(cfg.Context)
		}
	}
	
	contentProvider := gitadapter.NewGitContentProvider(".")
	annotator := chunkers.NewASTAnnotator()
	msgClassifier := classifier.NewClassifier(git)

	return &CommitService{
		projectCfg:      projectCfg,
		git:             git,
		llm:             llm,
		chunker:         chunker,
		annotator:       annotator,
		classifier:      msgClassifier,
		contentProvider: contentProvider,
		security:        security,
		taskLog:         newTaskLogger(cfg.LogPath, cfg.MaxLogLines),
		cfg:             cfg,
	}
}

// CommitResult holds the outcome of a commit operation.
type CommitResult struct {
	Operation string                `json:"operation"`
	Message   string                `json:"result,omitempty"`
	Commits   []string              `json:"commits,omitempty"`
	Excluded  []domain.ExcludedFile `json:"excluded,omitempty"`
	Warnings  []string              `json:"warnings,omitempty"`
	Type      string                `json:"type"`
}

type chunkResult struct {
	chunk   domain.DiffChunk
	message string
	index   int
	err     error
}

type preparedState struct {
	chunks   []domain.DiffChunk
	deleted  []string
	decision domain.CommitIntent
}

// prepareStages runs the shared preparation pipeline (stages files, checks security, chunks diff).
func (s *CommitService) prepareStages(instruction string) (*preparedState, error) {
	log.Printf("[DEBUG] prepareStages: starting for instruction: %s", instruction)
	status, err := s.git.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	log.Printf("[DEBUG] prepareStages: status has %d files", len(status.Files))

	var tracked, untracked, deleted []string
	for _, f := range status.Files {
		switch f.Status {
		case "??":
			untracked = append(untracked, f.Path)
		case "D ":
			deleted = append(deleted, f.Path)
		default:
			tracked = append(tracked, f.Path)
		}
	}

	log.Printf("[DEBUG] prepareStages: tracked=%d, untracked=%d, deleted=%d", len(tracked), len(untracked), len(deleted))

	allUntracked, err := s.git.ListUntracked()
	if err == nil && len(allUntracked) > 0 {
		untracked = allUntracked
		log.Printf("[DEBUG] prepareStages: using allUntracked: %d files", len(allUntracked))
	}

	decision, err := s.llm.DecideCommit(
		instruction,
		formatCommitStatus(status),
		strings.Join(untracked, "\n"),
		strings.Join(tracked, "\n"),
		strings.Join(deleted, "\n"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM decision: %w", err)
	}
	log.Printf("[DEBUG] prepareStages: LLM decision - includeUntracked=%v, filter=%q", decision.IncludeUntracked, decision.Filter)

	if decision.IncludeUntracked {
		log.Printf("[DEBUG] prepareStages: staging all files (includeUntracked=true)")
		if err := s.git.Add([]string{"."}); err != nil {
			return nil, fmt.Errorf("failed to add all files: %w", err)
		}
	} else if len(decision.Filter) > 0 {
		log.Printf("[DEBUG] prepareStages: staging filtered: %v", decision.Filter)
		if err := s.git.Add(decision.Filter); err != nil {
			return nil, fmt.Errorf("failed to add files matching filter %v: %w", decision.Filter, err)
		}
	} else if len(tracked) > 0 || len(deleted) > 0 {
		log.Printf("[DEBUG] prepareStages: staging tracked+deleted: %d files", len(tracked)+len(deleted))
		filesToStage := tracked
		filesToStage = append(filesToStage, deleted...)
		if err := s.git.Add(filesToStage); err != nil {
			return nil, fmt.Errorf("failed to add files: %w", err)
		}
	} else {
		log.Printf("[DEBUG] prepareStages: NO FILES TO STAGE - tracked=%d, deleted=%d", len(tracked), len(deleted))
	}

	diff, err := s.git.DiffStaged()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged diff: %w", err)
	}
	log.Printf("[DEBUG] prepareStages: diff length=%d", len(diff))
	if diff == "" {
		return nil, fmt.Errorf("nothing to commit after staging")
	}

	filesToCheck := getFilesToCommit(status, decision)
	if len(filesToCheck) > 0 {
		// Get a fresh diff only for the files we are about to commit
		targetedDiff, err := s.git.DiffStaged(filesToCheck...)
		if err != nil {
			targetedDiff = diff // fallback to full diff if target fails
		}

		secResult := s.security.CheckFiles(filesToCheck, targetedDiff)
		if secResult.IsBlocked() {
			s.git.Reset("HEAD", ".")
			if first := secResult.FirstBlocking(); first != nil {
				return nil, fmt.Errorf("[SECURITY] Commit blocked: %s (in %s)", first.Message, first.File)
			}
			return nil, fmt.Errorf("[SECURITY] Commit blocked: potential secret detected")
		}
	}

	chunks, err := s.chunker.Chunk(diff, s.cfg.ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk diff: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("nothing to commit")
	}

	if err := s.annotateChunks(chunks); err != nil {
		log.Printf("[WARN] Failed to annotate chunks: %v", err)
	}

	s.classifyChunks(chunks)
	s.resolveChunkScopes(chunks)

	return &preparedState{chunks: chunks, deleted: deleted, decision: decision}, nil
}

// resolveChunkScopes assigns the functional area scope to each chunk using the project config.
func (s *CommitService) resolveChunkScopes(chunks []domain.DiffChunk) {
	if s.projectCfg == nil || len(s.projectCfg.Areas) == 0 {
		return
	}
	for i := range chunks {
		chunks[i].Scope = s.projectCfg.ResolveScope(chunks[i].Files)
	}
}

// classifyChunks runs the message classifier on each annotated chunk. Results
// with low confidence are logged but do not block the workflow — the LLM stage
// will determine the final commit type for ambiguous chunks.
func (s *CommitService) classifyChunks(chunks []domain.DiffChunk) {
	if s.classifier == nil {
		return
	}
	for i := range chunks {
		commitType, confidence := s.classifier.Classify(&chunks[i])
		if commitType != "" && confidence < 0.70 {
			log.Printf("[DEBUG] classifier: chunk %d low confidence %.2f for type %q", i, confidence, commitType)
		}
	}
}

// annotateChunks enriches diff chunks with AST-based semantic labels (function/type changes).
// It uses the content provider to retrieve before/after file contents and the annotator
// to analyze AST changes and populate chunk.AnnotatedDiff.
func (s *CommitService) annotateChunks(chunks []domain.DiffChunk) error {
	for i := range chunks {
		chunk := &chunks[i]
		
		// Skip empty chunks
		if len(chunk.Files) == 0 {
			continue
		}
		
		// Get file contents for this chunk
		fileContents, err := s.contentProvider.GetContents(chunk.Files)
		if err != nil {
			log.Printf("[WARN] Failed to get contents for chunk %d: %v", i, err)
			continue // Skip failed chunks, continue with others
		}
		
		// For each file in the chunk, call the annotator
		for _, fc := range fileContents {
			if err := s.annotator.Annotate(chunk, fc.Before, fc.After); err != nil {
				log.Printf("[WARN] Failed to annotate file %s in chunk %d: %v", fc.Filename, i, err)
				// Continue with other files - non-fatal error
			}
		}
	}
	return nil
}

func formatCommitStatus(status domain.Status) string {
	var b strings.Builder
	for _, f := range status.Files {
		b.WriteString(fmt.Sprintf("%s: %s\n", f.Status, f.Path))
	}
	return b.String()
}

func getFilesToCommit(status domain.Status, decision domain.CommitIntent) []string {
	var files []string
	seen := make(map[string]bool)
	for _, f := range status.Files {
		if seen[f.Path] {
			continue
		}

		// Apply filter if present - check against all filter patterns
		if len(decision.Filter) > 0 {
			matched := false
			for _, filter := range decision.Filter {
				if strings.Contains(f.Path, filter) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		seen[f.Path] = true
		if f.Status == "??" {
			if decision.IncludeUntracked {
				files = append(files, f.Path)
			}
		} else {
			files = append(files, f.Path)
		}
	}
	return files
}

// DiffChunksToChunkFiles converts domain.DiffChunk slices to per-message file lists for storage in OperationPlan.
// Each chunk's Files becomes a []string entry in the resulting slice.
// If chunks is empty, returns nil.
func DiffChunksToChunkFiles(chunks []domain.DiffChunk) [][]string {
	if len(chunks) == 0 {
		return nil
	}
	result := make([][]string, len(chunks))
	for i, chunk := range chunks {
		result[i] = chunk.Files
	}
	return result
}
