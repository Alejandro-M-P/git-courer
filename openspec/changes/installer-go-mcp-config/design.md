# Design: installer-go-mcp-config

## Technical Approach

Enable MCP configuration to run automatically after `go install` by detecting `GIT_COURER_POSTINSTALL=1` environment variable. The binary checks this flag at startup and triggers setup before entering MCP server mode.

## Architecture Decisions

### Decision: Environment Variable over CLI Flag

| Option | Choice | Rationale |
|--------|--------|-----------|
| ENV var `GIT_COURER_POSTINSTALL=1` | **Selected** | Works seamlessly with shell post-install hooks: `GIT_COURER_POSTINSTALL=1 git-courer setup` |
| CLI flag `--postinstall` | Rejected | Requires positional args; ENV is more composable with shell |
| Symlink/wrapper script | Rejected | Adds installation complexity; ENV approach is simpler |

### Decision: Run Before MCP Server Mode

| Choice | Rationale |
|--------|-----------|
| Check ENV in `main()` before routing | Avoids duplicating logic; any subcommand can trigger post-install |
| Check in `runSetup()` | Rejected — setup is a user action, not auto-trigger |

## Data Flow

```
go install → shell hook
    ↓
GIT_COURER_POSTINSTALL=1 git-courer setup
    ↓
main() → detects ENV → runPostInstall()
    ↓
RunSetup(".") → ConfigureAllMCP()
    ↓
MCP configs created
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `cmd/main.go` | Modify | Check `GIT_COURER_POSTINSTALL` before arg parsing |
| `internal/installer/installer.go` | Modify | Add `RunPostInstall()` function |

## Interfaces / Contracts

### New Function: `RunPostInstall()`
```go
// RunPostInstall runs setup if GIT_COURER_POSTINSTALL=1.
// Called from main() before any subcommand routing.
func RunPostInstall() {
    if os.Getenv("GIT_COURER_POSTINSTALL") != "1" {
        return
    }
    // Run setup non-interactively (already the default)
    RunSetup(".")
}
```

### Main Flow
```go
func main() {
    // Check post-install trigger FIRST
    if os.Getenv("GIT_COURER_POSTINSTALL") == "1" {
        installer.RunPostInstall()
    }
    
    // Continue with normal command routing...
}
```

## Edge Cases

| Scenario | Handling |
|----------|----------|
| `GIT_COURER_POSTINSTALL=1` but already configured | `ConfigureMCP()` skips clients with existing entries (idempotent) |
| Network failure during MCP config | Each client logs warning, continues with others |
| No MCP clients detected | `DetectClients()` returns empty; message printed, exits 0 |
| Binary not in PATH | `FindBinaryPath()` returns error; uses fallback `"git-courer"` |

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `RunPostInstall()` behavior | Mock `RunSetup()`, verify called when ENV set |
| Integration | End-to-end post-install flow | `GIT_COURER_POSTINSTALL=1 git-courer setup` → verify configs |
| E2E | `go install` + post-install | Fresh install, verify MCP configs exist |

## Migration / Rollout

No migration required. This is purely additive — existing workflows unchanged.

## Open Questions

- None