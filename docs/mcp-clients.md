# MCP Clients

git-courer supports **5 MCP clients** and automatically configures all detected tools, security policies, prompt blocks, and lifecycle hooks with a single command.

---

## Supported Clients

| Client | Status | Platform | Config Format | Auto-configured Files |
|--------|--------|----------|---------------|-----------------------|
| **OpenCode** | ✓ | Linux, macOS, Windows | JSON | `~/.config/opencode/opencode.json` (MCP + policy), `~/.config/opencode/AGENTS.md` (prompt block) |
| **Claude Code** | ✓ | Linux, macOS, Windows | JSON | `~/.claude.json` (MCP), `~/.claude/settings.json` (inline hooks), `~/.claude/CLAUDE.md` (prompt block) |
| **Codex** | ✓ | Linux, macOS, Windows | TOML | `~/.codex/config.toml` (MCP), `~/.codex/hooks.json` (hooks), `~/.codex/AGENTS.md` (prompt block) |
| **pi** | ✓ | Linux, macOS, Windows | JSON | `~/.pi/agent/mcp.json`, `~/.pi/agent/AGENTS.md` (prompt block) |
| **Antigravity** | ✓ | Linux, macOS, Windows | JSON | `~/.gemini/antigravity-cli/mcp_config.json` (MCP), `~/.gemini/antigravity-cli/hooks.json` (hooks), `~/.gemini/antigravity-cli/settings.json` (permissions), `~/.gemini/GEMINI.md` (prompt block) |

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
git-courer mcp setup antigravity
git-courer mcp setup pi
```

Setup is **idempotent** — re-running it never duplicates entries and never overwrites user content that is not owned by git-courer. Running setup twice produces byte-identical output.

### Backup and Restore During Setup

Before mutating any existing config file, git-courer backs it up alongside the original:

- `opencode.json` → `opencode.json.bak`
- `.claude.json` / `~/.claude/settings.json` → `.bak` sibling
- `~/.codex/hooks.json` → `hooks.json.bak`
- `~/.gemini/antigravity-cli/settings.json` → `settings.json.gc.bak`

On uninstall (`git-courer remove`), the `.bak` is restored over the live file and the backup is removed. If no backup exists, git-courer strips only the git-courer-owned entries in place (preserving everything else). The Antigravity settings backup uses a distinct `.gc.bak` suffix to avoid clashing with any Antigravity-native backup scheme.

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

When you run `git-courer mcp setup`, git-courer installs advanced integrations to ensure that agents follow the correct workflows. Each integration is targeted at the client that needs it — no client receives an integration it cannot use.

### 1. Hooks System (Claude Code, Codex, Antigravity)

git-courer registers **lifecycle hooks** so the agent always knows the git-courer workflow, even in a fresh terminal window or a subagent.

Four hook events are used:

| Event | Matcher | Command | Purpose |
|-------|---------|---------|---------|
| `PreToolUse` | `Bash` (Claude Code, Codex) / `run_command` (Antigravity) | `git-courer hook-check` | Classify the shell command the agent is about to run; suggest the git-courer MCP tool for git commands. Never denies — only suggests. |
| `SessionStart` | `startup\|resume` | `git-courer session-start-hook` | Inject the Golden Rules into the agent's context at session start. |
| `SubagentStart` | `general-purpose\|Explore\|Plan` | `git-courer subagent-start-hook` | Inject the Golden Rules when a subagent starts, so subagents also follow the rules. |
| `PreInvocation` | *(empty matcher)* | `git-courer pre-invocation-hook` | Inject the Golden Rules before every model call (Antigravity only — it has no SessionStart/SubagentStart). |

**Why it matters**: agents always see the `session start` → `status` → `diff`/`review` → `pr-review` workflow, without you having to repeat it in every repo or every subagent.

See [hooks.md](./hooks.md) for the full event reference, per-client differences, lifecycle, and configuration examples.

### 2. Claude Code Inline Hooks

Claude Code stores hooks **inline** in `~/.claude/settings.json` (under the top-level `hooks` object, keyed by event name) — not in a separate hooks file. git-courer merges its entries into that object and preserves every non-git-courer hook and every other top-level settings key (`permissions`, `model`, `theme`, etc.).

Claude Code uses the exec form: `type: "command"` with separate `command` and `args` fields, and supports an optional `timeout` (seconds). git-courer sets a 10-second timeout on `SessionStart`, `SubagentStart`, and `UserPromptSubmit` (Claude Code's name for the `PreInvocation` event).

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer hook-check", "args": [] }]
      }
    ],
    "SessionStart": [
      {
        "matcher": "startup|resume",
        "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer session-start-hook", "args": [], "timeout": 10 }]
      }
    ],
    "SubagentStart": [
      {
        "matcher": "general-purpose|Explore|Plan",
        "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer subagent-start-hook", "args": [], "timeout": 10 }]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer pre-invocation-hook", "args": [], "timeout": 10 }]
      }
    ]
  }
}
```

The merged file is written **atomically** (temp file + rename) so a crash mid-write never corrupts the real settings file.

### 3. Codex Hooks (Separate File)

