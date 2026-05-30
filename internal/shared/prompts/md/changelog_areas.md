JSON only.

{{if .Context}}## Project
{{.Context}}
{{end}}
## Commits by area
{{.Groups}}

## Task
Translate each commit into user-facing release notes grouped by area.
Write like Linear, Vercel, or Stripe changelogs — concrete, human, specific.

## Rules
- One sentence per item, subject implied
- ❌ feat(core): add semantic annotator → ✅ Commit type detection now uses AST analysis for significantly more accurate results
- ❌ fix(security): nil token → ✅ Fixed a crash that occurred when the auth token was missing
- ❌ fix(classifier): false positive refactor → ✅ Commit classification no longer incorrectly marks unchanged functions as refactors
- ❌ feat!: remove MCP release tool → ✅ The release command has moved to the CLI — run `git-courer release` instead of using the MCP tool
- Breaking (!) → explain exactly what the user must change, not just that something changed
- Commits with no description, empty body, or prefixed with wip → skip entirely, do not invent a translation
- If an area has no translatable commits → omit it from output entirely

## Output
{"group_1": ["translated change"], "group_2": ["translated change"]}