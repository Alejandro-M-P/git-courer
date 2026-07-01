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

git-courer has 6 cumulative security layers that block commits with:
- API keys (OpenAI, Stripe, AWS, etc.)
- Passwords
- Private tokens

The layers run in this order (matches `internal/security/security.go`): binary audit → folder blacklist → name blacklist → static analysis → regex → LLM verification. Legitimate Go test files (`*_test.go`) are exempted from the binary audit and regex layers.

**If it's a false positive:**
```bash
# Skip security check (not recommended)
git commit --no-verify
```

**Fix:** Move secrets to `.env` and add it to `.gitignore`.

## Diagnostics with `doctor`

Run a read-only health check across every detected MCP client:

```bash
git-courer doctor
```

For each client it prints:
- **Config path** — where the client config file lives.
- **MCP configured** — whether the `git-courer` MCP entry is present.
- **Prompt block injected** — whether the `<!-- git-courer start -->` golden-rules block is present in the client's instructions file.
- **Hooks installed** — `yes` / `no` / `partial` (clients that support hook files).
- **Claude hooks** — only shown for Claude Code (inline `settings.json` hooks).

If `doctor` reports `MCP configured: no` but you already ran `mcp setup`, the client's config path moved or was rewritten by another tool. Re-run `git-courer mcp setup`. If `Hooks installed: partial`, at least one expected hook entry is missing — re-run `mcp setup` (it is idempotent and only writes missing entries).

## `hook-check` troubleshooting

`hook-check` classifies a shell command and emits JSON. For the 23 git subcommands covered by git-courer MCP tools (status, diff, commit, etc.) it emits `permissionDecision: "deny"` with a message pointing to the equivalent tool. For unknown git subcommands it also emits deny as a safe default. Non-git commands and uncovered git subcommands (init, clone, config, etc.) exit cleanly with no output.

```bash
# Direct invocation
git-courer hook-check "git status"

# Codex stdin mode (no args; reads Codex hook JSON from stdin)
echo '{"event":{"input":{"command":"git push"}}}' | git-courer hook-check
```

- **No output for a non-git command** — expected. `hook-check` only emits JSON when the command classifies as a git command (`Decision == "ask"`).
- **`usage: git-courer hook-check <command>`** — you passed no argument and stdin is not a pipe. Either pass a command as the first arg or pipe Codex hook JSON via stdin.
- **No output in stdin mode** — the input wasn't valid Codex hook JSON. `hook-check` exits cleanly as a safe fallback (it never blocks the agent).
- Companion hooks (`session-start-hook`, `subagent-start-hook`, `pre-invocation-hook`) emit golden-rules `additionalContext` and never set `permissionDecision`.

## Session issues

### Worktree conflicts on `session finish`

`session finish` removes the session worktree but **leaves the branch alive** for manual integration (no merge is attempted). If finish fails with `cleanup_failed`, the worktree may still be locked:

```bash
# List worktrees to find stale ones
git worktree list

# Remove a stale worktree manually
git worktree remove --force /path/to/worktree
git worktree prune
```

After a `cleanup_failed` finish, the session metadata is preserved with `status: cleanup_failed`. Retry `session finish`, or discard the session with `session discard` (requires `confirmed: true`).

### Stale sessions

Sessions whose worktree was deleted out-of-band (manually, disk cleanup, reboot) become stale. Recover with:

```bash
git-courer session status   # via the MCP tool — lists all sessions
git-courer session discard  # removes session metadata without touching the branch
```

`session finish` will not delete the session branch — if you want it gone, delete it explicitly with `branch DELETE`.

### Uncommitted changes block finish

`session finish` runs a light preview whose only abort gate is uncommitted/untracked changes (data-loss guard). Commit or stash your work first:

```bash
# Via the MCP tools
git-courer commit APPLY     # or
git-courer stash SAVE
```

## `init`, `version --predict`, `remove` commands

### `git-courer init`

Launches the TUI init wizard to create or update project configuration (per-project `.git/git-courer/config.json`, including `user_name` / `user_email` / `signing_key`).

```bash
git-courer init
```

### `git-courer version --predict`

Predicts the next release tag from conventional commits since the latest tag — no LLM required, pure Go:

```bash
git-courer version --predict
# Current: v2.8.0
# Bump:    minor
# Next:    v2.9.0
```

Requires a git repo with at least one tag. With no new commits since the latest tag, it prints the current tag and exits.

### `git-courer remove`

Cleans project-level git-courer artifacts (the per-project config). It does **not** uninstall the binary — use `git-courer uninstall` for that.

## Self-update asset matching

`git-courer update` queries the latest GitHub release and matches a release asset to your `GOOS`/`GOARCH`:

1. Goreleaser archive pattern first (`git-courer_<version>_<os>_<arch>.tar.gz` / `.zip`).
2. Raw binary pattern as fallback (`git-courer_<version>_<os>_<arch>`).

If no asset matches, it fails with `no compatible asset found for <GOOS>/<GOARCH>`. Workarounds:

- Build from source: `go install github.com/blak0p/git-courer@latest` (requires Go 1.26+, `CGO_ENABLED=1`).
- `git-courer update --force` re-runs the check and reconfigures MCP clients afterward.

## OpenCode policy issues

`mcp setup` merges the git-courer policy into `~/.config/opencode/opencode.json`:

- `permission.bash["git *"] = "ask"` (preserving any existing keys; Go's map ordering is non-deterministic so existing keys are kept).
- `instructions` array includes the `AGENTS.md` path (legacy `GIT_COURER.md` entries are removed).

If `opencode.json` is unparseable JSON, `mcp setup` backs it up to `opencode.json.bak` and writes a fresh config with the policy. If your OpenCode agent isn't prompting for permission on `git *` commands:

1. Run `git-courer doctor` — confirm `MCP configured: yes` and `Prompt block injected: yes`.
2. Check the backup: `ls ~/.config/opencode/*.bak` — restore manually if a third-party tool overwrote your config.
3. Re-run `git-courer mcp setup` (idempotent; only writes missing keys).

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
