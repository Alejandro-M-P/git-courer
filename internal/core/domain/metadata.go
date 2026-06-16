package domain

import (
	"strings"
)

// MetadataDir defines the location of git-courer metadata files.
const MetadataDir = ".git/git-courer"

// IsMetadataPath returns true if the given path is the metadata directory or located within it.
func IsMetadataPath(path string) bool {
	return path == MetadataDir || strings.HasPrefix(path, MetadataDir+"/")
}
