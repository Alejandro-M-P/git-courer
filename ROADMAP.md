# Roadmap

Roadmap as issues — each feature is a GitHub issue describing scope, motivation, and acceptance criteria. No fixed versions or milestones — we ship when ready.

> **Last updated:** June 27, 2026 · **Current release:** v2.5.0

---

## Vision

git-courer is the MCP git server that treats code as structure, not text.

Every git operation today returns text and leaves reasoning to the caller. git-courer flips that: it parses ASTs, builds dependency graphs, classifies changes deterministically, and returns structured insights instead.

**Core rule: no AI agent ever runs raw git again.** Every commit, branch, merge, rebase, and release goes through MCP tools that understand semantics, enforce safety, and preserve context across the full development cycle.

---

## Strategic Pillars

| Pillar | Focus | Issues |
|--------|-------|--------|
| Agent MCP DX | Make AI agents prefer git-courer tools over bash. Better descriptions, prompt injection, seamless setup. | [#151](https://github.com/blak0p/git-courer/issues/151), [#143](https://github.com/blak0p/git-courer/issues/143), [#152](https://github.com/blak0p/git-courer/issues/152) |
| Git workflow completeness | Cover every git operation with structured tooling. Branch from any ref, enrich PR attribution, full cycle. | [#144](https://github.com/blak0p/git-courer/issues/144), [#145](https://github.com/blak0p/git-courer/issues/145) |
| Semantic depth | Deeper AST analysis: `delete:` type detection, cross-file refactoring detection, automatic breaking-change classification. | [#51](https://github.com/blak0p/git-courer/issues/51) |
| Scale & performance | Large-repo performance, parallel graph operations. | Post-v2 |

---

## Issues

| Issue | Theme | Status |
|-------|-------|--------|
| [#51](https://github.com/blak0p/git-courer/issues/51) | `delete:` commit type for deleted files | `good first issue` |
| [#143](https://github.com/blak0p/git-courer/issues/143) | Remove IDE apps from MCP auto-config | Draft |
| [#144](https://github.com/blak0p/git-courer/issues/144) | Branch CREATE `from` parameter | Draft |
| [#145](https://github.com/blak0p/git-courer/issues/145) | PR enrichment: attribute commits to PRs in changelog | Draft |
| [#151](https://github.com/blak0p/git-courer/issues/151) | Interactive release wizard | Draft |
| [#152](https://github.com/blak0p/git-courer/issues/152) | Inject prompt rules into CLI agents | Draft |
| [#180](https://github.com/blak0p/git-courer/issues/180) | Backup auto-prune (reachability-based) | Draft |

All issues are independent — no blockers between them. Implement in any order.

---

## Completed

See [Releases](https://github.com/blak0p/git-courer/releases).

- **v2.5.0** — moves metadata storage into Git's internal directory (`refs/courer/<branch>`) and implements sidecar sync so changelog history survives squash merges. Includes automatic migration from old `.git/git-courer/branches/` layout.
