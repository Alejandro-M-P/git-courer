<!-- mcp-name: io.github.blak0p/git-courer -->
<!-- markdownlint-disable MD041 -->
<img width="1259" height="619" alt="Gemini_Generated_Image_g9lcw7g9lc" src="https://github.com/user-attachments/assets/2e9c0e64-b0de-4b83-9159-3a5906f9f3f4" />

<p align="center">
  <a href="https://github.com/blak0p/git-courer/releases/latest">
    <img src="https://img.shields.io/github/v/release/blak0p/git-courer?color=%2300BFFF&label=latest" alt="Release">
  </a>
  <a href="https://github.com/blak0p/git-courer/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/blak0p/git-courer/test.yml?branch=main" alt="Build">
  </a>
  <a href="https://github.com/blak0p/git-courer/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/blak0p/git-courer" alt="MIT License">
  </a>
</p>

> **Issues & Bugs**: [@blak0p/git-courer/issues](https://github.com/blak0p/git-courer/issues) · **Discussions**: [@blak0p/git-courer/discussions](https://github.com/blak0p/git-courer/discussions)

| Doc | Description |
| --- | --- |
| [Web](https://blak0p.github.io/git-courer/) | Visit the official website |
| [Roadmap](docs/roadmap.md) | What's coming next and the strategic vision |
| [Architecture](docs/architecture.md) | Codebase structure, patterns, and how to add features |
| [Troubleshooting](docs/troubleshooting.md) | Fix: Ollama not running, MCP not detected, permission errors |
| [MCP Clients](docs/mcp-clients.md) | All 5 supported CLI agents, config formats, manual setup |
| [Config Options](docs/config.md) | All `~/.config/git-courer/config.yaml` and `.git/git-courer/config.json` settings |
| [Commands](docs/commands.md) | Complete reference for all 13 MCP tools |
| [Contributing](docs/contributing.md) | Setup, running tests, and how to collaborate |

---

# git-courer

**Git, but agents can't break it.**

An MCP server that gives AI agents a full, safe interface to Git — not just commits, the whole surface: status, diff, branch, stash, history, sync. Every mutation backs itself up automatically. Nothing routes through Bash, so there's no `git reset --hard` happening behind your back.

13 tools. Structured JSON in, structured JSON out. No pagers, no text parsing, no guessing what the agent actually did to your repo.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/blak0p/git-courer/main/scripts/install.sh | sh
```

```go
go install github.com/blak0p/git-courer@latest
```

**Homebrew:**
```bash
brew install blak0p/tap/git-courer
```

### Quick start

```bash
git-courer mcp setup     # auto-configures your agent (OpenCode, Claude Code, Codex, pi, Antigravity)
```

Restart your agent, then ask it to run `status` on any repo. If it comes back with structured JSON instead of raw `git status` output, you're connected.

### Diagnostics & lifecycle

```bash
git-courer doctor              # read-only health report for every detected MCP client
git-courer hook-check "git status"   # classify a shell command (agent hook; never denies)
git-courer init                # TUI wizard to create/update project config
git-courer version --predict   # predict next release tag from conventional commits
git-courer remove              # remove project-level config (keeps the binary)
git-courer update              # self-update to the latest release + reconfigure MCP
```

`doctor` reports per client: config path, MCP configured, prompt block injected, hooks installed (`yes`/`no`/`partial`), and Claude inline hooks (Claude Code only).

---

## How an agent works with git-courer

```
Agent receives a task
        ↓
session start → creates isolated worktree + branch
        ↓
All MCP tools (diff, status, commit, branch...)
automatically redirect to that worktree
        ↓
commit PREVIEW → Go parses AST + dependency graph
                  classifies type (feat/fix/breaking...)
        ↓
commit APPLY → two modes depending on config:

  ┌─ WITH LLM ──────────────────────────────┐
  │  Local LLM writes the WHY/WHAT message │
  │  Go decides the type (agent can         │
  │  override with type= if wrong)          │
  └─────────────────────────────────────────┘

  ┌─ WITHOUT LLM (toggle off) ─────────────┐
  │  Perfect for modest laptops, no GPU    │
  │  Agent writes the message directly     │
  │  Preview, apply, and type override     │
  │  still work                             │
  └─────────────────────────────────────────┘

        ↓
Security checks → auto backup → commit
        ↓
"✓ fix: refactor session finish workflow to stop automatic merging

    WHY
    The previous implementation automatically attempted to merge session
    branches into the base branch, which forced a specific integration
    strategy and required managing two different git repositories.

    WHAT
    * Removed automated merge logic and the dependency on a second Git
      instance (mainGit) in the session handler and workflow.
    * Updated cleanup to remove worktrees while leaving session branches
      alive for manual integration.
    * Switched to PreviewLight validation to prevent data loss from
      uncommitted changes.
    * Updated FinishResult to include BranchAlive status."

session finish → closes session + cleans up worktree
```

---

## LLM Toggle

Modest laptop with no GPU? Flip the toggle and git-courer runs fully without an LLM. The agent writes messages directly, Go still decides the type.

```yaml
llm:
  enabled: false
```

Without LLM: commit with `message`, preview and type override work. Release is not available. Everything else (status, diff, branch, session, backup) works normally.

---

## Why it's different

### 1. Commit type decided by Go, not the LLM
AST analysis + deterministic rules. The LLM only writes the message. If the type is wrong, the agent overrides it with `type=`.

### 2. Dependency graph
Before committing, it maps what your changes affect across the entire codebase. Real impact, not just "you touched 3 files".

### 3. Isolated worktrees
Each agent gets its own directory and branch. No stepping on each other. `session start` creates, `session finish` closes and cleans up.

### 4. Commits as LLM context
Structured summary with WHY/WHAT. Any LLM consumes it directly. Fewer tokens, fewer hallucinations.

### 5. Automatic backup
Every write operation backs up before executing. One command undoes anything.

### 6. Releases that survive squashes
Commits are stored in `refs/courer/*`. Squash, rebase, force push — your changelog doesn't disappear.

---

## What a release looks like

```
❯ git-courer release

Tag? [v3.0.0]:
Add guidance for changelog generation? (y/N): n

   📦 Release Preview

  Tag: v3.0.0    Version Bump: major

  --- Changelog ---
  ...

Apply? (y/N/r/e):
```

This is the changelog it writes:

> **v2.8.0** — This update introduces an advanced session management system using git worktrees to enable parallel workflows and improves the robustness of agent execution rules.
>
> **Session Management and Isolation**
> - Implemented isolated sessions using git worktrees to prevent agents from interfering with each other; includes full lifecycle with listing, selection, and automatic cleanup via slugified identifiers.
> - Integrated sessionGit wrapper into the MCP server for automatic directory redirection.
>
> **Developer Experience and Configuration**
> - Refined golden rules, now prohibiting work in the repository root to enforce strict workspace isolation.
> - Automatic injection of prompt rule blocks into client configuration files.
> - Fixed TUI MCP setup bug where clients were not configured correctly.
>
> **System Robustness and Refactoring**
> - Refactored agent instruction structure, removing unnecessary tool maps.
> - Improved file cleanup using robust base names to prevent errors with complex paths.

---

## Workflows

### Session
`session start` → isolated worktree + branch. All MCP tools redirect there. `session finish` closes and cleans up. `session discard` throws it away.

### Commit
`PREVIEW` → review proposed commits. `APPLY` → executes them. Go splits files by dependency graph into atomic commits.

### PR Review
`pr-review` → tests + conflicts + diff stats + divergence. All in one call.

### Release
`git-courer release` → interactive. Pick the tag, guide the LLM, preview the changelog, confirm. Commits live in `refs/courer/*` — they survive squashes.

### Undo
`backup RESTORE` → undoes any operation.

---

## Tools (13)

| Tool | Subcommands | What it does |
| --- | --- | --- |
| `status` | — | Full repo state: branch, changes, conflicts, stash, etc. |
| `diff` | — | Diff with AST tags (`NEW_FUNC`, `MOD_SIG`, `DEPS`, `DEL`) |
| `commit` | `PREVIEW` → `APPLY` | 3-phase LLM pipeline: preview, review, apply |
| `branch` | `CREATE` / `SWITCH` / `DELETE` / `RENAME` / `LIST` | Branch management |
| `stage` | `RM` / `RESTORE` / `CLEAN` | Staging area control |
| `stash` | `SAVE` / `POP` / `SHOW` | Stash management |
| `history` | `LOG` / `REFLOG` / `BLAME` | History inspection |
| `sync` | `PUSH` / `PULL` / `FETCH` | Remote sync |
| `pr-review` | — | Tests + conflicts + diff stats + divergence in one call |
| `backup` | `RESTORE` / `LIST` | Undo amend/merge/rebase |
| `rewrite` | `AMEND` / `REVERT` / `SOFT` / `HARD` | History rewriting |
| `integrate` | `MERGE` / `UPDATE` / `PICK` / `CONTINUE` / `ABORT` | Branch integration |
| `session` | `start` / `finish` / `status` / `select` / `discard` | Isolated worktree lifecycle |

Full reference with examples: [docs/commands.md](docs/commands.md).

---

## Supported clients

| Tool | Auto-configured |
|------|----------------|
| OpenCode | ✓ |
| Claude Code | ✓ |
| Codex | ✓ |
| pi | ✓ |
| Antigravity | ✓ |

`git-courer mcp setup` configures all at once. Manual setup and config formats: [docs/mcp-clients.md](docs/mcp-clients.md).

---

## Hooks & golden rules

`mcp setup` does more than register the MCP server — it also injects guardrails so agents route git operations through git-courer instead of raw Bash.

**Golden rules injection.** A `<!-- git-courer start -->` / `<!-- git-courer end -->` block is injected (and kept up to date) in each client's instructions file (`AGENTS.md` for OpenCode, `CLAUDE.md` for Claude Code, `GEMINI.md` for Antigravity). The block encodes the golden rules: check `status` before mutating, run `diff` + `review` before a PR, always `session start` first.

**Hooks.** Clients that support shell hooks get entries wired to git-courer subcommands:

| Event | Matcher | Command | Fires when |
|-------|---------|---------|------------|
| PreToolUse | `git *` | `git-courer hook-check` | Before any Bash `git ...` run |
| SessionStart | — | `git-courer session-start-hook` | Agent session opens |
| SubagentStart | — | `git-courer subagent-start-hook` | A sub-agent starts |
| PreInvocation | — | `git-courer pre-invocation-hook` | Before each model call (Antigravity) |

`hook-check` classifies the command and emits `additionalContext` suggesting the matching git-courer MCP tool — it **never denies**. The session/subagent/pre-invocation hooks inject the golden rules as `additionalContext`. Claude Code uses inline `settings.json` hooks (`UserPromptSubmit` instead of PreInvocation); Codex uses a separate `hooks.json`; Antigravity uses a separate `hooks.json` with a `run_command` matcher and only 2 events. Full reference: [docs/hooks.md](docs/hooks.md).

**OpenCode policy.** For OpenCode (which has no shell hooks), `mcp setup` merges into `opencode.json`:
- `permission.bash["git *"] = "ask"` — OpenCode prompts the user before any `git` Bash command, so the agent is nudged toward the MCP tool.
- `instructions` array includes the `AGENTS.md` path (legacy `GIT_COURER.md` entries are removed). The merge is idempotent; a `.bak` backup is written before any change.

Run `git-courer doctor` to verify all of the above per client.

---

## FAQ

**Who decides the commit type?**
Go. The LLM only writes the message. The agent can override it.

**Do I need a GPU or local LLM?**
No. Flip the toggle (`llm.enabled: false`) and it runs on any laptop. The agent writes messages directly.

**Does my code leave my machine?**
No. Everything runs locally — git-courer, Ollama, your data.

**What about release without an LLM?**
Not available. Release needs an LLM for the changelog.

**How do I mark a breaking change?**
`feat!:` or `BREAKING CHANGE:` in the body. Go detects it automatically.
