# Proposal: fix-mcp-notification-preview-json

## Intent

Fix the MCP notification previews that currently show "nothing" because AI agents ignore the `show_to_user` text field and make summaries instead. The root problem is inconsistent JSON formats across four preview functions, dummy `options` array, and insufficient metadata that fails to force AI tooling to display the full structured preview to users.

## Scope

### In Scope
- Modify all JSON preview functions in `internal/delivery/mcp/handlers.go`
-) `readyJSON(preview)` → generic operations preview
-) `commitPlanJSON(plan)` → commit-specific preview  
-) `releasePlanJSON(intent, changelog)` → release preview
-) `processingJSON(message)` → processing status notification
- Add mandatory `structured_preview` field that requires AI to display sections
- Implement real `options` with actionable values for each operation type
- Enhance metadata fields: `reasoning`, `impact_indicators`, `risk_level`
- Maintain JSON backward compatibility — existing AI clients must still parse responses

### Out of Scope
- Changes to MCP tool definitions or protocol
- Modifications to domain models (`OperationPlan`, `ReleaseIntent`)
- GitHub API integration changes
- Security or secret detection changes
- Preview generation logic (only JSON serialization format)

## Capabilities

### New Capabilities
- `mcp-notification-preview`: Structured JSON preview format that forces AI agents to display full preview content, including mandatory sections and actionable options.

### Modified Capabilities
- `mcp-commit-operation`: Requires modified preview JSON format to include structured_preview and real options.
- `mcp-release-operation`: Requires modified preview JSON format to include structured_preview and risk indicators.
- `mcp-branch-operation`: (Implied) Branch operations use `readyJSON` — will inherit new structured format.

## Approach

Introduce a **mandatory structured_preview field** in each JSON response that contains sections the AI MUST display:

```
structured_preview: {
  "summary": "One-line human-readable summary",
  "sections": [
    {"title": "Operation", "content": "Commit all changes"},
    {"title": "Affected Files", "content": "5 files changed"},
    {"title": "Impact", "content": "Low risk"},
    {"title": "Reasoning", "content": "Why this operation was triggered"}
  ]
}
```

Real `options` per operation type:
- Commit: `["APPLY", "REGENERATE_MESSAGE", "ABORT", "STAGE_SELECTION"]`
- Release: `["APPLY", "ABORT", "EDIT_TAG_NAME", "VIEW_CHANGELOG"]`
- Generic: `["CONTINUE", "ABORT", "VIEW_DETAILS"]`

Add metadata fields:
- `reasoning`: Context why this operation is needed (e.g., "User requested commit")
- `impact_indicators`: `["LOW_RISK", "MEDIUM_RISK", "HIGH_RISK"]`
- `risk_level`: calculated risk per operation type

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/delivery/mcp/handlers.go` | Modified | All JSON preview functions updated |
| `internal/delivery/mcp/handlers_test.go` | Modified | Test cases for new JSON structure |
| MCP clients (Opencode, Claude, Cursor) | Indirect | Benefit from better preview display |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| JSON backward compatibility break | Low | Keep existing fields unchanged, add new fields only |
| AI clients ignore new fields as they ignore `show_to_user` | Medium | Design `structured_preview` to be machine-parseable with clear sections; include example in documentation |
| Performance impact from additional JSON size | Low | Minimal extra fields (tens of bytes) |
| Testing coverage gaps | Medium | Add unit tests for each preview function with new fields |

## Rollback Plan

If new JSON format causes AI integration failures:
1. Revert all JSON function changes in `handlers.go`
2. Revert test changes
3. Tag release as `mcp-preview-revert-{date}`
4. Fallback to original `show_to_user` text-only approach with improved wording

## Dependencies

- None external
- Internal: requires understanding of existing AI client behavior (observed from exploration)

## Success Criteria

- [ ] All four JSON preview functions include `structured_preview` field
- [ ] Each operation type has real, actionable `options` array
- [ ] AI clients display full previews without summarization (confirmed via testing)
- [ ] All existing unit tests pass with new fields
- [ ] No breaking changes to existing MCP clients