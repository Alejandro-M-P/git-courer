// CommitService orchestrates the commit workflow:
// status → LLM decides what to stage → security check → chunk diff → LLM messages → git commit(s).
package workflow

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	gitadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/infra/classifier"
)

// CommitServiceConfig holds tuneable values for the commit service.
type CommitServiceConfig struct {
	ChunkSize       int    // max chars per diff chunk sent to LLM
	MaxLogLines     int    // circular buffer size for task.log
	LogPath         string // path to task log file
	NumParallel     int    // max concurrent LLM calls (default: 1 = serial)
	Context         string // optional project context for prompt injection
	ContentProvider ports.ContentProvider
}

// DefaultCommitServiceConfig returns sensible defaults derived from Ollama context window.
func DefaultCommitServiceConfig(contextWindow, maxLogLines int, logPath string) CommitServiceConfig {
	cw := contextWindow
	if cw == 0 {
		cw = 4096
	}
	// Each raw chunk becomes 2-3x bigger after annotation (labels + diff lines).
	// Divide by 4 to leave room for prompt template + output tokens.
	chunkSize := cw / 4
	if chunkSize > 4000 {
		chunkSize = 4000
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
	unifiedPass      *chunkers.UnifiedASTPass
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
	
	contentProvider := cfg.ContentProvider
	if contentProvider == nil {
		contentProvider = gitadapter.NewGitContentProvider(".")
	}
	
	// Get the language catalog from the chunker and pass it to classifier
	var catalog *chunkers.LanguageCatalog
	if concreteChunker, ok := chunker.(*chunkers.DiffChunker); ok {
		catalog = concreteChunker.GetLanguageCatalog()
	}
	msgClassifier := classifier.NewClassifierWithCatalog(git, catalog, classifier.WithBinaryClassifier(llm))

	return &CommitService{
		projectCfg:      projectCfg,
		git:             git,
		llm:             llm,
		chunker:         chunker,
		unifiedPass:     chunkers.NewUnifiedASTPass(catalog),
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

// prepareStages runs the shared preparation pipeline (checks security, chunks diff).
// Automatic staging has been removed. The caller is responsible for staging files.
func (s *CommitService) prepareStages(instruction string) (*preparedState, error) {
	log.Printf("[DEBUG] prepareStages: starting for instruction: %s", instruction)
	status, err := s.git.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	diff, err := s.git.DiffStaged()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged diff: %w", err)
	}
	log.Printf("[DEBUG] prepareStages: diff length=%d", len(diff))
	if diff == "" {
		return nil, fmt.Errorf("nothing staged to commit. Use git_write command=ADD first.")
	}

	// We still need to know which files are staged for security and chunking
	var stagedFiles []string
	var deleted []string
	for _, f := range status.Files {
		if f.Staged {
			stagedFiles = append(stagedFiles, f.Path)
			if f.Status == "D " || f.Status == "D" {
				deleted = append(deleted, f.Path)
			}
		}
	}

	if len(stagedFiles) > 0 {
		secResult := s.security.CheckFiles(stagedFiles, diff)
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

	if err := s.annotateChunks(chunks, diff); err != nil {
		log.Printf("[WARN] Failed to annotate chunks: %v", err)
	}

	s.classifyChunks(chunks)
	s.resolveChunkScopes(chunks)

	// Decision is now empty/identity as the agent already decided by staging
	decision := domain.CommitIntent{IncludeUntracked: false, Filter: stagedFiles}

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
		
		// Binary LLM delegation: when confidence < 0.95 after symmetry+heuristics,
		// delegate to LLM for precise fix/refactor classification
		if (commitType == "fix" || commitType == "refactor") && confidence < 0.95 {
			if s.llm != nil {
				// Extract the chunk diff for LLM classification
				chunkDiff := chunks[i].Diff
				if chunkDiff == "" {
					chunkDiff = "No diff content available"
				}
				
				// Prepare the binary classification prompt
				prompt := "" +
					"Diff:\n" + chunkDiff + "\n\n" +
					"Context: A 'fix' corrects incorrect behavior or addresses a bug. " +
					"A 'refactor' improves code structure without changing behavior."
				
				// Delegate to LLM for binary classification using the dedicated method
				llmResponse, err := s.llm.ClassifyBinary(prompt)
				if err == nil && llmResponse != "" {
					// Normalize response to lower case and trim whitespace
					normalized := strings.ToLower(strings.TrimSpace(llmResponse))
					if normalized == "fix" || normalized == "refactor" {
						chunks[i].CommitType = normalized
						chunks[i].ConfidenceScore = 0.97 // High confidence for LLM binary classification
						log.Printf("[DEBUG] LLM binary classification: chunk %d classified as %q with confidence 0.97", i, normalized)
						continue // Skip the low confidence log for this case
					}
				}
			}
		}
		
		if commitType != "" && confidence < 0.70 {
			log.Printf("[DEBUG] classifier: chunk %d low confidence %.2f for type %q", i, confidence, commitType)
		}
	}
}

// annotateChunks enriches diff chunks with AST-based semantic labels (function/type changes).
// It uses the content provider to retrieve before/after file contents and the annotator
// to analyze AST changes and populate chunk.AnnotatedDiff.
// It also populates chunk.GoBefore/GoAfter for Go files to enable AST identity detection.
func (s *CommitService) annotateChunks(chunks []domain.DiffChunk, rawDiff string) error {
	for i := range chunks {
		chunk := &chunks[i]
		
		if len(chunk.Files) == 0 {
			continue
		}
		
		fileContents, err := s.contentProvider.GetContents(chunk.Files)
		if err != nil {
			log.Printf("[WARN] Failed to get contents for chunk %d: %v", i, err)
			continue
		}
		
		for _, fc := range fileContents {
			labels, cfgDiff, err := s.unifiedPass.ProcessWithContent(fc.Filename, fc.Before, fc.After, nil)
			if err != nil {
				log.Printf("[WARN] Failed to annotate file %s in chunk %d: %v", fc.Filename, i, err)
			}
			
			for _, l := range labels {
				if chunk.AnnotatedDiff != "" {
					chunk.AnnotatedDiff += "\n"
				}
				breaking := ""
				if l.Breaking {
					breaking = " ⚠ BREAKING"
				}
				chunk.AnnotatedDiff += fmt.Sprintf("📄 %s\n%s [%s%s] %s:%d\n", l.File, l.Name, l.Type, breaking, l.File, l.Line)
			}

			// Populate CFG metadata from annotator when non-zero
			if cfgDiff.Before != (domain.CFGCount{}) || cfgDiff.After != (domain.CFGCount{}) {
				chunk.CFGBefore = &cfgDiff.Before
				chunk.CFGAfter = &cfgDiff.After
			}

			if strings.HasSuffix(fc.Filename, ".go") {
				if chunk.GoBefore == nil {
					chunk.GoBefore = make(map[string]string)
				}
				if chunk.GoAfter == nil {
					chunk.GoAfter = make(map[string]string)
				}
				if len(fc.Before) > 0 {
					chunk.GoBefore[fc.Filename] = string(fc.Before)
				}
				if len(fc.After) > 0 {
					chunk.GoAfter[fc.Filename] = string(fc.After)
				}
			}
		}

		chunkers.MergeDiffIntoAnnotations(chunk, rawDiff)
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

// PreparePlan runs the commit preparation pipeline and generates LLM messages
// for each chunk, returning an OperationPlan for the caller to preview and approve.
// feedback is optional; if non-empty, it is passed as context to the LLM for
// message regeneration.
func (s *CommitService) PreparePlan(instruction string, feedback string) (*domain.OperationPlan, error) {
	state, err := s.prepareStages(instruction)
	if err != nil {
		return nil, err
	}

	// Generate LLM messages in parallel
	msgs, warnings := s.generateMessages(state.chunks, instruction, feedback)

	// Build preview string
	var preview strings.Builder
	preview.WriteString(fmt.Sprintf("Commit plan: %d commit(s)\n", len(msgs)))
	for i, msg := range msgs {
		fileList := ""
		if i < len(state.chunks) {
			fileList = strings.Join(state.chunks[i].Files, ", ")
		}
		preview.WriteString(fmt.Sprintf("  %d. %s [%s]\n", i+1, msg, fileList))
	}

	plan := &domain.OperationPlan{
		Operation:   "commit",
		Messages:    msgs,
		Chunks:      DiffChunksToChunkFiles(state.chunks),
		DeletedFiles: state.deleted,
		Instruction: instruction,
		Preview:     preview.String(),
		Reasoning:   "Changes prepared for staged diff",
	}

	if len(warnings) > 0 {
		plan.Reasoning += "\nWarnings: " + strings.Join(warnings, "; ")
	}

	return plan, nil
}

// generateMessages runs parallel LLM message generation for all chunks.
func (s *CommitService) generateMessages(chunks []domain.DiffChunk, instruction, feedback string) (messages []string, warnings []string) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if s.llm == nil {
		// Fallback: return placeholder messages
		for i := range chunks {
			messages = append(messages, fmt.Sprintf("chore: changes in %s", strings.Join(chunks[i].Files, ", ")))
		}
		return messages, nil
	}

	type result struct {
		idx int
		msg string
		err error
	}

	results := make([]result, len(chunks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.cfg.NumParallel)

	var mu sync.Mutex
	for i, chunk := range chunks {
		wg.Add(1)
		idx := i
		ch := chunk
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			msg, err := s.llm.GenerateChunkMessage(ch)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, fmt.Sprintf("chunk %d: %v", idx+1, err))
				mu.Unlock()
			}
			results[idx] = result{idx: idx, msg: msg, err: err}
		}()
	}
	wg.Wait()

	// Collect in index order
	for _, r := range results {
		if r.err == nil && r.msg != "" && r.msg != "chore: no meaningful changes" {
			messages = append(messages, r.msg)
		}
	}
	return messages, warnings
}
