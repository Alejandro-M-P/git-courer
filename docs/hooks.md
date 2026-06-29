# Hooks Reference

git-courer installs **lifecycle hooks** into MCP clients so agents always see the git-courer Golden Rules and are nudged toward the git-courer MCP tools instead of raw git.

This is the dedicated hooks reference. For the per-client setup overview, see [mcp-clients.md](./mcp-clients.md).

---

## Event Types

git-courer uses four hook events. Each event has a matcher (which agent actions it fires on), a command (which git-courer subcommand is invoked), and a purpose.

| Event | Matcher | Command | Fires When | Purpose |
|-------|---------|---------|------------|---------|
| `PreToolUse` | `Bash` (Claude Code, Codex) / `run_command` (Antigravity) | `git-courer hook-check` | The agent is about to run a shell command | Classify the command; if it's a git command, emit `additionalContext` suggesting the git-courer MCP tool. **Never denies** — only suggests. |
| `SessionStart` | `startup\|resume` | `git-courer session-start-hook` | A new agent session starts (or resumes) | Inject the Golden Rules into the agent's context so the workflow is known from the first turn. |
| `SubagentStart` | `general-purpose\|Explore\|Plan` | `git-courer subagent-start-hook` | A subagent is spawned | Inject the Golden Rules into the subagent's context, so subagents also follow `session start` → `status` → `diff`/`review` → `pr-review`. |
| `PreInvocation` | *(empty matcher)* | `git-courer pre-invocation-hook` | Before every model call (Antigravity only) | Inject the Golden Rules before each model invocation. Compensates for clients that have no `SessionStart`/`SubagentStart` events. |

### What each hook emits

