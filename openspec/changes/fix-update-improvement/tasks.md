# Tasks: Update Command Improvement

## Task List

### Phase 1: Setup

- [x] **T001**: Add `go-selfupdate` dependency to `go.mod`
  - Run: `go get github.com/creativeprojects/go-selfupdate/v1@v1.5.2`
  - Verify `go.mod` and `go.sum` updated

### Phase 2: Implementation

- [x] **T002**: Modify `internal/installer/update.go` - add imports for go-selfupdate
  - Add `context`, `selfupdate`, `source` imports
  - Remove `archive/tar`, `compress/gzip`, `io` imports (no longer needed)

- [x] **T003**: Modify `CheckForUpdates()` - use `selfupdate.DetectLatest()`
  - Replace `FetchLatestRelease()` call with `selfupdate.DetectLatest(ctx, source, "Alejandro-M-P", "git-courer")`
  - Return version comparison result
  - See design.md lines 46-61 for reference implementation

- [x] **T004**: Modify `DownloadUpdate()` - use `selfupdate.UpdateTo()`
  - Replace manual download, extract, copy with `selfupdate.UpdateTo(ctx, release, currentPath)`
  - Keep MCP reconfiguration at the end (lines 97-102)
  - Remove hardcoded `/tmp/` paths (lines 49, 133)
  - See design.md lines 66-94 for reference implementation

- [x] **T005**: Remove `extractBinaryFromTarGz()` function from `update.go`
  - Entire function (lines 108-153) - ~45 lines
  - No longer needed with go-selfupdate

- [x] **T006**: Remove `copyFile()` function from `update.go`
  - Entire function (lines 156-175) - ~20 lines
  - No longer needed with go-selfupdate

### Phase 3: Cleanup

- [x] **T007**: Remove github.go functions (NO usages elsewhere confirmed)
  - **Remove from `internal/installer/github.go`:**
    - `FetchLatestRelease()` (lines 26-54)
    - `FindAsset()` (lines 58-69)
    - `contains()` (lines 72-87)
    - `indexOf()` (lines 90-97)
    - `isAlphanumeric()` (lines 100-102)
    - `DownloadAsset()` (lines 105-124)
    - `GetInstallPath()` (lines 127-136)
    - `Release` type (lines 14-17)
    - `Asset` type (lines 20-23)
  - **Expected result:** `github.go` should be empty or contain only unused helper functions

- [x] **T008**: Remove/update tests that depend on removed code
  - **Remove `TestFindAsset`** from `installer_test.go` (lines 64-132)
    - Tests the now-removed `FindAsset` method
  - **Remove `TestGetInstallPath_ReturnsValidPath`** from `installer_test.go` (lines 546-559)
    - Tests the now-removed `GetInstallPath` function

### Phase 4: Verification

- [x] **T009**: Run `go build ./...` to verify compilation
  - All imports should resolve
  - No undefined reference errors

- [ ] **T010**: Run `go test ./internal/installer/...` to verify tests pass
  - All remaining tests should pass
  - No test failures related to removed functions

- [ ] **T011**: Verify `git-courer update --help` shows help
  - Confirm update command is accessible

- [ ] **T012**: Manual test - run `git-courer version` and `git-courer update`
  - Should complete without errors
  - Should display appropriate messages

## Dependencies

```
T001 (add dependency)
   └── T002, T003, T004 (modify update.go)
             └── T005, T006 (remove old functions)
                       └── T007 (cleanup github.go)
                              └── T008 (update tests)
                                    └── T009, T010, T011, T012 (verification)
```

## Usage Analysis (Pre-verification)

| Function | Used In | Action |
|----------|---------|--------|
| `FetchLatestRelease` | `update.go:20,37` | **REMOVE** - replaced by selfupdate |
| `FindAsset` | `update.go:43`, `installer_test.go:120` | **REMOVE** - replaced by selfupdate |
| `DownloadAsset` | `update.go:51` | **REMOVE** - replaced by selfupdate |
| `GetInstallPath` | `installer_test.go:547` | **REMOVE** - unused after update.go change |
| `Release` type | `update.go`, `installer_test.go` | **REMOVE** - replaced by selfupdate types |
| `Asset` type | `update.go`, `installer_test.go` | **REMOVE** - replaced by selfupdate types |
| `extractBinaryFromTarGz` | `update.go:61` | **REMOVE** - replaced by selfupdate |
| `copyFile` | `update.go:83` | **REMOVE** - replaced by selfupdate |

## Code Size Impact

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| `update.go` | 189 lines | ~60 lines | **-129 lines** |
| `github.go` | 136 lines | 0 lines | **-136 lines** |
| Total | 325 lines | ~60 lines | **-265 lines** |

## Notes

- **Windows fix:** go-selfupdate handles OS-specific temp paths automatically - no more `/tmp/` hardcoding
- **Rollback safety:** `selfupdate.UpdateTo()` automatically restores previous binary on failure
- **Checksum verification:** Built into go-selfupdate library
- **T007 conditional:** If any usage is found during implementation, mark functions as deprecated instead of removing