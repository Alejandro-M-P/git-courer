# SPEC: client-prompts-deduplication

## Requirements

### R1: Single Source of Truth
**When**: Agent instructions are needed  
**Then**: Use `internal/installer/prompts/agent-instructions.md` as single source

### R2: Prompts Print Subcommand
**When**: User runs `git-courer prompts print --plugin <plugin-name>`  
**Then**: Output the embedded prompt content for that plugin

### R3: Plugin Dynamic Loading
**When**: OpenCode plugin loads  
**Then**: Fetch prompt content at runtime via subprocess, not hardcoded

### R4: Graceful Fallback
**When**: Binary not in PATH or subprocess fails  
**Then**: Fall back to embedded content within plugin

### R5: Preserve Existing Behavior
**When**: Fallback is used  
**Then**: Plugin works identically to before

## User-Facing Behavior

### User Flow 1: Print Prompt Content
```bash
git-courer prompts print --plugin opencode

# Expected output:
# # Git-courer AI Assistant Rules
# ...
# (full agent-instructions.md content)
```

### User Flow 2: Plugin Fetches Prompt
```bash
# OpenCode starts and loads git-courer plugin
opencode

# Plugin executes: git-courer prompts print --plugin opencode
# Output is injected into system prompt
```

### Plugin Behavior (Internal)
```typescript
// In git-courer.ts plugin
const result = await $.exec("git-courer", ["prompts", "print", "--plugin", "opencode"])
const promptContent = result.stdout
// Inject into agent context
```

## Edge Cases

| Scenario | Expected Behavior |
|----------|-----------------|
| Binary not in PATH | Fall back to hardcoded content in plugin |
| Subprocess times out | Fall back to hardcoded content |
| Invalid --plugin flag | Print error to stderr, exit non-zero |
| Plugin unknown | Error: "unknown plugin: <name>" |
| Output not valid UTF-8 | Fall back to hardcoded content |

## Test Scenarios

### TS1: Prompts Print Output
```bash
git-courer prompts print --plugin opencode

# Verify: Output matches agent-instructions.md
# Expected: Full content of internal/installer/prompts/agent-instructions.md
```

### TS2: Plugin Uses Subprocess
```bash
# Check that plugin doesn't have hardcoded content
grep -A 5 "GIT_INSTRUCTIONS =" prompts/plugins/opencode/git-courer.ts

# Expected: Should NOT find 138-line constant
# (or it's minimal/empty and loaded dynamically)
```

### TS3: Fallback When Binary Missing
```bash
# Setup: Remove git-courer from PATH
# Start OpenCode with plugin

# Verify: Plugin still works
# Expected: Uses fallback content in plugin
```

### TS4: Unknown Plugin Error
```bash
git-courer prompts print --plugin unknown

# Expected:
# Error: unknown plugin: unknown
# Exit code: 1
```

### TS5: Dynamic Content at Runtime
```bash
# Modify agent-instructions.md content
echo "# Updated" >> internal/installer/prompts/agent-instructions.md
# Rebuild binary
go build -o git-courer ./cmd

# Run plugin with updated binary
git-courer prompts print --plugin opencode

# Verify: Output contains new content
# Expected: Shows updated content
```

## Non-Functional Requirements

### Binary Size
- Embedded prompt adds ~3KB to binary (acceptable)

### Runtime
- Subprocess call must complete within 500ms
- Fallback activates after 1s timeout

### Backward Compatibility
- Plugin must work if binary missing (fallback)
- No behavior change for end users

### Fallback Content
- Include minimal fallback in plugin (~10 lines max)
- Mark as "fallback - may be outdated"