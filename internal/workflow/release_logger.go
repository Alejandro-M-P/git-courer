package workflow

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
	if err := os.WriteFile(l.logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		log.Printf("releaseLogger: failed to write log file %s: %v", l.logPath, err)
	}
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
