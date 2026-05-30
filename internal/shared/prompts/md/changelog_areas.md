JSON only.

{{if .Context}}## Project
{{.Context}}
{{end}}
## Commits by area
{{.Groups}}

## Task
Rewrite each commit into a single user-facing release note, grouped by area.
Write like Linear, Vercel, or Stripe changelogs — short, concrete, specific.
Every item you write MUST be grounded in the commit text. Do not invent or infer capabilities, features, or behaviors that are not explicitly stated or trivially implied by the commit.

## Rules
- One sentence per item, subject implied
- ❌ feat(core): add semantic annotator → ✅ AST-based commit type detection is now more accurate
- ❌ fix(security): nil token → ✅ Fixed a crash when the auth token was missing
- ❌ fix(classifier): false positive refactor → ✅ Commit classification no longer marks unchanged functions as refactors
- ❌ feat!: remove MCP release tool → ✅ The release command has moved to the CLI — run `git-courer release` instead of the MCP tool
- Breaking (!) → explain exactly what the user must change, not just that something changed
- Commits with no description, empty body, or prefixed with wip → skip entirely, do not invent a translation
- If an area has no translatable commits → omit it from output entirely

## Anti-patterns (DO NOT)
- Do NOT add features, capabilities, or behaviors not stated in the commit. If a commit says "simplify", do NOT write "added new feature X"
- Do NOT copy the commit subject verbatim — always rewrite for human readers
- Do NOT use technical jargon (scope prefixes, internal function names) unless it is a user-facing concept
- Do NOT merge or split commits — one output item per input commit
- Do NOT write vague items like "Improved X" or "Updated Y" — say what changed and why it matters

## Output
{"group_1": ["translated change"], "group_2": ["translated change"]}