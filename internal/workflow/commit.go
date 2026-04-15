// CommitService orchestrates the commit workflow:
// status → LLM decides what to stage → security check → chunk diff → LLM messages → git commit(s).
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

// CommitServiceConfig holds tuneable values for the commit service.
type CommitServiceConfig struct {
	BackgroundThreshold int    // diff size (chars) above which we run async
	ChunkSize           int    // max chars per diff chunk sent to LLM
	MaxLogLines         int    // circular buffer size for task.log
	LogPath             string // path to task log file
}

// DefaultCommitServiceConfig returns sensible defaults derived from Ollama context window.
func DefaultCommitServiceConfig(contextWindow, backgroundThreshold, maxLogLines int, logPath string) CommitServiceConfig {
	cw := contextWindow
	if cw == 0 {
		cw = 4096
	}
	return CommitServiceConfig{
		BackgroundThreshold: backgroundThreshold,
		ChunkSize:           cw / 2,
		MaxLogLines:         maxLogLines,
		LogPath:             logPath,
	}
}

// CommitService handles the commit workflow.
type CommitService struct {
	git      ports.Git
	llm      ports.LLM
	chunker  ports.DiffChunker
	security ports.SecurityService
	taskLog  *taskLogger
	cfg      CommitServiceConfig
}

// NewCommitService creates a new CommitService.
func NewCommitService(git ports.Git, llm ports.LLM, chunker ports.DiffChunker, security ports.SecurityService, cfg CommitServiceConfig) *CommitService {
	return &CommitService{
		git:      git,
		llm:      llm,
		chunker:  chunker,
		security: security,
		taskLog:  newTaskLogger(cfg.LogPath, cfg.MaxLogLines),
		cfg:      cfg,
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
	status, err := s.git.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

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

	allUntracked, err := s.git.ListUntracked()
	if err == nil && len(allUntracked) > 0 {
		untracked = allUntracked
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

	if decision.IncludeUntracked {
		if err := s.git.Add([]string{"."}); err != nil {
			return nil, fmt.Errorf("failed to add all files: %w", err)
		}
	} else if decision.Filter != "" {
		if err := s.git.Add([]string{decision.Filter}); err != nil {
			return nil, fmt.Errorf("failed to add files matching filter %q: %w", decision.Filter, err)
		}
	} else if len(tracked) > 0 || len(deleted) > 0 {
		filesToStage := tracked
		filesToStage = append(filesToStage, deleted...)
		if err := s.git.Add(filesToStage); err != nil {
			return nil, fmt.Errorf("failed to add files: %w", err)
		}
	}

	diff, err := s.git.DiffStaged()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged diff: %w", err)
	}
	if diff == "" {
		return nil, fmt.Errorf("nothing to commit after staging")
	}

	filesToCheck := getFilesToCommit(status, decision)
	if len(filesToCheck) > 0 {
		secResult := s.security.CheckFiles(filesToCheck, diff)
		if secResult.IsBlocked() {
			s.git.Reset("HEAD", ".")
			if first := secResult.FirstBlocking(); first != nil {
				return nil, fmt.Errorf("[SECURITY] Commit blocked: %s", first.Message)
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

	return &preparedState{chunks: chunks, deleted: deleted, decision: decision}, nil
}

// Execute runs the full commit workflow (prepare + execute).
func (s *CommitService) Execute(instruction string, preview bool) (string, error) {
	state, err := s.prepareStages(instruction)
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			resp, _ := json.Marshal(CommitResult{Operation: "commit", Message: err.Error(), Type: "write"})
			return string(resp), nil
		}
		return "", err
	}

	if len(state.chunks) > 3 || len(state.chunks)*s.cfg.ChunkSize > s.cfg.BackgroundThreshold {
		return s.executeBackground(instruction, state.chunks, state.deleted)
	}
	return s.executeSync(instruction, state.chunks, state.deleted)
}

// PrepareCommit prepares the commit without executing it.
// Returns generated messages, chunks, deleted files, warnings, reasoning, and error.
func (s *CommitService) PrepareCommit(instruction string) ([]string, []domain.DiffChunk, []string, []string, string, error) {
	state, err := s.prepareStages(instruction)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	messages := make([]string, len(state.chunks))
	var warnings []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChan := make(chan chunkResult, len(state.chunks))
	go func() {
		defer close(resultChan)
		for i, chunk := range state.chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}
			msg, err := s.llm.GenerateChunkMessage(chunk)
			select {
			case <-ctx.Done():
				return
			case resultChan <- chunkResult{chunk: chunk, message: msg, index: i, err: err}:
			}
		}
	}()

	for r := range resultChan {
		if r.err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			continue
		}
		messages[r.index] = r.message
	}

	return messages, state.chunks, state.deleted, warnings, state.decision.Reasoning, nil
}

