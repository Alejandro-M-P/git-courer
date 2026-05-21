JSON only.

## Directory tree
{{.DirectoryTree}}

## Task
Map this tree into 4-6 functional areas. Group directories that serve the same purpose.

## Rules
- Use ONLY paths that appear verbatim above
- NEVER invent paths
- 4-6 areas maximum — not one per directory
- Group: internal/core/ + internal/delivery/ + internal/infra/ → one area
- Group: cmd/ + scripts/ → cli
- Group: test/ + test/e2e/ → tests

## Output
{"areas": {"cli": ["cmd/", "scripts/"], "core": ["internal/core/", "internal/workflow/"], "adapters": ["internal/adapters/"], "tui": ["tui/"], "tests": ["test/"]}}