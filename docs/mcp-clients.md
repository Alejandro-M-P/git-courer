# MCP Clients

git-courer supports **5 MCP clients** and auto-configures all detected tools with a single command.

## Supported Clients

| Client | Status | Platform | Config Format |
|--------|--------|----------|---------------|
| OpenCode | ✓ | Linux, macOS, Windows | Object (`mcp`) |
| Claude Code | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Codex | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| pi | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Antigravity | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |

## Auto-Configure All

```bash
git-courer mcp setup
# ✓ OpenCode configured
# ✓ Codex configured
# ...
```

## Configure a Specific Client

```bash
git-courer mcp setup opencode
git-courer mcp setup codex
git-courer mcp setup claude-code
```

## Config Formats

### Object Format (most clients)
```json
{
  "mcpServers": {
    "git-courer": {
      "command": "/usr/.local/bin/git-courer",
      "args": ["mcp"]
    }
  }
}
```

### OpenCode Format
```json
{
  "mcp": {
    "git-courer": {
      "type": "local",
      "enabled": true,
      "command": ["/usr/.local/bin/git-courer", "mcp"]
    }
  }
}
```

## Detection Rules

git-courer detects a client if:
- The binary is in your `PATH` (e.g., `which opencode`), OR
- The config directory exists (e.g., `~/.config/opencode/`)

If auto-detect fails, you can manually configure:
```bash
git-courer mcp setup <client>
```

## Adding a New Client

If your AI tool supports MCP and isn't listed, open an issue. Adding a new client is usually **5 lines of code** — just define the config format and paths.

## Client not detecting?

For desktop apps installed via Flatpak/Snap, the config paths may differ. Check the client's documentation for the MCP config location.
