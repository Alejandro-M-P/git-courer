# MCP Clients

git-courer supports **11 MCP clients** and auto-configures all detected tools with a single command.

## Supported Clients

| Client | Status | Platform | Config Format |
|--------|--------|----------|---------------|
| OpenCode | ✓ | Linux, macOS, Windows | Object (`mcp`) |
| Claude Code | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Cursor | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Windsurf | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Cline | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Continue | ✓ | Linux, macOS, Windows | **Array** (`mcpServers`) |
| VS Code | ✓ | Linux, macOS, Windows | Object (`servers`) |
| Zed | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Gemini CLI | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |
| Claude Desktop | ✓ | macOS, Windows only | Object (`mcpServers`) |
| Codex | ✓ | Linux, macOS, Windows | Object (`mcpServers`) |

## Auto-Configure All

```bash
git-courer mcp setup
# ✓ OpenCode configured
# ✓ Cursor configured
# ...
```

## Configure a Specific Client

```bash
git-courer mcp setup opencode
git-courer mcp setup cursor
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

### Continue Format (Array)
```json
{
  "mcpServers": [
    {
      "name": "git-courer",
      "command": "/usr/.local/bin/git-courer",
      "args": ["mcp"]
    }
  ]
}
```

### VS Code Format
```json
{
  "servers": {
    "git-courer": {
      "command": "/usr/.local/bin/git-courer",
      "args": ["mcp"]
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

## Client not detecting on Linux?

Some clients (like Claude Desktop) are **macOS/Windows only** and don't have Linux support yet.

For desktop apps installed via Flatpak/Snap, the config paths may differ. Check the client's documentation for the MCP config location.
