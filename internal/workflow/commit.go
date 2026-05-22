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

// DefaultCommitServiceConfig returns sensible defaults derived from LLM context window.
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
	annotator        ports.ChunkAnnotator
	classifier       ports.MessageClassifier
	typeHelper       ports.CommitTypeHelper
	catalogProvider  ports.CatalogProvider
	contentProvider  ports.ContentProvider
	security         ports.SecurityService
	taskLog          *taskLogger
	cfg              CommitServiceConfig
	projectCfg       *domain.ProjectConfig // nil if init hasn't run
	progress         ProgressFunc
}

// SetProgressCallback sets the callback for progress notifications.
func (s *CommitService) SetProgressCallback(fn ProgressFunc) {
	s.progress = fn
}

// SetContext sets the project context on the LLM adapter if it supports it.
func (s *CommitService) SetContext(context string) {
	if setter, ok := s.llm.(interface{ SetContext(string) }); ok {
		setter.SetContext(context)
	}
}

// SetWhy sets the user's reason for the change on the LLM adapter if it supports it.
func (s *CommitService) SetWhy(why string) {
	if setter, ok := s.llm.(interface{ SetWhy(string) }); ok {
		setter.SetWhy(why)
	}
}

// ClearWhy resets the why field on the LLM adapter if it supports it.
func (s *CommitService) ClearWhy() {
	if clearer, ok := s.llm.(interface{ ClearWhy() }); ok {
		clearer.ClearWhy()
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
	
	return &CommitService{
		projectCfg:      projectCfg,
		git:             git,
		llm:             llm,
		chunker:         chunker,
		annotator:       nil, // set via SetAnnotator or SetDependencies
		classifier:      nil, // set via SetDependencies
		typeHelper:      nil, // set via SetDependencies
		catalogProvider: nil, // set via SetDependencies
		contentProvider: contentProvider,
		security:        security,
		taskLog:         newTaskLogger(cfg.LogPath, cfg.MaxLogLines),
		cfg:             cfg,
	}
}

// SetDependencies injects the driven port dependencies that were previously
// constructed directly inside NewCommitService. This must be called after
// NewCommitService but before any workflow execution.
//
// This method exists to allow a gradual migration: callers that have not yet
// been updated to pass dependencies through the constructor can use this.
// When all callers are updated, this method will be removed and the
// dependencies will be constructor parameters.
func (s *CommitService) SetDependencies(annotator ports.ChunkAnnotator, classifier ports.MessageClassifier, typeHelper ports.CommitTypeHelper, catalogProvider ports.CatalogProvider) {
	s.annotator = annotator
	s.classifier = classifier
	s.typeHelper = typeHelper
	s.catalogProvider = catalogProvider
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
	if s.progress != nil {
		s.progress(1, 6, "Parsing diff and building AST…")
	}
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

	if s.progress != nil {
		s.progress(2, 6, "Building dependency graph…")
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
	if s.progress != nil {
		s.progress(3, 6, "Classifying chunks by type…")
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
		
		if s.annotator != nil {
			if err := s.annotator.AnnotateWithContent(chunk, fileContents, rawDiff); err != nil {
				log.Printf("[WARN] Failed to annotate chunk %d: %v", i, err)
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

// PreparePlan runs the commit preparation pipeline and generates LLM messages
// for each chunk, returning an OperationPlan for the caller to preview and approve.
// feedback is optional; if non-empty, it is passed as context to the LLM for
// message regeneration.
func (s *CommitService) PreparePlan(instruction string, feedback string) (*domain.OperationPlan, error) {
	chunks, msgs, deleted, warnings, err := s.prepareChunksAndMessages(instruction, feedback)
	if err != nil {
		return nil, err
	}

	// Build preview string
	var preview strings.Builder
	preview.WriteString(fmt.Sprintf("Commit plan: %d commit(s)\n", len(msgs)))
	for i, msg := range msgs {
		fileList := ""
		if i < len(chunks) {
			fileList = strings.Join(chunks[i].Files, ", ")
		}
		preview.WriteString(fmt.Sprintf("  %d. %s [%s]\n", i+1, msg, fileList))
	}

	plan := &domain.OperationPlan{
		Operation:    "commit",
		Messages:     msgs,
		Chunks:       DiffChunksToChunkFiles(chunks),
		DeletedFiles: deleted,
		Instruction:  instruction,
		Preview:      preview.String(),
		Reasoning:    "Changes prepared for staged diff",
	}

	if len(warnings) > 0 {
		plan.Reasoning += "\nWarnings: " + strings.Join(warnings, "; ")
	}

	return plan, nil
}

// prepareChunksAndMessages combines initial chunks if they fit in the context window.
// Otherwise, it falls back to file-by-file generation, invoking the LLM for each file and
// composing a single unified commit message.
// Returns the slice of chunks (always length 1), the slice of messages (always length 1), and warnings.
func (s *CommitService) prepareChunksAndMessages(instruction, feedback string) ([]domain.DiffChunk, []string, []string, []string, error) {
	state, err := s.prepareStages(instruction)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// 1. Calculate total annotated diff size
	totalSize := 0
	for _, chunk := range state.chunks {
		if chunk.AnnotatedDiff != "" {
			totalSize += len(chunk.AnnotatedDiff)
		} else {
			totalSize += len(chunk.Diff)
		}
	}

	var warnings []string

	if totalSize <= s.cfg.ChunkSize {
		// Happy path: combine all chunks into a single DiffChunk
		combinedChunk := s.combineChunks(state.chunks)

		// Run classification and scope resolution on the combined chunk
		s.classifyChunks([]domain.DiffChunk{combinedChunk})
		s.resolveChunkScopes([]domain.DiffChunk{combinedChunk})

		// Generate the commit message
		var msg string
		if s.llm != nil {
			msg, err = s.llm.GenerateChunkMessage(combinedChunk)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to generate message: %v", err))
				msg = formatFallbackMessage(combinedChunk, fmt.Sprintf("changes in %s", strings.Join(combinedChunk.Files, ", ")))
			}
		} else {
			msg = formatFallbackMessage(combinedChunk, fmt.Sprintf("changes in %s", strings.Join(combinedChunk.Files, ", ")))
		}

		return []domain.DiffChunk{combinedChunk}, []string{msg}, state.deleted, warnings, nil
	}

	// Fallback path: size exceeds ChunkSize. Group staged changes file-by-file.
	// Extract staged files from initial chunks
	var files []string
	seenFiles := make(map[string]bool)
	for _, chunk := range state.chunks {
		for _, f := range chunk.Files {
			if !seenFiles[f] {
				seenFiles[f] = true
				files = append(files, f)
			}
		}
	}

	var allFileChunks []domain.DiffChunk
	var fileMessages []string

	// For concurrency limit
	type fileResult struct {
		idx    int
		chunks []domain.DiffChunk
		msg    string
		warns  []string
		err    error
	}
	results := make([]fileResult, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.cfg.NumParallel)

	for i, file := range files {
		wg.Add(1)
		idx := i
		fPath := file
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 1. Get diff staged for this file
			fileDiff, err := s.git.DiffStaged(fPath)
			if err != nil {
				results[idx] = fileResult{idx: idx, err: fmt.Errorf("failed to get diff for %s: %w", fPath, err)}
				return
			}
			if fileDiff == "" {
				results[idx] = fileResult{idx: idx}
				return
			}

			// 2. Chunk this file diff
			fChunks, err := s.chunker.Chunk(fileDiff, s.cfg.ChunkSize)
			if err != nil {
				results[idx] = fileResult{idx: idx, err: fmt.Errorf("failed to chunk diff for %s: %w", fPath, err)}
				return
			}

			// 3. Annotate, classify, and resolve scope
			if err := s.annotateChunks(fChunks, fileDiff); err != nil {
				log.Printf("[WARN] Failed to annotate chunks for %s: %v", fPath, err)
			}
			s.classifyChunks(fChunks)
			s.resolveChunkScopes(fChunks)

			// 4. Generate messages for the file chunks
			var fMsg string
			var warns []string
			// Build a fallback chunk for type inference from classified fChunks
			fallbackChunk := domain.DiffChunk{Files: []string{fPath}, Diff: fileDiff}
			if len(fChunks) > 0 && fChunks[0].CommitType != "" {
				fallbackChunk.CommitType = fChunks[0].CommitType
				fallbackChunk.ConfidenceScore = fChunks[0].ConfidenceScore
			}
			if s.llm != nil {
				var msgs []string
				for _, ch := range fChunks {
					m, err := s.llm.GenerateChunkMessage(ch)
					if err != nil {
						warns = append(warns, fmt.Sprintf("file %s: %v", fPath, err))
						m = formatFallbackMessage(ch, fmt.Sprintf("changes in %s", fPath))
					}
					msgs = append(msgs, m)
				}
				fMsg = composeMessage(msgs, formatFallbackMessage(fallbackChunk, fmt.Sprintf("changes in %s", fPath)))
			} else {
				fMsg = formatFallbackMessage(fallbackChunk, fmt.Sprintf("changes in %s", fPath))
			}

			results[idx] = fileResult{
				idx:    idx,
				chunks: fChunks,
				msg:    fMsg,
				warns:  warns,
			}
		}()
	}
	wg.Wait()

	for _, res := range results {
		if res.err != nil {
			warnings = append(warnings, res.err.Error())
			continue
		}
		if len(res.chunks) > 0 {
			allFileChunks = append(allFileChunks, res.chunks...)
			if res.msg != "" {
				fileMessages = append(fileMessages, res.msg)
			}
		}
		if len(res.warns) > 0 {
			warnings = append(warnings, res.warns...)
		}
	}

	if len(allFileChunks) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("no staged file changes could be chunked")
	}

	// Combine all file-by-file chunks into a single combined DiffChunk
	combinedChunk := s.combineChunks(allFileChunks)
	var composedMsg string
	if s.llm != nil {
		var err error
		composedMsg, err = s.llm.GenerateCommitSynthesis(combinedChunk, fileMessages)
		if err != nil {
			log.Printf("[WARN] Failed to generate commit synthesis: %v", err)
			warnings = append(warnings, fmt.Sprintf("failed to generate synthesis message: %v", err))
			composedMsg = composeMessage(fileMessages, formatFallbackMessage(combinedChunk, "update staged files"))
		}
	} else {
		composedMsg = composeMessage(fileMessages, formatFallbackMessage(combinedChunk, "update staged files"))
	}

	return []domain.DiffChunk{combinedChunk}, []string{composedMsg}, state.deleted, warnings, nil
}

// combineChunks merges multiple DiffChunk objects into a single combined DiffChunk.
func (s *CommitService) combineChunks(chunks []domain.DiffChunk) domain.DiffChunk {
	if len(chunks) == 0 {
		return domain.DiffChunk{}
	}
	if len(chunks) == 1 {
		return chunks[0]
	}

	var combined domain.DiffChunk
	seenFiles := make(map[string]bool)
	var diffs []string
	var annotatedDiffs []string
	combined.GoBefore = make(map[string]string)
	combined.GoAfter = make(map[string]string)
	var branchCount, loopCount, returnCount, errorCount int
	var branchCountAfter, loopCountAfter, returnCountAfter, errorCountAfter int
	hasCFGBefore := false
	hasCFGAfter := false

	for _, chunk := range chunks {
		for _, f := range chunk.Files {
			if !seenFiles[f] {
				seenFiles[f] = true
				combined.Files = append(combined.Files, f)
			}
		}
		if chunk.Diff != "" {
			diffs = append(diffs, chunk.Diff)
		}
		if chunk.AnnotatedDiff != "" {
			annotatedDiffs = append(annotatedDiffs, chunk.AnnotatedDiff)
		}
		for k, v := range chunk.GoBefore {
			combined.GoBefore[k] = v
		}
		for k, v := range chunk.GoAfter {
			combined.GoAfter[k] = v
		}
		if chunk.CFGBefore != nil {
			hasCFGBefore = true
			branchCount += chunk.CFGBefore.Branch
			loopCount += chunk.CFGBefore.Loop
			returnCount += chunk.CFGBefore.Return
			errorCount += chunk.CFGBefore.Error
		}
		if chunk.CFGAfter != nil {
			hasCFGAfter = true
			branchCountAfter += chunk.CFGAfter.Branch
			loopCountAfter += chunk.CFGAfter.Loop
			returnCountAfter += chunk.CFGAfter.Return
			errorCountAfter += chunk.CFGAfter.Error
		}
	}

	combined.Diff = strings.Join(diffs, "\n")
	combined.AnnotatedDiff = strings.Join(annotatedDiffs, "\n")
	if hasCFGBefore {
		combined.CFGBefore = &domain.CFGCount{
			Branch: branchCount,
			Loop:   loopCount,
			Return: returnCount,
			Error:  errorCount,
		}
	}
	if hasCFGAfter {
		combined.CFGAfter = &domain.CFGCount{
			Branch: branchCountAfter,
			Loop:   loopCountAfter,
			Return: returnCountAfter,
			Error:  errorCountAfter,
		}
	}

	// Preserve best CommitType from sub-chunks using max-confidence selection
	// with weight-based tie-breaking and breaking suffix propagation.
	type bestCandidate struct {
		baseType   string
		confidence float64
		weight     int
		breaking   bool
		index      int
	}

	var best bestCandidate
	for i, chunk := range chunks {
		if chunk.CommitType == "" {
			continue
		}
		baseType := strings.TrimSuffix(chunk.CommitType, "!")
		hasBreaking := strings.HasSuffix(chunk.CommitType, "!")
		var weight int
		if s.typeHelper != nil {
			weight = s.typeHelper.CommitTypeWeight(baseType)
		} else {
			weight = domain.CommitTypeWeight(baseType)
		}

		better := false
		if best.baseType == "" {
			better = true
		} else if weight > best.weight {
			better = true
		} else if weight == best.weight && chunk.ConfidenceScore > best.confidence {
			better = true
		} else if weight == best.weight && chunk.ConfidenceScore == best.confidence && i < best.index {
			better = true
		}

		if better {
			best = bestCandidate{
				baseType:   baseType,
				confidence: chunk.ConfidenceScore,
				weight:     weight,
				breaking:   hasBreaking,
				index:      i,
			}
		}
	}

	// Check if any sub-chunk has breaking suffix (orthogonal to best type)
	anyBreaking := best.breaking
	if !anyBreaking {
		for _, chunk := range chunks {
			if strings.HasSuffix(chunk.CommitType, "!") {
				anyBreaking = true
				break
			}
		}
	}

	if best.baseType != "" {
		combined.CommitType = best.baseType
		if anyBreaking {
			combined.CommitType += "!"
		}
		combined.ConfidenceScore = best.confidence
	}

	return combined
}

// formatFallbackMessage creates a type-aware fallback commit message when LLM
// generation fails. It uses the chunk's CommitType if available, otherwise
// infers the type from diff content.
func formatFallbackMessage(chunk domain.DiffChunk, description string) string {
	commitType := chunk.CommitType
	if commitType == "" {
		commitType = domain.InferCommitType(chunk)
	}
	breaking := strings.HasSuffix(commitType, "!")
	baseType := strings.TrimSuffix(commitType, "!")
	if baseType == "" {
		baseType = "chore"
	}
	if breaking {
		return fmt.Sprintf("%s!: %s", baseType, description)
	}
	return fmt.Sprintf("%s: %s", baseType, description)
}

// composeMessage combines multiple message chunks into a single commit message.
// The LLM prompt now enforces a single clean message with structured [EL WHY PRIMERO] /
// [Y DESPUÉS ASÍ] format, so this is just a simple join for the fallback path.
func composeMessage(chunks []string, fallback string) string {
	if len(chunks) == 0 {
		return fallback
	}
	var joined []string
	for _, ch := range chunks {
		ch = strings.TrimSpace(ch)
		if ch != "" {
			joined = append(joined, ch)
		}
	}
	if len(joined) == 0 {
		return fallback
	}
	return strings.Join(joined, "\n\n")
}

// generateMessages runs parallel LLM message generation for all chunks.
func (s *CommitService) generateMessages(chunks []domain.DiffChunk, instruction, feedback string) (messages []string, warnings []string) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if s.llm == nil {
		// Fallback: return type-aware placeholder messages
		for i := range chunks {
			messages = append(messages, formatFallbackMessage(chunks[i], fmt.Sprintf("changes in %s", strings.Join(chunks[i].Files, ", "))))
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

