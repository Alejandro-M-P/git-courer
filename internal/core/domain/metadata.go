package domain

import (
	"os"
	"path/filepath"
	"strings"
)

// MetadataDir defines the location of git-courer metadata files.
const MetadataDir = ".git/git-courer"

// IsMetadataPath returns true if the given path is the metadata directory or located within it.
func IsMetadataPath(path string) bool {
	return path == MetadataDir || strings.HasPrefix(path, MetadataDir+"/")
}

// ResolveMetadataDir resolves the git-courer metadata directory location, supporting standard
// repositories and linked Git worktrees.
func ResolveMetadataDir(workDir string) string {
	gitPath := filepath.Join(workDir, ".git")

	info, err := os.Stat(gitPath)
	if err == nil && !info.IsDir() {
		data, err := os.ReadFile(gitPath)
		if err == nil {
			content := strings.TrimSpace(string(data))
			if strings.HasPrefix(content, "gitdir: ") {
				gitDir := strings.TrimPrefix(content, "gitdir: ")
				gitDir = strings.TrimSpace(gitDir)
				if gitDir != "" {
					if !filepath.IsAbs(gitDir) {
						gitDir = filepath.Join(workDir, gitDir)
					}

					commonDirFile := filepath.Join(gitDir, "commondir")
					commonData, err := os.ReadFile(commonDirFile)
					if err == nil {
						commonDir := strings.TrimSpace(string(commonData))
						if commonDir != "" {
							if !filepath.IsAbs(commonDir) {
								commonDir = filepath.Join(gitDir, commonDir)
							}
							return filepath.Clean(filepath.Join(commonDir, "git-courer"))
						}
					}
					return filepath.Clean(filepath.Join(gitDir, "git-courer"))
				}
			}
		}
	}

	return filepath.Clean(filepath.Join(workDir, ".git", "git-courer"))
}
