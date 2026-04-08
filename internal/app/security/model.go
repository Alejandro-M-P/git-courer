package security

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// ParseModelSize parses a model name and returns its size category.
// Examples:
//   - "qwen3.5-14b" → ModelSizeLarge
//   - "llama3.1-70b" → ModelSizeLarge
//   - "qwen2.5-7b" → ModelSizeSmall
//   - "mistral-7b" → ModelSizeSmall
//   - "codellama-34b" → ModelSizeLarge
//
// If the model name cannot be parsed, returns ModelSizeSmall (safe default).
func ParseModelSize(modelName string) domain.ModelSize {
	// Normalize: lowercase
	lower := strings.ToLower(modelName)

	// Pattern to match: digits followed by 'b' at word boundary
	// Matches: 7b, 14b, 70b, 3b, 1b, 13b, 34b, etc.
	re := regexp.MustCompile(`(\d+)b\b`)
	match := re.FindStringSubmatch(lower)

	if len(match) < 2 {
		// Cannot parse → safe default
		return domain.ModelSizeSmall
	}

	size, err := strconv.Atoi(match[1])
	if err != nil {
		return domain.ModelSizeSmall
	}

	// >= 14B → Large (14B+ are trusted for LLM security scanning)
	if size >= 14 {
		return domain.ModelSizeLarge
	}

	// 7B-13B → Medium (not trusted, per spec)
	if size >= 7 {
		return domain.ModelSizeMedium
	}

	// < 7B → Small (safe default for very small models)
	return domain.ModelSizeSmall
}
