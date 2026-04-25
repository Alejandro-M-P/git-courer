# Design: Update Command with go-selfupdate

## Architecture Overview

Replace custom update implementation with `github.com/creativeprojects/go-selfupdate/v1` library.

## Current Architecture (To Be Removed)
- `CheckForUpdates()`: Custom GitHub API call via `FetchLatestRelease()`
- `DownloadUpdate()`: Manual tar.gz download, extract, and copy
- `extractBinaryFromTarGz()`: Custom tar.gz extraction
- `copyFile()`: Manual file copy without atomicity
- `FetchLatestRelease()`, `FindAsset()`, `DownloadAsset()` in `github.go`

## New Architecture (Library-Based)

### Flow Diagram
```
User runs: git-courer update
    ↓
runUpdate() in cmd/main.go
    ↓
installer.RunUpdate(force)
    ↓
CheckForUpdates() - uses go-selfupdate.DetectLatest()
    ↓
[If new version] DownloadUpdate()
    ↓
go-selfupdate.UpdateTo(ctx, release, executable)
    ↓
[On failure] Automatic rollback via library
    ↓
Reconfigure MCP clients
```

### Key Changes

#### 1. `internal/installer/update.go`

**Remove:**
- `extractBinaryFromTarGz()` function (entire function, ~45 lines)
- `copyFile()` function (entire function, ~20 lines)
- Hardcoded `/tmp/` paths

**Modify `CheckForUpdates()`:**
```go
func CheckForUpdates() (bool, string, error) {
    ctx := context.Background()
    source := source.NewGitHubSource()
    
    release, err := selfupdate.DetectLatest(ctx, source, "Alejandro-M-P", "git-courer")
    if err != nil {
        return false, "", fmt.Errorf("failed to detect latest version: %w", err)
    }
    
    currentVersion := getCurrentVersion()
    if release.Version() != currentVersion {
        return true, release.Version(), nil
    }
    
    return false, currentVersion, nil
}
```

**Modify `DownloadUpdate()`:**
```go
func DownloadUpdate() error {
    ctx := context.Background()
    source := source.NewGitHubSource()
    
    release, err := selfupdate.DetectLatest(ctx, source, "Alejandro-M-P", "git-courer")
    if err != nil {
        return fmt.Errorf("failed to fetch release: %w", err)
    }
    
    currentPath, err := os.Executable()
    if err != nil {
        return fmt.Errorf("failed to get current executable path: %w", err)
    }
    
    // go-selfupdate handles: download, extract, verify checksum, atomic replace, rollback on failure
    if err := selfupdate.UpdateTo(ctx, release, currentPath); err != nil {
        return fmt.Errorf("update failed: %w", err)
    }
    
    // Reconfigure MCP
    binPath, _ := FindBinaryPath()
    if configured, err := ConfigureAllMCP(binPath); err != nil {
        fmt.Fprintf(os.Stderr, "  MCP setup: %v\n", err)
    } else if configured > 0 {
        fmt.Printf("  %d MCP client(s) reconfigured\n", configured)
    }
    
    return nil
}
```

**Add import:**
```go
import (
    "context"
    "github.com/creativeprojects/go-selfupdate/v1/selfupdate"
    "github.com/creativeprojects/go-selfupdate/v1/source"
)
```

#### 2. `internal/installer/github.go`

**Remove entirely or mark as deprecated:**
- `FetchLatestRelease()` function
- `FindAsset()` function  
- `DownloadAsset()` function
- Related types (`Release`, `Asset` structs)

Note: These may be used elsewhere. Check before removing.

#### 3. `go.mod`

Add dependency:
```
require github.com/creativeprojects/go-selfupdate/v1 v1.5.2
```

Run `go mod tidy` to update `go.sum`.

#### 4. `cmd/main.go`

**No changes needed** - `runUpdate()` already calls `installer.RunUpdate(force)`.

## Error Handling

### Rollback Mechanism
The `go-selfupdate` library automatically handles rollback:
- On successful download but failed install: restores previous binary
- Uses atomic file operations (rename instead of copy)
- No manual rollback code needed

### Error Cases
1. **Network error**: Library returns error, binary unchanged
2. **No new version**: `CheckForUpdates()` returns false, no update attempted
3. **Checksum mismatch**: Library verifies SHA256, fails if mismatch
4. **Permission error**: Standard Go error, user must fix permissions

## Testing Strategy

### Unit Tests
- Mock the `selfupdate.DetectLatest()` function
- Test `CheckForUpdates()` with mock releases
- Test version comparison logic

### Integration Tests
- Test update flow on test repository
- Verify rollback on simulated failure
- Test cross-platform compatibility (Windows/Linux/macOS)

### Manual Testing
- Run `git-courer update` on Windows
- Run `git-courer update` on Linux/macOS
- Simulate network failure (disconnect during update)
- Verify previous binary exists after failed update

## Migration Path

1. Add `go-selfupdate` dependency: `go get github.com/creativeprojects/go-selfupdate/v1`
2. Modify `update.go` to use library
3. Remove/unused custom functions
4. Run tests: `go test ./internal/installer/...`
5. Build and manual test on all platforms

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Library API changes | Pin to v1.x, monitor releases |
| `github.go` functions used elsewhere | Check all usages before removing |
| Binary not found after update | Library uses `os.Executable()`, reliable |

## References
- Library: https://github.com/creativeprojects/go-selfupdate
- Current code: `internal/installer/update.go`
