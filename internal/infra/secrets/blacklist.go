package secrets

import (
	"path/filepath"
	"strings"
)

// FolderBlacklist contains directory names that should never be committed.
var FolderBlacklist = []string{
	"node_modules",
	".git",
	"vendor",
	"__pycache__",
	".venv",
	"dist",
	"build",
	"bin",
	"out",
	".env",
}

// NameBlacklist contains file patterns/names that should never be committed.
var NameBlacklist = []string{
	".env",
	".env.local",
	".env.production",
	".pem",
	".key",
	".p12",
	".pkcs8",
	".keystore",
	"credentials.json",
	"credentials.yaml",
	"secrets.yaml",
	"secrets.yml",
	"id_rsa",
	"id_ecdsa",
	"id_ed25519",
	"git-courer", // The binary itself
}

// IsBlacklistedFolder checks if any component of the path is in the folder blacklist.
// Example: "vendor/foo/bar" → true (vendor is blacklisted)
// Example: "src/main.go" → false
func IsBlacklistedFolder(path string) bool {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")

	for _, part := range parts {
		for _, blacklisted := range FolderBlacklist {
			if part == blacklisted {
				return true
			}
		}
	}
	return false
}

// IsBlacklistedName checks if the filename matches the name blacklist.
func IsBlacklistedName(name string) bool {
	lower := strings.ToLower(name)

	// Exception: .env.example and .env.template are allowed (template files, not real secrets)
	if lower == ".env.example" || lower == ".env.template" {
		return false
	}

	// Check exact matches
	exactMatches := []string{
		".env", ".env.local", ".env.production",
		".pem", ".key", ".p12", ".pkcs8", ".keystore",
		"credentials.json", "credentials.yaml",
		"secrets.yaml", "secrets.yml",
		"id_rsa", "id_ecdsa", "id_ed25519",
		"git-courer",
	}

	for _, blacklisted := range exactMatches {
		if lower == blacklisted {
			return true
		}
	}

	// Check extensions
	extensions := []string{".pem", ".key", ".p12", ".pkcs8", ".keystore"}
	for _, ext := range extensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	// Check prefixes
	prefixes := []string{"id_rsa", "id_ecdsa", "id_ed25519", "credentials", "secrets"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}

// ExtractFilename extracts just the filename from a full path.
func ExtractFilename(path string) string {
	return filepath.Base(path)
}
