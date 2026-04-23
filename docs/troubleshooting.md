# Troubleshooting

Solutions for the most common issues with git-courer.

## git-courer is not detected by my AI tool

**Check 1: Is git-courer installed?**
```bash
which git-courer
# Should return: /path/to/git-courer
```

If not found, run:
```bash
git-courer update
# or reinstall:
curl -fsSL https://raw.githubusercontent.com/Alejandro-M-P/git-courer/main/scripts/install.sh | sh
```

**Check 2: Is the MCP configured?**
```bash
# For OpenCode
cat ~/.config/opencode/opencode.json

# For Claude Code
cat ~/.claude.json

# For Cursor
cat ~/.cursor/mcp.json
```

You should see a `"git-courer"` entry. If not, run:
```bash
git-courer mcp setup
```

**Check 3: Does the tool support MCP?**
- OpenCode: ✓ Native support
- Claude Code: ✓ Native support
- Cursor: ✓ Native support
- Windsurf: ✓ Native support
- VS Code (Cline): ✓ Via extension
- Claude Desktop: ✓ Only on macOS/Windows

## Ollama is not running / commit messages are generic

git-courer works without Ollama, but commit messages will be generic (`chore: update files`).

**To enable AI commit messages:**
```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model (recommended: qwen2.5 or llama3.2)
ollama pull qwen2.5:latest

# Verify it's running
ollama list
```

**Config in `.gcourer/config.yaml`:**
```yaml
ollama:
  model: qwen2.5:latest
  host: http://localhost:11434
```

## "Permission denied" when installing

```bash
# Use sudo for global install, or manually install to ~/.local/bin
mkdir -p ~/.local/bin
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz | tar -xz -C ~/.local/bin git-courer
chmod +x ~/.local/bin/git-courer
```

## Secrets detected in commit

git-courer has 5 security layers that block commits with:
- API keys (OpenAI, Stripe, AWS, etc.)
- Passwords
- Private tokens

**If it's a false positive:**
```bash
# Skip security check (not recommended)
git commit --no-verify
```

**Fix:** Move secrets to `.env` and add it to `.gitignore`.

## "Near-zero tokens" but my AI still uses tokens

git-courer only intercepts **git operations** (diff, commit, branch, merge). Your AI still uses tokens for:
- Reading and understanding your code
- Generating new code
- Answering questions

What git-courer saves: the 500–2,000 tokens × every automatic git operation your AI does.

## MCP config file locations

| Tool | Config Path |
|------|-------------|
| OpenCode | `~/.config/opencode/opencode.json` |
| Claude Code | `~/.claude.json` or `./.claude.json` |
| Cursor | `~/.cursor/mcp.json` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` |
| Cline | `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |
| Continue | `~/.continue/config.json` |
| VS Code | `~/.config/Code/User/mcp.json` |
| Zed | `~/.config/zed/settings.json` |
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) |
| Gemini CLI | `~/.gemini/settings.json` |

## Still having issues?

Open an issue: https://github.com/Alejandro-M-P/git-courer/issues
