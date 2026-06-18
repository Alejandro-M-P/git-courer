# Roadmap

Roadmap as issues — each release is a GitHub milestone with a single issue describing scope, motivation, and acceptance criteria. As work progresses, checklist items in the issue get ticked off.

> **Last updated:** June 2026 · **Current release:** v2.5.0 — moves metadata storage into Git's internal directory (`refs/courer/<branch>`) and implements sidecar sync so changelog history survives squash merges. Includes automatic migration from old `.git/git-courer/branches/` layout.

---

## Vision

git-courer is the MCP git server that treats code as structure, not text.

Every git operation today returns text and leaves reasoning to the caller. git-courer flips that: it parses ASTs, builds dependency graphs, classifies changes deterministically, and returns structured insights instead.

**Core rule: no AI agent ever runs raw git again.** Every commit, branch, merge, rebase, and release goes through MCP tools that understand semantics, enforce safety, and preserve context across the full development cycle.

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
**Theme:** Agent MCP DX · **Milestone:** [#1](https://github.com/blak0p/git-courer/milestone/1) · **Issue:** [#142](https://github.com/blak0p/git-courer/issues/142) · **Status:** In Progress

The prerequisite release. Before any agent-facing features, we clean up dead fields (`stackID`, `stackBranch`) that confuse the data model, and rewrite every MCP tool description so LLMs understand exactly what each tool does.

**Acceptance criteria:**
- All references to `stackID` and `stackBranch` removed from codebase and schema
- Every MCP tool description updated and validated against agent call logs
- No regression in existing tool behavior

**Unblocks:** v2.6.0 (better descriptions make prompt rules land immediately), v2.8.0 (`prNumber` replaces `stackID`'s conceptual role).

---

### Next

| Release | Theme | Milestone | Issue | Status |
|---------|-------|-----------|-------|--------|
| **v2.5.1a** — remove dead stackID/stackBranch fields | Agent MCP DX | [#1](https://github.com/blak0p/git-courer/milestone/1) | [#142](https://github.com/blak0p/git-courer/issues/142) | In Progress |
| **v2.5.1b** — reorganize MCP tools (merge, simplify, remove) + improve descriptions | Agent MCP DX | [#1](https://github.com/blak0p/git-courer/milestone/1) | [#146](https://github.com/blak0p/git-courer/issues/146) | Planned |

Two parallel patches within the same milestone. **v2.5.1a** cleans up dead `stackID`/`stackBranch` fields that confuse the data model. **v2.5.1b** reduces the MCP tool surface by merging related tools (`amend`/`revert`/`cherry_pick`/`reset` → `undo`, `merge`+`rebase` → `merge-rebase`, `blame` → `history`), removes underused tools (`tag`, `remotes`, `config`, `commit-jobs`), simplifies `commit`/`branch` enums, and rewrites every tool description for LLM clarity.

**Dependency for:** v2.6.0 (better descriptions + fewer tools make prompt rules land immediately), v2.8.0 (v2.5.1a removes `stackBranch`, v2.5.1b cleans up registration).

---

#### v2.7.0 — branch CREATE `--from` flag: create branches from any base ref
**Theme:** Git workflow completeness · **Milestone:** [#3](https://github.com/blak0p/git-courer/milestone/3) · **Issue:** [#144](https://github.com/blak0p/git-courer/issues/144) · **Status:** Draft

| Release | Theme | Milestone | Issue | Status |
|---------|-------|-----------|-------|--------|
| **v2.6.0** — installer cleanup: remove IDE auto-config + inject prompt rules into CLI agents | Agent MCP DX | [#2](https://github.com/blak0p/git-courer/milestone/2) | [#143](https://github.com/blak0p/git-courer/issues/143) | Draft |
| **v2.7.0** — branch CREATE `from` flag: create branches from any base ref | Git workflow completeness | [#3](https://github.com/blak0p/git-courer/milestone/3) | [#144](https://github.com/blak0p/git-courer/issues/144) | Draft |
| **v2.8.0** — PR enrichment: automatic PR attribution via branch matching (`git` mode vendor-agnostic + `gh` mode opt-in) | Git workflow completeness | [#4](https://github.com/blak0p/git-courer/milestone/4) | [#145](https://github.com/blak0p/git-courer/issues/145) | Draft |

**Deliverables:**
- `branch_create` tool accepts an optional `from` parameter (branch name, tag, or commit SHA)
- Defaults to current HEAD if `from` is omitted (no breaking change)
- Structured error if the ref doesn't exist

---

#### v2.8.0 — PR enrichment: attribute commits to PRs in changelog
**Theme:** Git workflow completeness · **Milestone:** [#4](https://github.com/blak0p/git-courer/milestone/4) · **Issue:** [#145](https://github.com/blak0p/git-courer/issues/145) · **Status:** Draft

Changelogs today list commits but have no link back to the PR that introduced them. This release adds PR attribution so each commit in the changelog carries its PR number, making it easier to trace changes back to their context.

**Deliverables:**
- `prNumber` field added to commit metadata in changelog output
- `stackID`/`stackBranch` fully replaced by `prNumber` (depends on v2.5.1)
- Changelog format updated; existing entries without PR data unaffected

---

## Dependency Graph

```
v2.5.1a (remove stackID/stackBranch) ──┐
                                        ├──> v2.6.0 (installer + prompt rules)  [agent DX pillar]
v2.5.1b (reorg tools + descriptions) ──┘
                                        |---> v2.7.0 (branch --from)             [independent]
                                        |
                                         └──> v2.8.0 (PR enrichment)             [needs v2.5.1a]
```

v2.7.0 is independent of v2.6.0 — they can be developed in parallel or in any order after v2.5.1 ships.

---

## Areas of focus (post-v2)

Not yet assigned to a release, but on the radar.

| Priority | Area | Idea | Issue |
|---|---|---|---|
| High | Agent MCP DX | Error message improvement — every error tells the agent what to do next | -- |
| Medium | Agent MCP DX | Tool usage analytics — which tools agents actually call | -- |
| Medium | Semantic depth | `delete:` commit type for deleted files instead of `refactor:`/`chore:` | [#51](https://github.com/blak0p/git-courer/issues/51) |

---

## Completed

See [Releases](https://github.com/blak0p/git-courer/releases).

**Past releases shipped:**

- **v2.5.0** — moves metadata storage into Git's internal directory (`refs/courer/<branch>`) and implements sidecar sync so changelog history survives squash merges. Includes automatic migration from old `.git/git-courer/branches/` layout.
