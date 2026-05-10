package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecAdapter_StashShow(t *testing.T) {
	t.Run("returns stash diff output", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		dir := t.TempDir()
		initGitRepo(t, dir)
		adapter := New(dir)

		// Create an initial commit
		os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)
		adapter.Add([]string{"file.txt"})
		adapter.Commit("initial")

		// Modify and stash
		os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)
		adapter.Add([]string{"file.txt"})
		adapter.Stash("test-stash")

		// StashShow should return a non-empty string
		output, err := adapter.StashShow()
		assert.NoError(t, err)
		assert.NotEmpty(t, output, "StashShow should return non-empty output when stash exists")
	})

	t.Run("returns error when no stash exists", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		dir := t.TempDir()
		initGitRepo(t, dir)
		adapter := New(dir)

		// Create an initial commit (no stash)
		os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)
		adapter.Add([]string{"file.txt"})
		adapter.Commit("initial")

		_, err := adapter.StashShow()
		assert.Error(t, err, "StashShow should return error when no stash exists")
	})
}