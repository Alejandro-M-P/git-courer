This release unifies the MCP API to reduce latency and fixes formatting issues in GitHub release descriptions.

## MCP API Improvements
- Unified the **branch creation** process by allowing a single call to both create and switch branches; this includes automatic **stashing** of uncommitted changes to ensure the operation is fully atomic and prevents work loss.
- Streamlined the **commit preview** workflow by requiring **target paths** upfront, which allows the system to automatically **stage** files and generate a comprehensive plan in a single step, reducing the number of required agent calls.
- Simplified the API surface by removing the standalone **stage ADD** command, as its functionality is now integrated into the commit preview process, reducing unnecessary complexity for agents.

## Release Management
- Fixed a visual bug in GitHub releases where duplicate **markdown headings** (e.g., `## ##`) were appearing; this was resolved by forcing a **plain-text summary** at the start of the changelog and prepending the **tag name** to the release body.