// ExecutePrepared commits using pre-generated messages from PrepareCommit.
func (s *CommitService) ExecutePrepared(messages []string, chunks []domain.DiffChunk, instruction string) (string, error) {
	s.taskLog.logStart()
	var committed []string
	var warnings []string

	// Stage all to get proper diff calculation
	if err := s.git.Add([]string{"."}); err != nil {
		return "", fmt.Errorf("failed to stage files: %w", err)
	}

	// Unstage everything but keep working tree changes
	if _, err := s.git.Reset("HEAD", "."); err != nil {
		return "", fmt.Errorf("failed to reset staging: %w", err)
	}

	for i, chunk := range chunks {
		if messages[i] == "" || messages[i] == "chore: no meaningful changes" {
			continue
		}
		if err := s.git.Add(chunk.Files); err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d stage skipped: %v", i+1, err))
			continue
		}
		if _, err := s.git.Commit(messages[i]); err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d commit skipped: %v", i+1, err))
			continue
		}
		committed = append(committed, messages[i])
		s.taskLog.logCommit(messages[i])
		s.taskLog.logProgress(len(committed), len(chunks))
	}

	if len(committed) == 0 {
		return "", fmt.Errorf("no commits were generated")
	}

	if strings.Contains(strings.ToLower(instruction), "push") {
		pushResult, err := s.git.Push()
		if err != nil {
			return "", fmt.Errorf("push failed: %w", err)
		}
		s.taskLog.logPush(pushResult)
	}

	s.taskLog.logDone(len(committed))
	resp, _ := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	return string(resp), nil
}

// ExecuteFromPlan commits using pre-approved messages and per-chunk file lists from the plan.
// chunkFiles[i] contains the files to stage for messages[i]. If chunkFiles is nil or shorter
// than messages, remaining messages are committed with whatever is currently staged.
// deletedFiles contains files with status "D " to commit separately at the end.
func (s *CommitService) ExecuteFromPlan(messages []string, chunkFiles [][]string, deletedFiles []string, instruction string) (string, error) {
	s.taskLog.logStart()
	var committed []string
	var warnings []string

	for i, msg := range messages {
		if msg == "" || msg == "chore: no meaningful changes" {
			continue
		}
		if i < len(chunkFiles) && len(chunkFiles[i]) > 0 {
			if err := s.git.Add(chunkFiles[i]); err != nil {
				warnings = append(warnings, fmt.Sprintf("Chunk %d stage failed: %v", i+1, err))
				continue
			}
		}
		result, err := s.git.Commit(msg)
		s.taskLog.logError(fmt.Sprintf("Commit %d result: %q, err: %v", i+1, result, err))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Commit %d failed: %v", i+1, err))
			continue
		}
		committed = append(committed, msg)
		s.taskLog.logCommit(msg)
		s.taskLog.logProgress(len(committed), len(messages))
	}

	if strings.Contains(strings.ToLower(instruction), "push") {
		pushResult, err := s.git.Push()
		if err != nil {
			return "", fmt.Errorf("push failed: %w", err)
		}
		s.taskLog.logPush(pushResult)
	}

	if len(deletedFiles) > 0 {
		if err := s.git.Add(deletedFiles); err != nil {
			warnings = append(warnings, fmt.Sprintf("deleted files stage failed: %v", err))
		} else {
			msg := "chore: remove " + strings.Join(deletedFiles, ", ")
			if _, err := s.git.Commit(msg); err != nil {
				warnings = append(warnings, fmt.Sprintf("deleted files commit failed: %v", err))
			} else {
				committed = append(committed, msg)
				s.taskLog.logCommit(msg)
			}
		}
	}

	if len(committed) == 0 {
		return "", fmt.Errorf("no commits were generated")
	}

	s.taskLog.logDone(len(committed))
	resp, _ := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	return string(resp), nil
}

func (s *CommitService) executeSync(instruction string, chunks []domain.DiffChunk, deleted []string) (string, error) {
	s.taskLog.logStart()
	var committed []string
	var warnings []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChan := make(chan chunkResult, len(chunks))
	go func() {
		defer close(resultChan)
		for i, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}
			msg, err := s.llm.GenerateChunkMessage(chunk)
			select {
			case <-ctx.Done():
				return
			case resultChan <- chunkResult{chunk: chunk, message: msg, index: i, err: err}:
			}
		}
	}()

	for r := range resultChan {
		if r.err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
			continue
		}
		if err := s.git.Add(r.chunk.Files); err != nil {
			s.rollback(committed)
			return "", fmt.Errorf("failed to stage chunk %d: %w", r.index+1, err)
		}
		if _, err := s.git.Commit(r.message); err != nil {
			s.rollback(committed)
			return "", fmt.Errorf("failed commit %d: %w", r.index+1, err)
		}
		committed = append(committed, r.message)
		s.taskLog.logCommit(r.message)
		s.taskLog.logProgress(len(committed), len(chunks))
	}

	if strings.Contains(strings.ToLower(instruction), "push") {
		pushResult, err := s.git.Push()
		if err != nil {
			return "", fmt.Errorf("push failed: %w", err)
		}
		s.taskLog.logPush(pushResult)
	}

	if len(deleted) > 0 {
		if err := s.git.Add(deleted); err != nil {
			warnings = append(warnings, fmt.Sprintf("deleted files stage failed: %v", err))
		} else {
			msg := "chore: remove " + strings.Join(deleted, ", ")
			if _, err := s.git.Commit(msg); err != nil {
				warnings = append(warnings, fmt.Sprintf("deleted files commit failed: %v", err))
			} else {
				committed = append(committed, msg)
				s.taskLog.logCommit(msg)
			}
		}
	}

	if len(committed) == 0 {
		return "", fmt.Errorf("no commits were generated")
	}

	s.taskLog.logDone(len(committed))
	resp, _ := json.Marshal(CommitResult{Operation: "commit", Commits: committed, Warnings: warnings, Type: "write"})
	return string(resp), nil
}

