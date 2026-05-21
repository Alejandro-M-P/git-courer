JSON only.

{{if .Context}}## Project
{{.Context}}
{{end}}
## Commits by area
{{.Groups}}

## Task
Translate each commit into user-facing language grouped by area.

## Rules
- ❌ feat(core): add semantic annotator → ✅ Added semantic diff analysis for more accurate commits
- ❌ fix(security): nil token → ✅ Fixed a crash when the auth token was missing
- Breaking (!) → explain what callers must update
- One sentence per item
- Only include areas with user-facing changes

## Output
{"group_1": ["translated change"], "group_2": ["translated change"]}