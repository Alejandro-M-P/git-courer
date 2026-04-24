# SPEC: installer-go-mcp-config

## Requirements

### R1: Post-install MCP Configuration
**When**: User runs `go install github.com/Alejandro-M-P/git-courer/cmd@latest`  
**Then**: User can trigger MCP configuration via post-install hook

### R2: Environment Variable Trigger
**When**: Binary runs with `GIT_COURER_POSTINSTALL=1` environment variable  
**Then**: MCP configuration runs non-interactively without user prompts

### R3: CLI Flag Trigger (Alternative)
**When**: Binary runs with `--postinstall` flag  
**Then**: Same behavior as R2

### R4: Idempotent Execution
**When**: MCP is already configured  
**Then**: Skip configuration without error

### R5: CI/Non-interactive Mode
**When**: No TTY is attached  
**Then**: Skip interactive prompts, continue silently

### R6: Network Failure Handling
**When**: Network call fails during configuration  
**Then**: Log error gracefully, don't crash

## User-Facing Behavior

### User Flow 1: Post-install via Environment Variable
```bash
go install github.com/Alejandro-M-P/git-courer/cmd@latest
GIT_COURER_POSTINSTALL=1 $(go env GOPATH)/bin/git-courer setup
```

### User Flow 2: Post-install via Flag
```bash
go install github.com/Alejandro-M-P/git-courer/cmd@latest
$(go env GOPATH)/bin/git-courer setup --postinstall
```

### Expected Output
```
Setting up git-courer...
  ✓ Project configured
  ✓ opencode configured
  ✓ cursor configured
  ✓ cline configured

✓ git-courer setup complete!
```

### Already Configured Output
```
Setting up git-courer...
  ✓ Project configured
  ✓ opencode already configured (skipped)

✓ git-courer setup complete!
```

## Edge Cases

| Scenario | Expected Behavior |
|---------|-----------------|
| No MCP clients detected | Print "No MCP clients detected", exit successfully |
| Config file permission denied | Print error to stderr, skip that client, continue |
| MCP already configured | Skip silently, log "already configured" |
| TTY unavailable | Skip interactive prompts, run non-interactively |
| Invalid config JSON | Print warning, skip that client |
| Binary path not found | Use "git-courer" as default |

## Test Scenarios

### TS1: Post-install MCP Configuration
```bash
# Setup: Clean environment with opencode installed
go install github.com/Alejandro-M-P/git-courer/cmd@latest
GIT_COURER_POSTINSTALL=1 $(go env GOPATH)/bin/git-courer setup

# Verify: opencode.json contains git-courer mcp config
cat ~/.config/opencode/opencode.json | grep git-courer
# Expected: contains "git-courer" with command pointing to binary
```

### TS2: Idempotent Execution
```bash
# Run setup twice
GIT_COURER_POSTINSTALL=1 git-courer setup
GIT_COURER_POSTINSTALL=1 git-courer setup

# Verify: No errors on second run
# Expected: "already configured" message
```

### TS3: CI Mode (No TTY)
```bash
# Run with no TTY
GIT_COURER_POSTINSTALL=1 git-courer setup </dev/null

# Verify: Doesn't hang or prompt for input
# Expected: Completes with output
```

### TS4: No MCP Clients
```bash
# Setup: No MCP clients installed
GIT_COURER_POSTINSTALL=1 git-courer setup

# Expected output:
#   No MCP clients detected
```

### TS5: Network Failure
```bash
# Setup: Network unavailable
GIT_COURER_POSTINSTALL=1 git-courer setup

# Verify: Graceful error handling
# Expected: Errors printed to stderr, not crash
```

## Non-Functional Requirements

### Performance
- Configuration completes within 5 seconds for 10 clients
- No blocking network calls without timeout (max 3s)

### Error Messages
- All errors printed to stderr
- Success messages printed to stdout

### Logging
- Use fmt.Printf for progress
- Use fmt.Fprintf(os.Stderr, ...) for warnings/errors