# Proposal: installer-go-mcp-config

## Intent

Solve the `go install` gap: users running `go install github.com/Alejandro-M-P/git-courer/cmd@latest` get the binary but NO MCP configuration. The shell installer (`scripts/install.sh`) runs `git-courer setup` after download, but `go install` bypasses this entirely.

**Why this matters**: `go install` is the documented way to install Go binaries. Users following standard Go patterns end up with a broken MCP setup, leading to confusion and support requests.

## Scope

### In Scope
- Modify `internal/installer/installer.go` to run MCP configuration after `go install`
- Ensure the setup runs non-interactively (CI-friendly)
- Handle edge cases: already configured, partial install, network failure

### Out of Scope
- Modifying the shell installer (`scripts/install.sh`) — it's already working
- Changing MCP server implementations
- Adding new MCP capabilities

## Capabilities

### New Capabilities
- `installer-go-install-mcp`: Configures MCP clients automatically after `go install`

### Modified Capabilities
- None — this is a net-new capability

## Approach

**Option A (Recommended)**: Embed a post-install hook in the binary itself
- When `git-courer` runs with `setup` subcommand (passed via ENV or flag), it runs `ConfigureAllMCP`
- Users add `post-install` hook to their `go install` command

**Option B**: Create a wrapper script that wraps `go install` + setup

**Recommendation**: Option A — use an environment variable `GIT_COURER_POSTINSTALL=1` to trigger MCP setup after the binary is installed. This is non-invasive and works with existing `go install` patterns.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/installer/installer.go` | Modified | Add post-install detection logic in `RunSetup` or main |
| `cmd/git-courer/main.go` | Modified | Pass setup flag/ENV to installer |
| `scripts/install.sh` | No change | Already works |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Duplicate setup on every run | Medium | Guard with flag/ENV check; only run once |
| CI environments affected | Low | Detect non-interactive mode; skip prompts |

## Rollback Plan

1. Remove the post-install environment variable handling
2. Revert changes to `installer.go` and `main.go`
3. Users manually run `git-courer setup` if needed

## Dependencies

- None — pure self-contained change

## Success Criteria

- [ ] `go install github.com/Alejandro-M-P/git-courer/cmd@latest && $(go env GOPATH)/bin/git-courer setup` configures MCPs non-interactively
- [ ] CI environments (no TTY) skip interactive prompts
- [ ] No duplicate setup on repeated runs