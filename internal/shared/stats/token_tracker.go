// Package stats tracks token usage and calculates cloud cost savings.
// Shows the user how much they've saved by using local AI vs cloud.
package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Tracker tracks token usage for a session/project.
type Tracker struct {
	mu sync.Mutex

	// Session stats (reset on restart)
	SessionOps     int
	SessionTokens  int64
	SessionSavings float64 // cents (USD)
	LastOpSavings  int64   // tokens saved on last operation

	// Historical stats (persisted)
	TotalOps      int
	TotalTokens   int64
	TotalSavings  float64 // cents (USD)
	LastResetTime time.Time
}

// Load reads persisted stats from disk.
func Load() *Tracker {
	t := &Tracker{}
	data, err := os.ReadFile(t.filePath())
	if err != nil {
		return t // No stats file yet
	}

	var persisted struct {
		TotalOps      int     `json:"total_ops"`
		TotalTokens   int64   `json:"total_tokens"`
		TotalSavings  float64 `json:"total_savings"`
		LastResetTime string  `json:"last_reset_time"`
	}

	if err := json.Unmarshal(data, &persisted); err != nil {
		return t
	}

	t.TotalOps = persisted.TotalOps
	t.TotalTokens = persisted.TotalTokens
	t.TotalSavings = persisted.TotalSavings
	if persisted.LastResetTime != "" {
		t.LastResetTime, _ = time.Parse(time.RFC3339, persisted.LastResetTime)
	}
	return t
}

// Save persists stats to disk.
func (t *Tracker) Save() {
	t.mu.Lock()
	defer t.mu.Unlock()

	os.MkdirAll(filepath.Dir(t.filePath()), 0755)

	data, _ := json.Marshal(struct {
		TotalOps      int     `json:"total_ops"`
		TotalTokens   int64   `json:"total_tokens"`
		TotalSavings  float64 `json:"total_savings"`
		LastResetTime string  `json:"last_reset_time"`
	}{
		TotalOps:      t.TotalOps,
		TotalTokens:   t.TotalTokens,
		TotalSavings:  t.TotalSavings,
		LastResetTime: t.LastResetTime.Format(time.RFC3339),
	})

	os.WriteFile(t.filePath(), data, 0644)
}

// RecordOperation records token usage for an operation.
// promptTokens: tokens used to evaluate the prompt (input)
// evalTokens: tokens generated in the response (output)
// Price reference: ~$0.003/1K tokens for input (GPT-4o-mini), ~$0.015/1K for output
func (t *Tracker) RecordOperation(totalTokens int64, promptTokens, evalTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// What cloud would have cost for the same operation:
	// Input: $0.003/1K tokens, Output: $0.015/1K tokens
	cloudCost := (float64(promptTokens)/1000*0.003 + float64(evalTokens)/1000*0.015) * 100 // cents

	t.SessionOps++
	t.SessionTokens += totalTokens
	t.SessionSavings += cloudCost
	t.LastOpSavings = totalTokens

	t.TotalOps++
	t.TotalTokens += totalTokens
	t.TotalSavings += cloudCost

	// Async persist
	go t.Save()
}

// FormatSavings returns human-readable savings.
func (t *Tracker) FormatSavings() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var lines []string
	lines = append(lines, "💰 Ahorro de tokens (git-courer vs nube)")

	// Session savings
	if t.SessionOps > 0 {
		savings := t.SessionSavings
		var savingsStr string
		if savings >= 100 {
			savingsStr = fmt.Sprintf("$%.2f", savings/100)
		} else {
			savingsStr = fmt.Sprintf("%.2f¢", savings)
		}
		lines = append(lines,
			fmt.Sprintf("   Este session: %d ops, ~%d tokens, %s", t.SessionOps, t.SessionTokens, savingsStr),
		)
	}

	// Total savings
	if t.TotalOps > 0 {
		var totalStr string
		if t.TotalSavings >= 100 {
			totalStr = fmt.Sprintf("$%.2f", t.TotalSavings/100)
		} else {
			totalStr = fmt.Sprintf("%.2f¢", t.TotalSavings)
		}
		lines = append(lines,
			fmt.Sprintf("   Histórico: %d ops, ~%d tokens, %s", t.TotalOps, t.TotalTokens, totalStr),
		)
	}

	// Last operation
	if t.LastOpSavings > 0 {
		lines = append(lines, fmt.Sprintf("   Última op: ~%d tokens", t.LastOpSavings))
	}

	if len(lines) == 1 {
		lines = append(lines, "   (sin datos aún)")
	}

	return strings.Join(lines, "\n")
}

// LastOpSavingsTokens returns tokens saved on last operation.
func (t *Tracker) LastOpSavingsTokens() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.LastOpSavings
}

func (t *Tracker) filePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".git-courer", "stats.json")
}
