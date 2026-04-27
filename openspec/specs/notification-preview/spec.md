# Notification Preview JSON Specifications

## Overview
Structured JSON preview format for MCP notifications that forces AI agents to display full preview content, including mandatory sections and actionable options.

## Requirements

### REQ-01: structured_preview field presence
The `structured_preview` field MUST be present in all four JSON preview functions:
- `readyJSON` for generic operations
- `commitPlanJSON` for commit-specific previews  
- `releasePlanJSON` for release previews
- `processingJSON` for processing status notifications

The `structured_preview` field MUST contain categorized sections that AI agents MUST display without summarization.

### REQ-02: options field contains real values
The `options` field MUST contain actionable values according to operation type:
- Commit operations: `["Execute", "Regenerate", "Edit manually", "Cancel"]`
- Release operations: `["Execute", "Cancel", "Edit tag name", "View changelog"]`
- Generic operations: `["Continue", "Abort", "View details"]`
- Processing status: empty array `[]`

### REQ-03: reasoning field present in commitPlanJSON
The `commitPlanJSON` MUST include a `reasoning` field that contains the context of why this commit operation is needed (e.g., "User requested commit").

### REQ-04: impact field present in releasePlanJSON and genericSections
The `releasePlanJSON` MUST include an `impact` field with risk level calculation based on warnings.
Generic operations MUST include an impact warning section stating "This operation will modify git state..."

### REQ-05: backward compatibility
All existing JSON fields (`show_to_user`, `status`, `preview`, `messages`, `files`, `hint`) MUST be preserved to maintain backward compatibility with existing AI clients.

### REQ-06: testing coverage minimum 80%
Modified JSON functions MUST have at least 80% test coverage to ensure reliability.

## Scenarios

### Scenario 1: Commit Operation Preview
**Given** a commit operation plan with reasoning
**When** `commitPlanJSON` is called
**Then** the JSON MUST contain:
- `structured_preview` field with sections: Operation, Affected Files, Reasoning
- `reasoning` field with plan reasoning
- `options` array with real commit actions
- All existing backward-compatible fields preserved

### Scenario 2: Release Operation Preview  
**Given** a release intent with changelog and warnings
**When** `releasePlanJSON` is called
**Then** the JSON MUST contain:
- `structured_preview` field with sections: Operation, Changelog Summary, Impact
- `impact` field calculated from warnings
- `options` array with real release actions
- All existing backward-compatible fields preserved

### Scenario构 3: Generic Operation Preview
**Given** a generic git operation
**When** `readyJSON` is called
**Then** the JSON MUST contain:
- `structured_preview` field with sections: Operation, Preview, Impact Warning
- Generic impact warning text
- `options` array with generic actions
- All existing backward-compatible fields preserved

### Scenario 4: Processing Status Notification
**Given** a processing status message
**When** `processingJSON` is called  
**Then** the JSON MUST contain:
- `structured_preview` field with a single "Processing" section
- Empty `options` array
- All existing backward-compatible fields preserved

## Implementation Notes

- Structured preview sections MUST be categorized by type (Operation, Files, Impact, Reasoning, etc.)
- Actions mapping MUST be consistent between `structured_preview.actions` and `options` array
- Backward compatibility is CRITICAL — existing AI clients MUST continue to parse responses
- Testing MUST verify presence of all new fields in each JSON function