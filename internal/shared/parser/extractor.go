// Package parser provides string extraction utilities for parsing natural language
// git instructions and extracting structured parameters.
package parser

import "strings"

// ExtractBranchName extracts a branch name from a natural language instruction.
// Supports bilingual prefixes (EN/ES).
func ExtractBranchName(instruction string) string {
	lower := strings.ToLower(instruction)

	prefixes := []string{
		"create branch ",
		"new branch ",
		"checkout to ",
		"checkout ",
		"switch to ",
		"switch ",
		"make branch ",
		"crea branch ",
		"crear branch ",
		"rama ",
		"branch ",
	}

	var afterPrefix string
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			afterPrefix = strings.TrimSpace(lower[len(prefix):])
			break
		}
	}

	if afterPrefix == "" {
		if idx := strings.Index(lower, "called "); idx >= 0 {
			afterPrefix = strings.TrimSpace(lower[idx+7:])
		} else if idx := strings.Index(lower, "llamado "); idx >= 0 {
			afterPrefix = strings.TrimSpace(lower[idx+8:])
		} else {
			afterPrefix = lower
		}
	}

	afterPrefix = strings.Trim(afterPrefix, "\"'")

	if len(afterPrefix) > 100 || strings.ContainsAny(afterPrefix, "!@#$%^&*()=+[]{}|\\:;<>?") {
		return ""
	}

	return afterPrefix
}

// ExtractResetTarget extracts the reset target (commit/branch) from an instruction.
// Defaults to "HEAD~1" if no target is found.
func ExtractResetTarget(instruction string) string {
	lower := strings.ToLower(instruction)
	parts := strings.Split(lower, "reset")
	if len(parts) < 2 {
		return "HEAD~1"
	}
	target := strings.TrimSpace(parts[1])
	target = strings.Trim(target, "\"")
	if target == "" {
		return "HEAD~1"
	}
	return target
}

// ExtractCommitHash extracts a commit hash from instructions like "cherry-pick abc1234".
func ExtractCommitHash(instruction string) string {
	lower := strings.ToLower(instruction)
	patterns := []string{"cherry-pick ", "revert "}
	for _, pattern := range patterns {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			hash := strings.TrimSpace(lower[idx+len(pattern):])
			hash = strings.Trim(hash, "\"")
			if len(hash) >= 4 && len(hash) <= 40 {
				return hash
			}
		}
	}
	parts := strings.Fields(instruction)
	for _, part := range parts {
		part = strings.Trim(part, "\"")
		if len(part) >= 4 && len(part) <= 40 {
			isHex := true
			for _, c := range part {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					isHex = false
					break
				}
			}
			if isHex {
				return part
			}
		}
	}
	return ""
}

// ExtractTagName extracts a tag name from an instruction.
func ExtractTagName(instruction string) string {
	lower := strings.ToLower(instruction)
	patterns := []string{"tag ", "create tag ", "new tag ", "make tag "}
	for _, pattern := range patterns {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			tag := strings.TrimSpace(lower[idx+len(pattern):])
			tag = strings.Trim(tag, "\"")
			if tag != "" && len(tag) <= 100 {
				return tag
			}
		}
	}
	return ""
}

// ExtractFileName extracts a file path from an instruction (e.g., "blame main.go").
func ExtractFileName(instruction string) string {
	lower := strings.ToLower(instruction)
	if idx := strings.Index(lower, "blame"); idx >= 0 {
		file := strings.TrimSpace(lower[idx+5:])
		file = strings.Trim(file, "\"")
		if file != "" {
			return file
		}
	}
	words := strings.Fields(instruction)
	for _, word := range words {
		word = strings.Trim(word, "\"")
		if strings.Contains(word, ".") && !strings.Contains(word, " ") {
			return word
		}
	}
	return ""
}

// ExtractFilesToAdd extracts file paths to stage from an instruction.
// Returns ["."] if no specific files are mentioned or if "all" / "-A" / "--all" is used.
func ExtractFilesToAdd(instruction string) []string {
	lower := strings.ToLower(instruction)
	var files []string
	patterns := []string{"add ", "stage "}
	for _, pattern := range patterns {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			filesStr := strings.TrimSpace(lower[idx+len(pattern):])
			filesStr = strings.Trim(filesStr, "\"")

			// Handle "add -A", "add --all", "add all", "add all changes"
			if filesStr == "-a" || filesStr == "--all" || filesStr == "all" ||
				strings.HasPrefix(filesStr, "all ") || strings.HasPrefix(filesStr, "-a ") {
				return []string{"."}
			}

			parts := strings.Fields(filesStr)
			for _, f := range parts {
				f = strings.Trim(f, "\"")
				if f != "" && f != "." && f != "-a" && f != "--all" {
					files = append(files, f)
				}
			}
		}
	}
	if len(files) == 0 {
		files = append(files, ".")
	}
	return files
}
