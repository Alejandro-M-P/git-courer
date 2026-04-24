# Proposal: Switch to go-selfupdate Library for Safer Updates

## Intent

Replace custom update implementation with `github.com/creativeprojects/go-selfupdate/v1` to fix broken Windows updates (hardcoded `/tmp/` paths), eliminate unsafe binary replacement (no rollback), and add checksum verification. Reduces ~180 lines of custom code to ~20 lines.

## Scope

### In Scope
- Refactor `internal/installer/update.go` to use go-selfupdate library
- Remove custom `extractBinaryFromTarGz` and `copyFile` functions
- Remove custom GitHub API logic in `github.go` (fetching, asset matching)
- Add dependency: `github.com/creativeprojects/go-selfupdate/v1`
- Update `go.mod` and `go.sum`

### Out of Scope
- GPG signature verification (future enhancement)
- Changing release process or archive format
- Modifying other installer commands (install, uninstall)

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `self-update`: Replace custom update implementation with library-based approach that provides rollback, checksum verification, and cross-platform temp path handling

## Approach

Use `github.com/creativeprojects/go-selfupdate/v1`:
- **Windows compatibility**: Library handles OS-specific temp paths (fixes hardcoded `/tmp/`)
- **Rollback**: `UpdateTo()` automatically restores previous binary on failure (fixes unsafe `os.Remove` before copy)
- **Security**: Built-in SHA256 checksum verification
- **Minimal code**: ~20 lines replace ~180 lines of custom tar/gzip handling
- **Active maintenance**: v1.5.2 (Dec 2025), Go 1.24 compatible

Implementation replaces `CheckForUpdates()` and `DownloadUpdate()` with library calls to `DetectLatest()` and `UpdateTo()`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/installer/update.go` | Modified | Remove ~180 lines, add ~20 lines using go-selfupdate |
| `internal/installer/github.go` | Modified | Remove custom FetchLatestRelease, FindAsset, DownloadAsset functions |
| `go.mod` | Modified | Add `github.com/creativeprojects/go-selfupdate/v1` dependency |
| `go.sum` | Modified | Update checksums for new dependency |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Library API changes in future versions | Low | Pin to v1.x series, update tests |
| Release archive format change | Low | Library supports multiple formats (tar.gz, zip, xz) |

## Rollback Plan

If issues arise after deployment:
1. Revert commit using `git revert`
2. Run `go mod tidy` to remove dependency
3. Restore previous `update.go` and `github.go` from git history

## Dependencies

- `github.com/creativeprojects/go-selfupdate/v1` (new dependency)

## Success Criteria

- [ ] Update command works on Windows without `/tmp/` errors
- [ ] Update command works on Linux/macOS
- [ ] Failed update leaves previous binary intact (rollback)
- [ ] Code reduction: ~180 lines removed, ~20 lines added
- [ ] All existing tests pass
