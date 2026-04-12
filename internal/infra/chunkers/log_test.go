package chunkers

import (
	"testing"
)

func TestNewLogChunker(t *testing.T) {
	tests := []struct {
		name          string
		contextWindow int
		want          int
	}{
		{
			name:          "small context window uses minimum of 10",
			contextWindow: 1000,
			want:          10,
		},
		{
			name:          "medium context window calculates correctly",
			contextWindow: 4000,
			want:          20,
		},
		{
			name:          "large context window uses maximum of 25",
			contextWindow: 10000,
			want:          25,
		},
		{
			name:          "boundary at 2000 returns 10",
			contextWindow: 2000,
			want:          10,
		},
		{
			name:          "boundary at 5000 returns 25",
			contextWindow: 5000,
			want:          25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewLogChunker(tt.contextWindow)
			if chunker.commitsPerChunk != tt.want {
				t.Errorf("NewLogChunker(%d) = %d, want %d",
					tt.contextWindow, chunker.commitsPerChunk, tt.want)
			}
		})
	}
}

func TestLogChunker_Chunk(t *testing.T) {
	tests := []struct {
		name            string
		log             string
		commitsPerChunk int
		wantCount       int
		wantErr         bool
	}{
		{
			name:            "empty log returns error",
			log:             "",
			commitsPerChunk: 10,
			wantCount:       0,
			wantErr:         true,
		},
		{
			name: "single commit returns single chunk",
			log: `commit abc123
Author: Test User <test@example.com>
Date:   Mon Apr 12 10:00:00 2026 +0000

    Test commit message

    Added some code`,
			commitsPerChunk: 10,
			wantCount:       1,
			wantErr:         false,
		},
		{
			name: "multiple commits split correctly",
			log: `commit c
Author: Test <test@example.com>
Date:   Mon Apr 12 12:00:00 2026 +0000

    Commit 3

    Changes

commit b
Author: Test <test@example.com>
Date:   Mon Apr 12 11:00:00 2026 +0000

    Commit 2

    Changes

commit a
Author: Test <test@example.com>
Date:   Mon Apr 12 10:00:00 2026 +0000

    Commit 1

    Changes`,
			commitsPerChunk: 2,
			wantCount:       2,
			wantErr:         false,
		},
		{
			name: "uses default commitsPerChunk when zero",
			log: `commit a
Author: Test <test@example.com>
Date:   Mon Apr 12 10:00:00 2026 +0000

    Commit`,
			commitsPerChunk: 0,
			wantCount:       1,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewLogChunker(4000) // default 20
			chunks, err := chunker.Chunk(tt.log, tt.commitsPerChunk)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(chunks) != tt.wantCount {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.wantCount)
			}
		})
	}
}

func TestLogChunker_ChunkWithCommitsPerChunkFromStruct(t *testing.T) {
	// Test that the Chunk method uses the struct's commitsPerChunk when parameter is 0
	chunker := NewLogChunker(2000) // should set commitsPerChunk to 10

	// Create a log with more than 10 commits
	log := ""
	for i := 1; i <= 15; i++ {
		log += "commit " + string(rune('a'+i-1)) + "\n"
		log += "Author: Test <test@example.com>\n"
		log += "Date:   Mon Apr 12 10:00:00 2026 +0000\n"
		log += "\n    Commit " + string(rune('0'+i)) + "\n"
		log += "\n    Changes\n"
		log += "\n"
	}

	chunks, err := chunker.Chunk(log, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With 15 commits and 10 per chunk, should get 2 chunks
	if len(chunks) != 2 {
		t.Errorf("got %d chunks, want 2", len(chunks))
	}
}
