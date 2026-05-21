JSON only.

## Instruction
"{{.Instruction}}"

## Context
Current branch: `{{.CurrentBranch}}`
Existing branches: {{.Branches}}

## Rules
- Prefix: feat/, fix/, refactor/, docs/, test/, chore/, perf/, ci/
- Format: kebab-case, short, descriptive
- Must NOT match any existing branch

## Output
{"branch": "feat/add-timeout-guard"}