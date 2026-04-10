package security

import (
	"strconv"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// ParseModelSize determines the model size category from the model name.
// Uses the parameter count in the name (e.g. "qwen3.5:14b" → Large).
func ParseModelSize(modelName string) domain.ModelSize {
	lower := strings.ToLower(modelName)

	// Extract numeric size from name (e.g. "14b", "7b", "3b")
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ':' || r == '-' || r == '_' || r == '.'
	})

	for _, part := range parts {
		// Strip trailing 'b' (billions)
		if strings.HasSuffix(part, "b") {
			numStr := part[:len(part)-1]
			if n, err := strconv.ParseFloat(numStr, 64); err == nil {
				switch {
				case n >= 14:
					return domain.ModelSizeLarge
				case n >= 7:
					return domain.ModelSizeMedium
				default:
					return domain.ModelSizeSmall
				}
			}
		}
	}

	return domain.ModelSizeSmall
}
