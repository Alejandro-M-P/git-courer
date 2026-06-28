# MCP Clients

git-courer supports **5 MCP clients** and automatically configures all detected tools, security policies, and prompt hooks with a single command.

---

## Supported Clients

| Client | Status | Platform | Config Format | Auto-configured Files |
|--------|--------|----------|---------------|-----------------------|
| **OpenCode** | ✓ | Linux, macOS, Windows | JSON | `~/.config/opencode/opencode.json` (Includes custom security policies) |
| **Claude Code** | ✓ | Linux, macOS, Windows | JSON | `~/.claude.json` (Includes lifecycle hook injection) |
| **Codex** | ✓ | Linux, macOS, Windows | TOML | `~/.codex/config.toml` (Includes `hooks.json` hooks configuration) |
| **pi** | ✓ | Linux, macOS, Windows | JSON | `~/.pi/agent/mcp.json` |
| **Antigravity** | ✓ | Linux, macOS, Windows | JSON | `~/.gemini/antigravity-cli/mcp_config.json` |

---

## Auto-Configure All

Run this command to automatically discover, configure, and install hooks for all installed clients on your machine:

```bash
git-courer mcp setup
```

To configure a single specific client:

```bash
git-courer mcp setup opencode
git-courer mcp setup codex
git-courer mcp setup claude-code
```

---

## Config Formats

### Standard JSON Format (Claude Code, pi, Antigravity)
Saves under `mcpServers` using standard camelCase:
```json
{
  "mcpServers": {
    "git-courer": {
      "command": "/usr/local/bin/git-courer",
      "args": ["mcp"]
    }
  }
}
```

### OpenCode JSON Format
Saves under `mcp` with type `local`:
```json
{
  "mcp": {
    "git-courer": {
      "type": "local",
      "enabled": true,
      "command": ["/usr/local/bin/git-courer", "mcp"]
    }
  }
}
```

### Codex TOML Format
Codex requires TOML configuration and uses the snake_case root key `mcp_servers`:
```toml
[mcp_servers.git-courer]
command = "/usr/local/bin/git-courer"
args = ["mcp"]
```

---

## Advanced Integration Features

When you run `git-courer mcp setup`, git-courer installs advanced integrations to ensure that agents follow the correct workflows:

### 1. Automatic Hook Injection (Claude Code, Codex, Antigravity)
git-courer registers hooks on events like `SessionStart` and `SubagentStart`. 
- **What it does**: These hooks intercept agent startup and dynamically inject git-courer's workflow rules (e.g. mandatory `session start` usage before modifying files) directly into the agent's system prompt context.
- **Why it matters**: This ensures that agents always know the git-courer rules, even in fresh terminal windows or subagents, without polluting your code repositories with rule files.

### 2. OpenCode Policy Injection
OpenCode requires explicit permission definitions for executing commands on the host machine.
- **What it does**: git-courer merges custom policy rules into `opencode.json`, automatically granting access to git commands and git-courer hooks to prevent annoying permission authorization prompts during code generation.

### 3. Prompt Block Injection
During setup or when running the `doctor` command, git-courer injects custom instruction prompt blocks directly into client-native instruction files such as `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` to guide agents correctly.
