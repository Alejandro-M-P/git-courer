# Roadmap

Roadmap as issues — each release is a GitHub milestone with a single issue describing scope, motivation, and acceptance criteria. As work progresses, checklist items in the issue get ticked off. When the release ships, the milestone is closed. This file is updated as the roadmap evolves.

> **Last updated:** June 2026 · **Current release:** - **v2.5.0** — moves metadata storage into Git's internal directory (`refs/courer/<branch>`) and implements sidecar sync so changelog history survives squash merges. Includes automatic migration from legacy `.git-courer/` structure. · Questions or feedback → [open a discussion](../../discussions)

---

## Vision

git-courer is the MCP git server that treats code as structure, not text.

Every git operation today returns text and leaves reasoning to the caller. git-courer flips that: it parses ASTs, builds dependency graphs, classifies changes deterministically, and returns structured JSON that any LLM can consume directly.

**Core rule: no AI agent ever runs raw git again.** Every commit, branch, merge, rebase, and release goes through MCP tools that understand semantics, enforce safety, and preserve context across the workflow.

---

## Strategic Pillars

| Pillar | Focus | Key releases |
|---|---|---|
| Agent MCP DX | Make AI agents prefer git-courer tools over bash. Better descriptions, prompt injection, seamless setup. | v2.5.1, v2.6.0 |
| Git workflow completeness | Cover every git operation with structured tooling. Branch from any ref, enrich PR attribution, full cycle. | v2.7.0, v2.8.0 |
| Scale & performance | Large-repo performance, parallel graph operations. | Post-v2 |
| Semantic depth | Deeper AST analysis: `delete:` type detection, cross-file refactoring detection, automatic breaking-change classification. | Post-v2 |

---

## Releases

### Current

#### v2.5.1 — remove dead stackID/stackBranch + polish MCP tool descriptions
**Theme:** Agent MCP DX · **Milestone:** [#1] · **Issue:** [#142] · **Status:** Planned

The prerequisite release. Before any agent-facing features, we clean up dead fields (`stackID`, `stackBranch`) that confuse the data model, and rewrite every MCP tool description so LLMs understand when to call each tool without guessing.

**Acceptance criteria:**
- All references to `stackID` and `stackBranch` removed from codebase and schema
- Every MCP tool description updated and validated against agent call logs
- No regression in existing tool behavior

**Unblocks:** v2.6.0 (better descriptions make prompt rules land immediately), v2.8.0 (`prNumber` replaces `stackID`'s conceptual role).

---

### Next

#### v2.6.0 — installer cleanup: remove IDE auto-config + inject prompt rules into CLI agents
**Theme:** Agent MCP DX · **Milestone:** [#2] · **Issue:** [#143] · **Status:** Draft

The current installer tries to auto-configure IDEs, which creates noise and fails silently in non-standard setups. This release strips that out and instead injects prompt rules directly into CLI agents, so git-courer tools are preferred over raw git without any manual setup from the user.

**Deliverables:**
- IDE auto-config removed from installer
- Prompt rules injected automatically into CLI agent configs on install
- Installer smoke-tested on macOS and Linux

---

#### v2.7.0 — branch CREATE `--from` flag: create branches from any base ref
**Theme:** Git workflow completeness · **Milestone:** [#3] · **Issue:** [#144] · **Status:** Draft

Right now, branch creation assumes the current HEAD as the base. Agents working across multiple branches or starting from a specific tag/commit have no structured way to do this — they fall back to raw git. This release adds a `--from` flag so any ref (branch, tag, commit SHA) can be the base.

**Deliverables:**
- `branch_create` tool accepts an optional `from` parameter (branch name, tag, or commit SHA)
- Defaults to current HEAD if `from` is omitted (no breaking change)
- Structured error if the ref doesn't exist

---

#### v2.8.0 — PR enrichment: attribute commits to PRs in changelog
**Theme:** Git workflow completeness · **Milestone:** [#4] · **Issue:** [#145] · **Status:** Draft

Changelogs today list commits but have no link back to the PR that introduced them. This release adds PR attribution so each commit in the changelog carries its PR number, making it easier to trace what changed and why.

**Deliverables:**
- `prNumber` field added to commit metadata in changelog output
- `stackID`/`stackBranch` fully replaced by `prNumber` (depends on v2.5.1)
- Changelog format updated; existing entries without PR data unaffected

---

## Dependency Graph

```
v2.5.1 (cleanup + descriptions)
  |---> v2.6.0 (installer + prompt rules)     [agent DX pillar]
  |
  |---> v2.7.0 (branch --from)                [independent — no dependency on v2.6.0]
  |
  |---> v2.8.0 (PR enrichment)                [needs v2.5.1 stackID removal]
```

v2.7.0 is independent of v2.6.0 — they can be developed in parallel or in any order after v2.5.1 ships.

---

## Areas of focus (post-v2)

Not yet assigned to a release, but on the radar.

| Priority | Area | Idea | Issue |
|---|---|---|---|
| High | Agent MCP DX | Error message improvement — every error tells the agent what to do next | -- |
| Medium | Agent MCP DX | Tool usage analytics — which tools agents actually call | -- |
| Medium | Semantic depth | `delete:` commit type for deleted files instead of `refactor:`/`chore:` | [#51] |

---

## Completed

See [Releases](../../releases).

**Past releases shipped:**

- **v2.5.0** — moves metadata storage into Git's internal directory (`refs/courer/<branch>`) and implements sidecar sync so changelog history survives squash merges. Includes automatic migration from legacy `.git-courer/` structure.