Every hook subcommand emits a JSON object on stdout shaped for the Codex hook contract:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "additionalContext": "..."
  }
}
```

- `hookEventName` echoes the event (`PreToolUse`, `SessionStart`, `SubagentStart`, `PreInvocation`).
- `additionalContext` is the Golden Rules markdown for the three context-injection hooks. For `PreToolUse` it is a short suggestion like `Use git-courer/<tool> instead of bash <command>` when the command is a git command; for non-git commands, `hook-check` exits cleanly with **no output** (no decision is emitted, no command is denied).
- `permissionDecision` / `permissionDecisionReason` are **never set** by git-courer — it never denies a command. The agent (and the user, via the client's own permission prompt) always retains the final say.

---

## Per-Client Differences

Clients store hooks differently and support different event subsets. git-courer adapts to each.

### Claude Code — inline hooks in `settings.json`

- **Storage**: hooks are stored **inline** in `~/.claude/settings.json` under the top-level `hooks` object, keyed by event name. There is no separate hooks file.
- **Form**: Claude Code uses the exec form — `type: "command"` with separate `command` and `args` fields, plus an optional `timeout` (seconds). git-courer sets `timeout: 10` on `SessionStart`, `SubagentStart`, and `UserPromptSubmit`.
- **Event names**: Claude Code calls the `PreInvocation` event **`UserPromptSubmit`**. git-courer installs it under that name.
- **Events installed**: `PreToolUse` (matcher `Bash`), `SessionStart` (`startup|resume`), `SubagentStart` (`general-purpose|Explore|Plan`), `UserPromptSubmit` (empty matcher).
- **Merge**: existing non-git-courer hooks and every other top-level settings key (`permissions`, `model`, `theme`, …) are preserved. Matching is by `(matcher, command containing "git-courer")` — same matcher + git-courer command is updated in place (handles binary path changes); same matcher + no git-courer command appends git-courer alongside the existing hook.
- **Write**: atomic (temp file + rename).
- **Backup**: `~/.claude/settings.json` → `~/.claude/settings.json.bak`.

### Codex — separate `hooks.json`

- **Storage**: hooks are stored in a **separate** `~/.codex/hooks.json` file (not inside `config.toml`).
- **Form**: simple `{ "type": "command", "command": "..." }` entries, no timeout field.
- **Events installed**: all four — `PreToolUse` (matcher `Bash`), `SessionStart` (`startup|resume`), `SubagentStart` (`general-purpose|Explore|Plan`), `PreInvocation` (empty matcher).
- **Stdin mode**: when `git-courer hook-check` is invoked with no args and stdin is a pipe, it reads Codex hook JSON from stdin (`event.input.command`), classifies it, and emits a Codex-shaped `hookSpecificOutput`.
- **Backup**: `hooks.json` → `hooks.json.bak`.

### Antigravity — separate `hooks.json` + declarative permissions

- **Storage**: hooks in `~/.gemini/antigravity-cli/hooks.json` AND declarative permissions in `~/.gemini/antigravity-cli/settings.json` (two separate files).
- **Matcher difference**: Antigravity uses the **`run_command`** matcher (not `Bash`) for `PreToolUse`.
- **Events installed**: only two — `PreToolUse` (`run_command`) and `PreInvocation` (empty matcher). Antigravity has no `SessionStart`/`SubagentStart` events, so `PreInvocation` runs before every model call to inject the Golden Rules.
- **Permissions** (`settings.json`): three entries are merged into the existing `permissions` object:
  - `permissions.allow` gains `"mcp(git-courer/*)"` — allow all git-courer MCP tools without prompting.
  - `permissions.ask` gains `"command(git *)"` — prompt before raw git commands.
  - `permissions.ask` gains `"command(*)"` — prompt before any other shell command.
- **Backup**: `hooks.json` → `hooks.json.bak`; `settings.json` → `settings.json.gc.bak` (distinct suffix to avoid colliding with any Antigravity-native backup scheme).

### OpenCode — no hooks (policy + prompt block instead)

- **No hooks**: OpenCode does not support lifecycle hooks. git-courer instead injects:
  - `permission.bash["git *"] = "ask"` into `opencode.json` — OpenCode asks before raw git commands.
  - The `AGENTS.md` path into the `instructions` array — so OpenCode reads the Golden Rules from the prompt block it already loads.
- See [mcp-clients.md § OpenCode Policy Injection](./mcp-clients.md#5-opencode-policy-injection).

### Cursor

- Cursor is **not** auto-configured by `git-courer mcp setup`. If you use Cursor, add the MCP server manually using the standard JSON format (see [mcp-clients.md § Config Formats](./mcp-clients.md#config-formats)). Cursor does not expose a lifecycle hook system git-courer can target, so no hooks or prompt block are installed for it.

---

## Lifecycle

### Install

`git-courer mcp setup` (or `mcp setup <client>`) installs hooks for every detected client that supports them:

1. **Backup** any existing hooks/settings file before the first mutation.
2. **Merge** git-courer hook entries into the existing hooks object, preserving every non-git-courer hook.
3. **Write** the merged file atomically where the client supports it (Claude Code, Antigravity).

Install is **idempotent** — running it twice produces byte-identical output. The backup is not overwritten on re-run.

### Status

`git-courer doctor` reports the hook status per client:

- **`installed`** — all git-courer hook events are present with a git-courer command for the expected matcher.
- **`not_installed`** — none of the git-courer hook events are present.
- **`partial`** (Claude Code only) — some but not all git-courer hook events are present. Re-run `git-courer mcp setup <client>` to complete the install.

### Remove

`git-courer remove` cleans up hooks for every known client (even clients whose binary is no longer on `PATH`):

1. If a `.bak` (or `.gc.bak`) exists, it is **restored** over the live file and the backup is removed.
2. Otherwise, git-courer strips only the git-courer-owned entries in place, preserving non-git-courer hooks and every other settings key.

Remove is idempotent — running it twice does not error and does not touch user content.

---

## Configuration Examples

### Claude Code (`~/.claude/settings.json`)

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "/usr/local/bin/git-courer hook-check", "args": [] }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "startup|resume",
        "hooks": [
          { "type": "command", "command": "/usr/local/bin/git-courer session-start-hook", "args": [], "timeout": 10 }
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": "general-purpose|Explore|Plan",
        "hooks": [
          { "type": "command", "command": "/usr/local/bin/git-courer subagent-start-hook", "args": [], "timeout": 10 }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          { "type": "command", "command": "/usr/local/bin/git-courer pre-invocation-hook", "args": [], "timeout": 10 }
        ]
      }
    ]
  }
}
```

### Codex (`~/.codex/hooks.json`)

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

### Antigravity

`~/.gemini/antigravity-cli/hooks.json`:

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

`~/.gemini/antigravity-cli/settings.json`:

```json
{
  "permissions": {
    "allow": ["mcp(git-courer/*)"],
    "ask": ["command(git *)", "command(*)"]
  }
}
```

---

## Verifying Your Installation

```bash
git-courer doctor
```

Check the `Hooks installed` and `Claude hooks` rows in the report. If either shows `not_installed` or `partial` for a client you use, re-run:

```bash
git-courer mcp setup <client>
```

To manually verify a hook is wired, run the subcommand directly — it should emit JSON on stdout:

```bash
/usr/local/bin/git-courer hook-check "git status"
/usr/local/bin/git-courer session-start-hook < /dev/null
```