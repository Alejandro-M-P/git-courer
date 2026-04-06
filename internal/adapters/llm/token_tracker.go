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
// tokensUsed: actual tokens consumed by this operation
// estimatedCloudCost: what it would have cost in cloud (tokens * price)
// Price reference: ~$0.003/1K tokens for input (GPT-4o-mini), ~$0.015/1K for output
func (s *TokenStats) RecordOperation(tokensUsed int64, promptLen, responseLen int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Estimate tokens: ~4 chars per token for English, ~2.5 for Spanish
	// Being conservative: 3.5 chars/token average
	estimatedTokens := int64(float64(promptLen+responseLen) / 3.5)

	// What cloud would have cost:
	// Average $0.01/1K tokens (input+output average for GPT-4o-mini/Claude-haiku)
	// We saved 100% of this since we used local Ollama
	cloudCost := float64(estimatedTokens) / 1000 * 0.01 // cents

	s.SessionOps++
	s.SessionTokens += estimatedTokens
	s.SessionSavings += cloudCost
	s.LastOpSavings = estimatedTokens

	s.TotalOps++
	s.TotalTokens += estimatedTokens
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
