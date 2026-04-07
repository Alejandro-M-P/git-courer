// Package commit provides the commit use case: analyze changes,
// generate commit messages via LLM, and execute commits with rollback on failure.
// Uses a pipeline pattern for maximum speed with large diffs.
package commit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

const (
	MaxLogLines = 500
	LogPath     = ".gcourer/task.log"
	// If total diff is larger than this, run in background
	BackgroundThreshold = 10000 // chars
)

// TaskLogger writes git operations to a circular buffer log file.
// Only the last MaxLogLines are kept.
type TaskLogger struct {
	logPath string
}

// NewTaskLogger creates a TaskLogger that writes to .gcourer/task.log
func NewTaskLogger() *TaskLogger {
	// Ensure directory exists
	dir := filepath.Dir(LogPath)
	os.MkdirAll(dir, 0755)
	return &TaskLogger{logPath: LogPath}
}

// log writes an entry to the log file with circular buffer behavior
func (l *TaskLogger) log(entryType, message string) {
	entry := fmt.Sprintf("%s [%s] %s", time.Now().Format("15:04:05"), entryType, message)

	// Read existing lines
	lines, _ := l.readLines()

	// Add new entry
	lines = append(lines, entry)

	// Keep only last MaxLogLines
	if len(lines) > MaxLogLines {
		lines = lines[len(lines)-MaxLogLines:]
	}

	// Write back
	l.writeLines(lines)
}

