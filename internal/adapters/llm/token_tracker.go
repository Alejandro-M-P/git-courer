package ollama

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TokenStats tracks token usage for a session/project
type TokenStats struct {
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

// file path for persisted stats
func (s *TokenStats) filePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".git-courer", "stats.json")
}

// Load reads persisted stats from disk
func (s *TokenStats) Load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath())
	if err != nil {
		return // No stats file yet
	}

	var persisted struct {
		TotalOps      int     `json:"total_ops"`
		TotalTokens   int64   `json:"total_tokens"`
		TotalSavings  float64 `json:"total_savings"`
		LastResetTime string  `json:"last_reset_time"`
	}

	if err := json.Unmarshal(data, &persisted); err != nil {
		return
	}

	s.TotalOps = persisted.TotalOps
	s.TotalTokens = persisted.TotalTokens
	s.TotalSavings = persisted.TotalSavings
	if persisted.LastResetTime != "" {
		s.LastResetTime, _ = time.Parse(time.RFC3339, persisted.LastResetTime)
	}
}

// Save persists stats to disk
func (s *TokenStats) Save() {
	s.mu.Lock()
	defer s.mu.Unlock()

	os.MkdirAll(filepath.Dir(s.filePath()), 0755)

	data, _ := json.Marshal(struct {
		TotalOps      int     `json:"total_ops"`
		TotalTokens   int64   `json:"total_tokens"`
		TotalSavings  float64 `json:"total_savings"`
		LastResetTime string  `json:"last_reset_time"`
	}{
		TotalOps:      s.TotalOps,
		TotalTokens:   s.TotalTokens,
		TotalSavings:  s.TotalSavings,
		LastResetTime: s.LastResetTime.Format(time.RFC3339),
	})

	os.WriteFile(s.filePath(), data, 0644)
}

// RecordOperation records token usage for an operation
// promptTokens: tokens used to evaluate the prompt (input)
// evalTokens: tokens generated in the response (output)
// Price reference: ~$0.003/1K tokens for input (GPT-4o-mini), ~$0.015/1K for output
func (s *TokenStats) RecordOperation(totalTokens int64, promptTokens, evalTokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// What cloud would have cost for the same operation:
	// Input: $0.003/1K tokens, Output: $0.015/1K tokens
	cloudCost := (float64(promptTokens)/1000*0.003 + float64(evalTokens)/1000*0.015) * 100 // cents

	s.SessionOps++
	s.SessionTokens += totalTokens
	s.SessionSavings += cloudCost
	s.LastOpSavings = totalTokens

	s.TotalOps++
	s.TotalTokens += totalTokens
	s.TotalSavings += cloudCost

	// Async persist
	go s.Save()
}

// FormatSavings returns human-readable savings
func (s *TokenStats) FormatSavings() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lines []string
	lines = append(lines, "💰 Ahorro de tokens (git-courer vs nube)")

	// Session savings
	if s.SessionOps > 0 {
		savings := s.SessionSavings
		var savingsStr string
		if savings >= 100 {
			savingsStr = fmt.Sprintf("$%.2f", savings/100)
		} else {
			savingsStr = fmt.Sprintf("%.2f¢", savings)
		}
		lines = append(lines,
			fmt.Sprintf("   Este session: %d ops, ~%d tokens, %s", s.SessionOps, s.SessionTokens, savingsStr),
		)
	}

	// Total savings
	if s.TotalOps > 0 {
		var totalStr string
		if s.TotalSavings >= 100 {
			totalStr = fmt.Sprintf("$%.2f", s.TotalSavings/100)
		} else {
			totalStr = fmt.Sprintf("%.2f¢", s.TotalSavings)
		}
		lines = append(lines,
			fmt.Sprintf("   Histórico: %d ops, ~%d tokens, %s", s.TotalOps, s.TotalTokens, totalStr),
		)
	}

	// Last operation
	if s.LastOpSavings > 0 {
		lines = append(lines, fmt.Sprintf("   Última op: ~%d tokens", s.LastOpSavings))
	}

	if len(lines) == 1 {
		lines = append(lines, "   (sin datos aún)")
	}

	return strings.Join(lines, "\n")
}

// LastOpSavingsTokens returns tokens saved on last operation
func (s *TokenStats) LastOpSavingsTokens() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LastOpSavings
}
