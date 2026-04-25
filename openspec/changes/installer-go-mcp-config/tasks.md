# Tasks: installer-go-mcp-config

## Problem
`go install` doesn't run setup → MCPs never configured

## Solution
Add post-install hook with GIT_COURER_POSTINSTALL=1

---

- [ ] 1. Add post-install hook in installer.go (file: internal/installer/installer.go)
  - Add function `RunPostInstall()` that checks for `GIT_COURER_POSTINSTALL=1` env var
  - When set, call `RunSetup(".")` to configure MCPs and write rule files
  - Test: Set env var, run `go install`, verify MCP config files created

- [ ] 2. Accept ENV in cmd/main.go (file: cmd/main.go)
  - Check for `GIT_COURER_POSTINSTALL=1` env var in main()
  - If set, call `installer.RunPostInstall()` before entering MCP server mode
  - Test: Set env var + run binary → setup runs automatically

---

## Files Modified
- `internal/installer/installer.go` - add RunPostInstall()
- `cmd/main.go` - check env and call post-install