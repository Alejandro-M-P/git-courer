package workflow

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type taskLogger struct {
	mu          sync.Mutex
	logPath     string
	maxLogLines int
}

func newTaskLogger(logPath string, maxLogLines int) *taskLogger {
	os.MkdirAll(filepath.Dir(logPath), 0755)
	return &taskLogger{logPath: logPath, maxLogLines: maxLogLines}
}

func (l *taskLogger) log(entryType, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
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
	if err := os.WriteFile(l.logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		log.Printf("taskLogger: failed to write log file %s: %v", l.logPath, err)
	}
}

func (l *taskLogger) logStart()            { l.log("START", "commit task began") }
func (l *taskLogger) logCommit(msg string) { l.log("COMMIT", msg) }
func (l *taskLogger) logProgress(done, total int) {
	l.log("PROGRESS", fmt.Sprintf("%d/%d commits", done, total))
}
func (l *taskLogger) logPush(target string) { l.log("PUSH", target) }
func (l *taskLogger) logError(msg string)   { l.log("ERROR", msg) }
func (l *taskLogger) logDone(total int)     { l.log("DONE", fmt.Sprintf("%d commits completed", total)) }
