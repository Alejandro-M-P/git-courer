# Tasks: client-prompts-deduplication

## Problem
`GetRuleFiles()` exists but NEVER called → rule files (CLAUDE.md, .cursorrules, skill) never created

## Solution
Call WriteRuleFiles in setup AND after update

---

- [ ] 1. Create WriteRuleFiles() in mcp_config.go (file: internal/installer/mcp_config.go)
  - Add function `WriteRuleFiles(binPath string) error`
  - Iterate over RuleFiles returned by `GetRuleFiles(binPath)`
  - Create parent directories with `os.MkdirAll`
  - Write each file with `os.WriteFile`
  - Skip files that already exist (don't overwrite)
  - Test: Call function → verify rule files created in home dir

- [ ] 2. Call WriteRuleFiles in RunSetup() (file: internal/installer/installer.go)
  - After MCP configuration, call `WriteRuleFiles(binPath)`
  - Print success message: "X rule file(s) created"
  - Test: Run `git-courer setup` → verify rule files exist

- [ ] 3. Call WriteRuleFiles after DownloadUpdate() (file: internal/installer/update.go)
  - After reconfiguring MCP (line ~100), call `WriteRuleFiles(binPath)`
  - Print updated message if files were created
  - Test: Run `git-courer update` → verify rule files exist

- [ ] 4. Delete unused plugin file (file: prompts/plugins/opencode/git-courer.js)
  - Remove file if it exists (skill replaces it)
  - Test: Verify file deleted, skill still works

---

## Files Modified
- `internal/installer/mcp_config.go` - add WriteRuleFiles()
- `internal/installer/installer.go` - call in RunSetup()
- `internal/installer/update.go` - call after DownloadUpdate()

## Files Deleted
- `prompts/plugins/opencode/git-courer.js`