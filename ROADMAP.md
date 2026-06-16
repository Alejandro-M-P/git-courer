# Roadmap

> Roadmap as issues — each release is a GitHub milestone with its own issue describing scope, motivation, and acceptance criteria.

## Vision

**git-courer is the MCP git server that treats code as structure, not text.**

Every git operation today returns text and leaves reasoning to the caller. git-courer flips that: it parses ASTs, builds dependency graphs, classifies changes deterministically, and returns structured JSON that any LLM can consume directly.

The north star: **no AI agent ever runs raw git again.** Every commit, branch, merge, rebase, release — all through MCP tools that understand semantics, enforce safety, and preserve context across the workflow.

## Strategic Pillars

| Pillar | Focus | Key releases |
|--------|-------|--------------|
| **Agent MCP DX** | Make AI agents prefer git-courer tools over bash. Better descriptions, prompt injection, seamless setup. | v2.5.1, v2.6.0 |
| **Git workflow completeness** | Cover every git operation with structured tooling. Branch from any ref, enrich PR attribution, full cycle. | v2.7.0, v2.8.0 |
| **Platform reach** | Support more remotes (GitLab, Gitea), more clients, more OS defaults. | Post-v2 |
| **Scale & performance** | Large-repo perf, streaming diffs, parallel graph operations. | Post-v2 |
| **Semantic depth** | Deeper AST analysis: `delete:` type detection, cross-file refactoring detection, automatic breaking-change classification. | Post-v2 |

---

## Current

| Release | Theme | Milestone | Issue | Status |
|---------|-------|-----------|-------|--------|
| **v2.5.1** — remove dead stackID/stackBranch + polish MCP tool descriptions | Agent MCP DX | [#1](https://github.com/blak0p/git-courer/milestone/1) | [#142](https://github.com/blak0p/git-courer/issues/142) | Planned |

This is the **prerequisite** release. Before any agent-facing features, we clean up dead fields (`stackID`, `stackBranch`) that confuse the data model, and rewrite every MCP tool description so LLMs understand *when* to call each tool without guessing.

**Dependency for:** v2.6.0 (better descriptions make prompt rules land immediately), v2.8.0 (removes `stackBranch`, adds `prNumber`).

---

## Next

| Release | Theme | Milestone | Issue | Status |
|---------|-------|-----------|-------|--------|
| **v2.6.0** — installer cleanup: remove IDE auto-config + inject prompt rules into CLI agents | Agent MCP DX | [#2](https://github.com/blak0p/git-courer/milestone/2) | [#143](https://github.com/blak0p/git-courer/issues/143) | Draft |
| **v2.7.0** — branch CREATE `from` flag: create branches from any base ref | Git workflow completeness | [#3](https://github.com/blak0p/git-courer/milestone/3) | [#144](https://github.com/blak0p/git-courer/issues/144) | Draft |
| **v2.8.0** — PR enrichment: automatic PR attribution via branch matching (`git` mode vendor-agnostic + `gh` mode opt-in) | Git workflow completeness | [#4](https://github.com/blak0p/git-courer/milestone/4) | [#145](https://github.com/blak0p/git-courer/issues/145) | Draft |

### Dependency graph

```
v2.5.1 (cleanup + descriptions)
  |---> v2.6.0 (installer + prompt rules)     [agent DX pillar]
  |
  |---> v2.7.0 (branch --from)                [independent -- no dependency on v2.6.0]
  |
   |---> v2.8.0 (PR enrichment)                [needs v2.5.1 stackBranch removal]
```

v2.7.0 is independent of v2.6.0 — they can be developed in parallel or in any order after v2.5.1 ships.

---

## How this works

- Each release is a **GitHub milestone** with a single **issue** describing the full scope
- Each issue explains the problem, what changes, what doesn't, and acceptance criteria
- As work progresses, the issue's checklist items get ticked off
- When the release ships, the milestone is closed
- This file is updated as the roadmap evolves

---

## Backlog / Ideas (post-v2)

These are not yet assigned to a release but are on the radar:

| Area | Idea | Issue |
|------|------|-------|
| Semantic depth | `delete:` commit type for deleted files instead of `refactor:`/`chore:` | [#51](https://github.com/blak0p/git-courer/issues/51) |
| Platform reach | GitLab / Gitea / Bitbucket remote support | -- |
| Platform reach | GitHub Actions integration (auto-release, changelog PRs) | -- |
| Scale | Streaming diff handlers for large repos | -- |
| Scale | Parallel graph operations (commit store, changelog) | -- |
| Agent MCP DX | Tool usage analytics -- which tools agents actually call | -- |
| Agent MCP DX | Error message improvement pass -- every error tells the agent what to do next | -- |

---

## Completed

See [Releases](https://github.com/blak0p/git-courer/releases).

Past releases shipped:
- **v2.5.0** — initial public release with full MCP tool suite, TUI installer, semantic commit pipeline, release automation, dependency graph analysis, and auto-backup.