Codex stores hooks in a separate `~/.codex/hooks.json` file. git-courer backs up the existing file to `hooks.json.bak` before the first mutation, then merges its entries. The structure is `hooks.<Event>[].matcher` + `hooks.<Event>[].hooks[].{type,command}`.

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Bash", "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer hook-check" }] }
    ],
    "SessionStart": [
      { "matcher": "startup|resume", "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer session-start-hook" }] }
    ],
    "SubagentStart": [
      { "matcher": "general-purpose|Explore|Plan", "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer subagent-start-hook" }] }
    ],
    "PreInvocation": [
      { "matcher": "", "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer pre-invocation-hook" }] }
    ]
  }
}
```

### 4. Antigravity Hooks + Permissions

Antigravity uses a **separate** `hooks.json` (with the `run_command` matcher instead of `Bash`) AND a separate `settings.json` for **declarative permissions**. git-courer installs both.

**Hooks** (`~/.gemini/antigravity-cli/hooks.json`) — only `PreToolUse` and `PreInvocation` (Antigravity has no SessionStart/SubagentStart events; `PreInvocation` runs before every model call to compensate):

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "run_command", "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer hook-check" }] }
    ],
    "PreInvocation": [
      { "matcher": "", "hooks": [{ "type": "command", "command": "/usr/local/bin/git-courer pre-invocation-hook" }] }
    ]
  }
}
```

**Permissions** (`~/.gemini/antigravity-cli/settings.json`) — three entries are merged into the existing `permissions` object:

- `permissions.allow` gains `"mcp(git-courer/*)"` — allow all git-courer MCP tools without prompting.
- `permissions.ask` gains `"command(git *)"` — prompt before raw git commands (so the agent is nudged toward the git-courer equivalent).
- `permissions.ask` gains `"command(*)"` — prompt before any other shell command.

Non-git-courer permission entries and every other top-level settings key are preserved. The backup uses `.gc.bak` so it does not collide with any Antigravity-native backup.

### 5. OpenCode Policy Injection

OpenCode requires explicit permission definitions for executing commands on the host machine. git-courer merges two policy entries into `opencode.json`:

- **`permission.bash["git *"] = "ask"`** — OpenCode asks the user before running raw git commands, which nudges the agent toward the git-courer MCP tool instead. Existing `permission.bash` keys are preserved; Go's alphabetical map sort on `json.MarshalIndent` naturally places `"git *"` after `"*"`, so last-match-wins applies.
- **`instructions`** array includes the path to `AGENTS.md` in the same directory as `opencode.json` (deduplicated; a string `instructions` value is converted to an array preserving the original entry). The legacy `GIT_COURER.md` path is removed if present.

If `opencode.json` does not exist, a fresh config with the policy is written. If it exists but is unparseable JSON, it is backed up and a fresh config with the policy is written. The merge is idempotent — running twice produces byte-identical output.

### 6. Prompt Block Injection

During setup (and when running `doctor`), git-courer injects a **prompt block** directly into the client-native instructions file so agents see the Golden Rules as part of their standing context:

- **OpenCode** → `~/.config/opencode/AGENTS.md`
- **Claude Code** → `~/.claude/CLAUDE.md`
- **Codex** → `~/.codex/AGENTS.md`
- **pi** → `~/.pi/agent/AGENTS.md`
- **Antigravity** → `~/.gemini/GEMINI.md`

The block is delimited by marker comments so git-courer can find and update it without touching anything else in the file:

```markdown
<!-- git-courer start -->
# git-courer — Golden Rules

git-courer is NOT a wrapper around git. Some tools do things raw git CANNOT.
Others are structured replacements that return JSON instead of human text.

## Golden Rules — save tokens and prevent mistakes

0. On session start (MANDATORY) → ALWAYS run `session start` to create an isolated worktree before starting any task.
1. Before any mutation → ALWAYS check `status` to know the repository state and identify active changes.
2. Before push or PR (or when verifying changes) → ALWAYS check `diff` + `review` to verify active diff checks.
3. Before PR → ALWAYS run `pr-review` to run all checks and verify changes in a single call.
<!-- git-courer end -->
```

If the file does not exist, git-courer creates it with just the wrapped block. If it exists, the block is inserted (or updated in place if a previous block is found between the markers). Anything outside the markers is left untouched. `git-courer remove` strips the block and leaves the rest of the file intact.

> The old physical `GIT_COURER.md` file is removed on setup if present — the prompt block now carries the same content in the instructions file the client already reads.

---

## Diagnostics: `doctor` and `hook-check`

### `doctor` (CLI / MCP)

```bash
git-courer doctor
```

Runs **read-only diagnostics** on every detected MCP client and prints a human-readable report. Per client it checks:

- **Config path** — the resolved config file.
- **MCP configured** — whether the config file exists and contains `git-courer`.
- **Prompt block injected** — whether the instructions file contains both `<!-- git-courer start -->` and `<!-- git-courer end -->`.
- **Hooks installed** — the Codex-style hook status (`installed` / `not_installed`) for clients with a `HooksPath`.
- **Claude hooks** — the Claude Code inline hook status (`installed` / `not_installed` / `partial`) for clients with a `SettingsPath`. Omitted for clients that do not use inline Claude hooks.

The MCP tool form returns the same diagnostic envelope as structured JSON (engram-doctor-style: a per-client list with each field). It is safe to run at any time — it never mutates anything.

### `hook-check` (CLI / hook entry point)

```bash
git-courer hook-check "<shell command>"
```

This is the command the `PreToolUse` hooks invoke. It classifies a shell command via the gitcmd classifier and emits the result as JSON on stdout. For git commands it includes `additionalContext` suggesting the git-courer MCP tool instead of bash — it **never denies** a command, it only nudges.

When invoked with no args and stdin is a pipe (Codex hook mode), it reads the Codex hook JSON from stdin, extracts the command, classifies it, and emits a Codex-shaped `hookSpecificOutput` with `additionalContext`. Non-git commands exit cleanly with no output.

The companion subcommands `session-start-hook`, `subagent-start-hook`, and `pre-invocation-hook` read stdin (ignored) and emit the Golden Rules as `additionalContext` for the `SessionStart`, `SubagentStart`, and `PreInvocation` events respectively.