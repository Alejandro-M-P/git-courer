# Proposal: client-prompts-deduplication

## Intent

Eliminate prompt duplication between the embedded installer prompt and the OpenCode plugin. Currently both files contain identical 138 lines of agent instructions:
- `internal/installer/prompts/agent-instructions.md` (source of truth)
- `prompts/plugins/opencode/git-courer.ts` (lines 11-148, duplicated)

This violates DRY and creates maintenance burden — changes must be applied in two places.

## Scope

### In Scope
- Modify the OpenCode plugin to import the embedded prompt instead of duplicating it
- Keep `agent-instructions.md` as the single source of truth
- Ensure plugin loads prompt at runtime (not build-time)

### Out of Scope
- Changing the prompt content itself — that's a separate change
- Modifying MCP server configurations in the installer

## Capabilities

### New Capabilities
- `client-dynamic-prompts`: Client plugins load prompt content dynamically at runtime

### Modified Capabilities
- None — this is a refactor, no spec-level behavior change

## Approach

**Recommended**: Embed the prompt as a Go template in the binary and expose via `git-courer prompts print` subcommand. The plugin fetches it:

```bash
git-courer prompts print --plugin opencode
# outputs the prompt content
```

**Flow**:
1. Plugin `onLoad` calls `git-courer prompts print --plugin opencode`
2. Shell parses output and injects into the agent context
3. No duplication — single source of truth in the Go binary

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/installer/prompts/agent-instructions.md` | New | Add file path marker for plugin identification |
| `prompts/plugins/opencode/git-courer.ts` | Modified | Replace hardcoded content with subprocess call |
| `cmd/git-courer/main.go` | Modified | Add `prompts print` subcommand |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Binary bloat from embedding | Low | Prompt is small (~3KB); acceptable |
| Runtime failure if binary not in PATH | Low | Plugin checks PATH and falls back gracefully |

## Rollback Plan

1. Revert `git-courer.ts` to previous hardcoded content
2. Remove `prompts print` subcommand
3. Plugin works standalone again

## Dependencies

- None — self-contained refactor

## Success Criteria

- [ ] `git-courer prompts print --plugin opencode` outputs valid prompt content
- [ ] Plugin works even if binary not in PATH (graceful fallback)
- [ ] Single source of truth in `agent-instructions.md`