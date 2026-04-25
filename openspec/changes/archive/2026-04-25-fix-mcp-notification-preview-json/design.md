# Design: fix-mcp-notification-preview-json

## Technical Approach

Implement structured JSON previews with mandatory sections that force AI agents to display all preview content instead of summarization. The approach modifies four existing JSON serialization functions (`processingJSON`, `readyJSON`, `commitPlanJSON`, `releasePlanJSON`) to include a `structured_preview` field containing categorized sections and actionable options, while maintaining full backward compatibility with existing AI clients.

## Architecture Decisions

### Decision: Structured Preview Schema Location

**Choice**: Create new file `internal/delivery/mcp/helpers.go` for type definitions
**Alternatives considered**: 
- Embed types in `handlers.go` (would bloat the main handler file)
- Add to `domain` package (not semantically correct — these are presentation types, not domain models)
**Rationale**: Keep handler code focused on logic, separate type definitions for clarity and reusability. Follows project pattern of helper files for auxiliary types.

### Decision: Backward Compatibility Strategy

**Choice**: Add new fields (`structured_preview`, `options`, `impact`, `risk_level`) alongside existing fields (`show_to_user`, `status`, `preview`, `messages`, `files`, `hint`)
**Alternatives considered**:
- Replace existing fields with new structure (would break existing AI clients)
- Create entirely new JSON functions (would require changing all call sites)
**Rationale**: Existing AI clients ignore unknown fields — safe to add. All current callers continue working unchanged. This is a non-breaking enhancement.

### Decision: Options → Actions Mapping

**Choice**: `options` array contains human-readable action labels that map to `structured_preview.actions` keys
**Alternatives considered**:
- Remove `options` field entirely (some tests expect it)
- Keep dummy `options` array unchanged (doesn't solve the problem)
**Rationale**: Real options provide actionable choices for users and AI agents. Mapping between labels and keys ensures consistency across the JSON structure.

### Decision: Section Types per Operation

**Choice**: Define operation-specific section templates in helper functions, not hardcoded in each JSON function
**Alternatives considered**:
- Hardcode sections in each of the 4 JSON functions (duplication, harder to maintain)
- Central template registry in a separate package (over-engineering for this scope)
**Rationale**: Helper functions per operation type (`commitSections`, `releaseSections`, `genericSections`) keep logic DRY while staying within the `mcp` delivery layer.

## Data Flow

```
AI Agent ──→ MCP Tool (git_write_review)
                    │
                    ▼
           handlers.go:handleGitWriteReview
                    │
                    ▼
         (operation-specific handler)
                    │
                    ▼
        JSON preview function (e.g., commitPlanJSON)
                    │
                    ▼
        JSON with structured_preview + options
                    │
                    ▼
           AI Agent MUST display sections
                    │
                    ▼
           User sees full preview → confirms
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/delivery/mcp/helpers.go` | Create | Defines `StructuredPreview`, `PreviewSection`, `Action` types and section builders |
| `internal/delivery/mcp/handlers.go` | Modify | Update all 4 JSON functions to include `structured_preview` field and real `options` |
| `internal/delivery/mcp/handlers_preview_test.go` | Modify | Update tests to verify new fields and options mapping |
| `internal/delivery/mcp/handlers_test.go` | Modify (if applicable) | Ensure existing integration tests pass |

## Interfaces / Contracts

```go
// in helpers.go
type StructuredPreview struct {
    Summary  string          `json:"summary"`
    Sections []PreviewSection `json:"sections"`
    Actions  []Action        `json:"actions"`
}

type PreviewSection struct {
    Title   string `json:"title"`
    Content string `json:"content"`
    Type    string `json:"type,omitempty"` // "text", "list", "code", "warning"
}

type Action struct {
    Label string `json:"label"` // Human-readable (e.g., "Execute")
    Key   string `json:"key"`   // Machine-actionable (e.g., "apply")
}

// Helper functions
func commitSections(plan *domain.OperationPlan) []PreviewSection
func releaseSections(intent *domain.ReleaseIntent, changelog string, warnings []string, ghAuth string) []PreviewSection
func genericSections(operation, preview string) []PreviewSection
func processingSections(message string) []PreviewSection

// Action builders
func commitActions() []Action        // ["Execute", "Regenerate", "Edit manually", "Cancel"]
func releaseActions() []Action       // ["Execute", "Cancel", "Edit tag", "View changelog"]
func genericActions() []Action       // ["Continue", "Cancel", "View details"]
func processingActions() []Action    // [] (none — informational only)
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Each JSON function includes `structured_preview` | Test that marshaled JSON contains new field with correct schema |
| Unit | `options` array matches `structured_preview.actions` labels | Verify label-key mapping consistency |
| Unit | Section content per operation type | Test `commitSections`, `releaseSections`, etc. produce expected sections |
| Regression | Existing fields unchanged (`show_to_user`, `status`, `preview`) | Ensure backward compatibility |
| Integration | AI client simulation (if possible) | Manual verification that AI displays full sections |

## Migration / Rollout

No migration required — this is an additive change. Existing AI clients will ignore new fields, new AI clients will benefit from structured previews. Rollout is immediate upon deployment.

## Open Questions

- [ ] Should `structured_preview` be required for all JSON responses or optional? (Decision: required — forces AI to parse it)
- [ ] How to handle extremely long section content that might exceed AI context windows? (Decision: trim content to max 200 chars per section)
- [ ] Should we add a version field to indicate new JSON format? (Decision: not needed — forward compatibility sufficient)