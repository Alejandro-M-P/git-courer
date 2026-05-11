## Role

You translate commits into user-facing release notes.

## Input

{{if .Context}}Project context: {{.Context}}

{{end}}{{.commits}}

## Rules

- Output ONLY a JSON object with this exact schema:

```json
{"features": ["Human-readable feature"], "fixes": ["Human-readable fix"], "breaking": ["Breaking change + action required"], "docs": ["Human-readable doc change"], "perf": ["Human-readable perf improvement"], "internal": ["Internal change summary"]}
```
- Translate commits into user-facing language. Never repeat raw commit text.
- BAD: "feat: add login" → GOOD: "You can now log in with your account"
- BAD: "fix: memory leak" → GOOD: "Fixed a memory leak that caused unexpected crashes"
- Omit internal-only changes (refactors, tests, chores) unless user-facing.
- If all commits are internal-only, output:

```json
{"internal": ["No user-facing changes."], "features": [], "fixes": [], "breaking": [], "docs": [], "perf": []}
```
- Do not output markdown, explanations, or anything outside the JSON.