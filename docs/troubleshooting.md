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
curl -fsSL https://raw.githubusercontent.com/blak0p/git-courer/main/scripts/install.sh | sh
```

**Check 2: Is the MCP configured?**
```bash
# For OpenCode
cat ~/.config/opencode/opencode.json

# For Claude Code
cat ~/.claude.json

# For Codex
cat ~/.codex/config.toml
```

You should see a `"git-courer"` entry. If not, run:
```bash
git-courer mcp setup
```

**Check 3: Does the tool support MCP?**
- OpenCode: ✓ Native support
- Claude Code: ✓ Native support
- Codex: ✓ Native support
- pi: ✓ Native support
- Antigravity: ✓ Native support

## Ollama is not running / commit messages fail

**git-courer requires a model to be configured.** Without a model, operations will fail with an error like `llm.model is required`.

**To enable AI commit messages with Ollama:**
```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model (recommended: qwen3.5 or llama3.2)
ollama pull model

# Verify it's running
ollama list
```

**Config in `~/.config/git-courer/config.yaml`:**
```yaml
llm:
  provider: ollama
  base_url: http://localhost:11434/v1
  model: qwen3.5:latest
```

## LLM backend not available

If you configured a non-Ollama backend, verify it's running:

**LM Studio:**
```bash
curl http://localhost:1234/v1/models
```

**vLLM:**
```bash
curl http://localhost:8000/v1/models
```

**LocalAI:**
```bash
curl http://localhost:8080/v1/models
```

If the endpoint is unreachable, git-courer will return an error. Check:
1. The server is running and the model is loaded
2. `base_url` in config matches the server's actual URL (include `/v1`)
3. If the server requires an API key, set `api_key` in the `llm:` section

## Ollama-specific issues

**Auto-start not working:**
- Auto-start only works if `ollama` is in your `$PATH`
- On some systems, you may need to start Ollama manually or as a service: `ollama serve`

**Model not found:**
- Run `ollama list` to see available models
- Pull the model first: `ollama pull gemma4:26b`

**v1 endpoints not working (older Ollama):**
Update Ollama to v0.1.25 or newer — git-courer requires `/v1/` endpoints which have been standard since December 2023.

## "Permission denied" when installing

```bash
# Use sudo for global install, or manually install to ~/.local/bin
mkdir -p ~/.local/bin
curl -fsSL https://github.com/blak0p/git-courer/releases/latest/download/git-courer_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz | tar -xz -C ~/.local/bin git-courer
chmod +x ~/.local/bin/git-courer
```

## Secrets detected in commit

git-courer has 6 security layers that block commits with:
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
| Codex | `~/.codex/config.toml` |
| pi | `~/.pi/agent/mcp.json` |
| Antigravity | `~/.gemini/antigravity-cli/mcp_config.json` |

## Still having issues?

Open an issue: https://github.com/blak0p/git-courer/issues
