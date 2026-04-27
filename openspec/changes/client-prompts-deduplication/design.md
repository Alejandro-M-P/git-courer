# Design: client-prompts-deduplication

## Technical Approach

Eliminate hardcoded prompt duplication in `prompts/plugins/opencode/git-courer.ts` by fetching content dynamically from the binary via `git-courer prompts print --plugin <name>`. The Go binary embeds `agent-instructions.md` and exposes it through a new subcommand.

## Architecture Decisions

### Decision: Subprocess over File Import

| Option | Choice | Rationale |
|--------|--------|-----------|
| Subprocess: `git-courer prompts print` | **Selected** | Plugin runs in Node.js, binary is Go; subprocess is natural boundary |
| Embedding prompt as npm package | Rejected | Adds coupling; subprocess works even if binary updated |
| Dynamic import of .md file | Rejected | Relative paths break in various plugin loading contexts |

### Decision: Graceful Fallback to Hardcoded Content

| Choice | Rationale |
|--------|-----------|
| Keep minimal fallback in plugin | Binary may not be in PATH initially; plugin must work standalone |

**Fallback Strategy:**
1. Plugin tries `git-courer prompts print --plugin opencode`
2. If fails (timeout 1s, binary missing, parse error) → use embedded fallback
3. Fallback marked as "may be outdated" comment

### Decision: Single Binary Subcommand

| Choice | Rationale |
|--------|-----------|
| `git-courer prompts print --plugin <name>` | Matches CLI patterns; extensible for future plugins |

## Data Flow

```
OpenCode loads plugin
    ↓
git-courer.ts onLoad()
    ↓
exec("git-courer", ["prompts", "print", "--plugin", "opencode"])
    ↓
Binary reads embeddedInstructions → prints to stdout
    ↓
Plugin parses stdout → injects into system prompt
    ↓
Fallback: if error → use hardcoded minimal content
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `cmd/main.go` | Modify | Add `prompts print` subcommand handling |
| `internal/installer/mcp_config.go` | Modify | Export `embeddedInstructions` for subcommand |
| `prompts/plugins/opencode/git-courer.ts` | Modify | Replace 138-line constant with subprocess call + fallback |

## Interfaces / Contracts

### New Subcommand: `git-courer prompts print`
```go
// In cmd/main.go
case "prompts":
    if len(os.Args) > 2 && os.Args[2] == "print" {
        plugin := ""
        for i := 3; i < len(os.Args); i++ {
            if os.Args[i] == "--plugin" && i+1 < len(os.Args) {
                plugin = os.Args[i+1]
                break
            }
        }
        if plugin == "" {
            fmt.Fprintln(os.Stderr, "Error: --plugin flag required")
            os.Exit(1)
        }
        installer.PrintPrompt(plugin)
        return
    }
```

### New Function: `PrintPrompt()`
```go
// In mcp_config.go
func PrintPrompt(plugin string) error {
    switch plugin {
    case "opencode":
        fmt.Print(embeddedInstructions)
    default:
        return fmt.Errorf("unknown plugin: %s", plugin)
    }
    return nil
}
```

### Updated Plugin: git-courer.ts
```typescript
const result = await Bun.spawn({
  cmd: ["git-courer", "prompts", "print", "--plugin", "opencode"],
  timeout: 1000,
}).exitCode

// Fallback: minimal content if binary fails
const FALLBACK_INSTRUCTIONS = `# FALLBACK - may be outdated. Install git-courer for full instructions.
1. NEVER generate commits yourself - use git_write_review(COMMIT_START, ...)
2. Use git_read and git_write for all git operations`

const GIT_INSTRUCTIONS = fetchInstructions() ?? FALLBACK_INSTRUCTIONS
```

## Edge Cases

| Scenario | Handling |
|----------|----------|
| Binary not in PATH | Timeout 1s, catch error, use fallback |
| Unknown plugin name | Print "unknown plugin: <name>" to stderr, exit 1 |
| Output not UTF-8 | Fallback activated |
| Plugin subprocess crash | Catch, use fallback |

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `PrintPrompt()` output | Capture stdout, verify matches embeddedInstructions |
| Integration | Full subprocess flow | `git-courer prompts print --plugin opencode` matches .md file |
| Plugin | Graceful fallback | Remove binary from PATH, verify plugin loads |

## Migration / Rollout

1. Add `prompts print` subcommand first
2. Deploy updated binary
3. Update plugin to use subprocess
4. Keep fallback for backward compatibility

## Open Questions

- None