func (s *CommitService) executeBackground(instruction string, chunks []domain.DiffChunk, deleted []string) (string, error) {
	s.taskLog.logStart()
	s.taskLog.logProgress(0, len(chunks))
	shouldPush := strings.Contains(strings.ToLower(instruction), "push")

	go func() {
		var committed []string
		resultChan := make(chan chunkResult, len(chunks))
		go func() {
			for i, chunk := range chunks {
				msg, err := s.llm.GenerateChunkMessage(chunk)
				resultChan <- chunkResult{chunk: chunk, message: msg, index: i, err: err}
			}
			close(resultChan)
		}()

		for r := range resultChan {
			if r.err != nil {
				s.taskLog.logError(fmt.Sprintf("Chunk %d failed: %v", r.index+1, r.err))
				continue
			}
			if len(r.chunk.Files) == 0 {
				continue
			}
			if err := s.git.Add(r.chunk.Files); err != nil {
				s.taskLog.logError(fmt.Sprintf("failed to stage chunk %d: %v", r.index+1, err))
				continue
			}
			if _, err := s.git.Commit(r.message); err != nil {
				s.taskLog.logError(fmt.Sprintf("failed commit %d: %v", r.index+1, err))
				continue
			}
			committed = append(committed, r.message)
			s.taskLog.logCommit(r.message)
			s.taskLog.logProgress(len(committed), len(chunks))
		}

		if len(committed) > 0 && shouldPush {
			pushResult, err := s.git.Push()
			if err != nil {
				s.taskLog.logError(fmt.Sprintf("push failed: %v", err))
			} else {
				s.taskLog.logPush(pushResult)
			}
		}
		s.taskLog.logDone(len(committed))
	}()

	resp, _ := json.Marshal(map[string]any{
		"operation": "commit", "type": "write", "state": "running",
		"message": fmt.Sprintf("Processing %d chunks in background. Check %q for progress.", len(chunks), s.cfg.LogPath),
	})
	return string(resp), nil
}

func (s *CommitService) rollback(committed []string) {
	for i := range committed {
		if _, err := s.git.Reset("--soft", "HEAD~1"); err != nil {
			s.taskLog.logError(fmt.Sprintf("rollback failed at step %d: %v", i+1, err))
			return
		}
	}
	s.git.Reset("HEAD", ".")
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

// --- Task logger (circular buffer) ---

type taskLogger struct {
	logPath     string
	maxLogLines int
}

func newTaskLogger(logPath string, maxLogLines int) *taskLogger {
	os.MkdirAll(filepath.Dir(logPath), 0755)
	return &taskLogger{logPath: logPath, maxLogLines: maxLogLines}
}

func (l *taskLogger) log(entryType, message string) {
	entry := fmt.Sprintf("%s [%s] %s", time.Now().Format("15:04:05"), entryType, message)
	lines, _ := l.readLines()
	lines = append(lines, entry)
	if len(lines) > l.maxLogLines {
		lines = lines[len(lines)-l.maxLogLines:]
	}
	l.writeLines(lines)
}

func (l *taskLogger) readLines() ([]string, error) {
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

func (l *taskLogger) writeLines(lines []string) {
	os.WriteFile(l.logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func (l *taskLogger) logStart()            { l.log("START", "commit task began") }
func (l *taskLogger) logCommit(msg string) { l.log("COMMIT", msg) }
func (l *taskLogger) logProgress(done, total int) {
	l.log("PROGRESS", fmt.Sprintf("%d/%d commits", done, total))
}
func (l *taskLogger) logPush(target string) { l.log("PUSH", target) }
func (l *taskLogger) logError(msg string)   { l.log("ERROR", msg) }
func (l *taskLogger) logDone(total int)     { l.log("DONE", fmt.Sprintf("%d commits completed", total)) }