func (l *TaskLogger) readLines() ([]string, error) {
	data, err := os.ReadFile(l.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Remove empty last line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

func (l *TaskLogger) writeLines(lines []string) {
	content := strings.Join(lines, "\n") + "\n"
	os.WriteFile(l.logPath, []byte(content), 0644)
}

// LogStart logs task start
func (l *TaskLogger) LogStart() {
	l.log("START", "commit task began")
}

// LogCommit logs a commit
func (l *TaskLogger) LogCommit(commitMsg string) {
	l.log("COMMIT", commitMsg)
}

// LogBranch logs a branch operation
func (l *TaskLogger) LogBranch(branchName string) {
	l.log("BRANCH", branchName)
}

// LogPush logs a push operation
func (l *TaskLogger) LogPush(target string) {
	l.log("PUSH", target)
}

// LogProgress logs progress update
func (l *TaskLogger) LogProgress(done, total int) {
	l.log("PROGRESS", fmt.Sprintf("%d/%d commits", done, total))
}

// LogError logs an error
func (l *TaskLogger) LogError(errMsg string) {
	l.log("ERROR", errMsg)
}

// LogDone logs task completion
func (l *TaskLogger) LogDone(totalCommits int) {
	l.log("DONE", fmt.Sprintf("%d commits completed", totalCommits))
}

// Service handles the commit workflow using a pipeline architecture.
type Service struct {
	git      ports.Git
	llm      ports.LLM
	chunker  ports.DiffChunker
	security ports.SecurityService
	taskLog  *TaskLogger
}

// NewService creates a new commit service.
func NewService(git ports.Git, llm ports.LLM, chunker ports.DiffChunker, security ports.SecurityService) *Service {
	return &Service{
		git:      git,
		llm:      llm,
		chunker:  chunker,
		security: security,
		taskLog:  NewTaskLogger(),
	}
}

// Result holds the outcome of a commit operation.
type Result struct {
	Operation string                `json:"operation"`
	Message   string                `json:"result,omitempty"`
	Commits   []string              `json:"commits,omitempty"`
	Excluded  []domain.ExcludedFile `json:"excluded,omitempty"`
	Warnings  []string              `json:"warnings,omitempty"`
	Type      string                `json:"type"`
	Tokens    int                   `json:"tokens"`
	Push      string                `json:"push,omitempty"`
}

// chunkResult holds the result of processing a single chunk through LLM.
type chunkResult struct {
	chunk   domain.DiffChunk
	message string
	index   int
	err     error
}

// formatStatus creates a string summary of git status for the LLM.
func formatStatus(status domain.Status) string {
	var b strings.Builder
	for _, f := range status.Files {
		b.WriteString(fmt.Sprintf("%s: %s\n", f.Status, f.Path))
	}
	return b.String()
}

// getFilesToCommit returns the list of files that will be committed based on decision.
func getFilesToCommit(status domain.Status, decision domain.CommitIntent) []string {
	var files []string
	seen := make(map[string]bool)

	for _, f := range status.Files {
		// Skip already seen files
		if seen[f.Path] {
			continue
		}
		seen[f.Path] = true

		switch f.Status {
		case "??":
			// Untracked file
			if decision.IncludeUntracked {
				files = append(files, f.Path)
			}
		case "D":
			// Deleted file - always include
			files = append(files, f.Path)
		default:
			// Modified or renamed file
			if decision.Filter != "" {
				// Filter pattern is set - include all tracked files
				// (the actual filtering was done by git add with the pattern)
				files = append(files, f.Path)
			} else {
				// No filter - include tracked files
				files = append(files, f.Path)
			}
		}
	}

	return files
}

// Execute runs the commit workflow. For large diffs, it runs in background.
// Returns immediately with "processing in background" message.
func (s *Service) Execute(instruction string, preview bool) (string, error) {
	// === STAGE 1: Get git status ===
	status, err := s.git.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	// Separate files by type
	var tracked, untracked, deleted []string
	for _, f := range status.Files {
		switch f.Status {
		case "??":
			untracked = append(untracked, f.Path)
		case "D":
			deleted = append(deleted, f.Path)
		default:
			tracked = append(tracked, f.Path)
		}
	}

	// === STAGE 2: Ask LLM what to do ===
	gitStatus := formatStatus(status)
	decision, err := s.llm.DecideCommit(instruction, gitStatus,
		strings.Join(untracked, "\n"),
		strings.Join(tracked, "\n"),
		strings.Join(deleted, "\n"))
	if err != nil {
		return "", fmt.Errorf("failed to get LLM decision: %w", err)
	}

	// === STAGE 3: Stage files based on decision ===
	if decision.IncludeUntracked {
		// Add all files (tracked + untracked)
		if err := s.git.Add([]string{"."}); err != nil {
			return "", fmt.Errorf("failed to add all files: %w", err)
		}
	} else if decision.Filter != "" {
		// Add files matching the filter pattern
		if err := s.git.Add([]string{decision.Filter}); err != nil {
			return "", fmt.Errorf("failed to add files matching filter %q: %w", decision.Filter, err)
		}
	} else {
		// Default: add only tracked files (modified/deleted)
		if len(tracked) > 0 {
			if err := s.git.Add(tracked); err != nil {
				return "", fmt.Errorf("failed to add tracked files: %w", err)
			}
		}
	}

	// Get the diff of what was staged
	diff, _ := s.git.DiffStaged()
	if diff == "" {
		resp, _ := json.Marshal(Result{
			Operation: "commit",
			Message:   "Nothing to commit after staging",
			Type:      "write",
			Tokens:    0,
		})
		return string(resp), nil
	}

	// === STAGE 4: Security check before commit ===
	// Get the list of files that will be committed
	filesToCheck := getFilesToCommit(status, decision)
	if len(filesToCheck) > 0 {
		secResult := s.security.CheckFiles(filesToCheck, diff)
		if secResult.IsBlocked() {
			// Unstage all files to clean up
			s.git.Reset("HEAD", ".")
			firstBlocking := secResult.FirstBlocking()
			if firstBlocking != nil {
				return "", fmt.Errorf("[SECURITY] Commit blocked: %s", firstBlocking.Message)
			}
			return "", fmt.Errorf("[SECURITY] Commit blocked: potential secret detected")
		}
	}

	// Produce chunks using the injected chunker
	chunks, err := s.chunker.Chunk(diff, 4000)
	if err != nil {
		return "", fmt.Errorf("failed to chunk diff: %w", err)
	}

	if len(chunks) == 0 {
		resp, _ := json.Marshal(Result{
			Operation: "commit",
			Message:   "Nothing to commit",
			Type:      "write",
			Tokens:    0,
		})
		return string(resp), nil
	}

	// Decide: background or sync?
	// If many chunks, run in background to avoid MCP timeout
	shouldRunBackground := len(chunks) > 3 || len(diff) > BackgroundThreshold

	if shouldRunBackground {
		return s.executeBackground(instruction, chunks)
	}

	return s.executeSync(instruction, chunks)
}

// executeSync runs the pipeline synchronously (for small diffs)
func (s *Service) executeSync(instruction string, chunks []domain.DiffChunk) (string, error) {
	s.taskLog.LogStart()

	var committed []string
	var warnings []string

	// Channel for LLM results
	resultChan := make(chan chunkResult, len(chunks))

	// LLM Worker goroutine
	go func() {
		for i, chunk := range chunks {
			message, err := s.llm.GenerateChunkMessage(chunk)
			resultChan <- chunkResult{chunk: chunk, message: message, index: i, err: err}
		}
		close(resultChan)
	}()

	// Committer
	for result := range resultChan {
		if result.err != nil {
			warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", result.index+1, result.err))
			s.taskLog.LogError(fmt.Sprintf("Chunk %d failed: %v", result.index+1, result.err))
			continue
		}

		if err := s.git.Add(result.chunk.Files); err != nil {
			s.rollback(committed)
			return "", fmt.Errorf("failed to stage chunk %d: %w", result.index+1, err)
		}

		if _, err := s.git.Commit(result.message); err != nil {
			s.rollback(committed)
			return "", fmt.Errorf("failed commit %d: %w", result.index+1, err)
		}

		committed = append(committed, result.message)
		s.taskLog.LogCommit(result.message)
		s.taskLog.LogProgress(len(committed), len(chunks))
	}

	if len(committed) == 0 {
		s.taskLog.LogError("No commits were generated")
		return "", fmt.Errorf("⚠️ No commits were generated")
	}

	if strings.Contains(strings.ToLower(instruction), "push") {
		pushResult, pushErr := s.git.Push()
		if pushErr != nil {
			s.taskLog.LogError(fmt.Sprintf("push failed: %v", pushErr))
			return "", fmt.Errorf("push failed: %w", pushErr)
		}
		s.taskLog.LogPush(pushResult)
	}

	s.taskLog.LogDone(len(committed))

	resp := Result{
		Operation: "commit",
		Commits:   committed,
		Warnings:  warnings,
		Type:      "write",
	}
	respBytes, _ := json.Marshal(resp)
	return string(respBytes), nil
}

// executeBackground runs the pipeline in a goroutine and returns immediately.
// The goroutine updates the log file as it progresses.
func (s *Service) executeBackground(instruction string, chunks []domain.DiffChunk) (string, error) {
	s.taskLog.LogStart()
	s.taskLog.LogProgress(0, len(chunks))

	// Copy instruction for the goroutine
	shouldPush := strings.Contains(strings.ToLower(instruction), "push")

	// Run pipeline in background
	go func() {
		var committed []string
		var warnings []string

		resultChan := make(chan chunkResult, len(chunks))

		// LLM Worker
		go func() {
			for i, chunk := range chunks {
				message, err := s.llm.GenerateChunkMessage(chunk)
				resultChan <- chunkResult{chunk: chunk, message: message, index: i, err: err}
			}
			close(resultChan)
		}()

		// Committer
		for result := range resultChan {
			if result.err != nil {
				warnings = append(warnings, fmt.Sprintf("Chunk %d failed: %v", result.index+1, result.err))
				s.taskLog.LogError(fmt.Sprintf("Chunk %d failed: %v", result.index+1, result.err))
				continue
			}

			// Skip chunks with empty file lists
			if len(result.chunk.Files) == 0 {
				s.taskLog.LogError(fmt.Sprintf("Chunk %d has no files, skipping", result.index+1))
				continue
			}

			if err := s.git.Add(result.chunk.Files); err != nil {
				s.taskLog.LogError(fmt.Sprintf("failed to stage chunk %d: %v", result.index+1, err))
				continue // Don't rollback, just skip this chunk
			}

			if _, err := s.git.Commit(result.message); err != nil {
				s.taskLog.LogError(fmt.Sprintf("failed commit %d: %v", result.index+1, err))
				continue // Don't rollback, just skip this chunk
			}

			committed = append(committed, result.message)
			s.taskLog.LogCommit(result.message)
			s.taskLog.LogProgress(len(committed), len(chunks))
		}

		if len(committed) > 0 && shouldPush {
			pushResult, pushErr := s.git.Push()
			if pushErr != nil {
				s.taskLog.LogError(fmt.Sprintf("push failed: %v", pushErr))
			} else {
				s.taskLog.LogPush(pushResult)
			}
		}

		s.taskLog.LogDone(len(committed))
	}()

	// Return immediately while background runs
	resp := map[string]any{
		"operation": "commit",
		"type":      "write",
		"state":     "running",
		"message":   fmt.Sprintf("⚡ Procesando %d chunks en segundo plano. Consultá '.gcourer/task.log' para ver progreso.", len(chunks)),
	}
	respBytes, _ := json.Marshal(resp)
	return string(respBytes), nil
}

func (s *Service) rollback(committed []string) {
	for range committed {
		s.git.Reset("--soft", "HEAD~1")
	}
	s.git.Reset("HEAD", ".")
}
