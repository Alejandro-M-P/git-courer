## Intelligent Changelog Generation
- Removed the rigid 'Areas' configuration to allow for more natural, freeform changelog generation; the LLM now dynamically determines categories based on actual commit content rather than being forced into predefined boxes.
- Empowered the Orchestrator to control commit types and scopes during the APPLY phase; this prevents the AST from incorrectly labeling functional changes (like a fix that introduces a new function) as features, ensuring the changelog reflects the user's true intent.
- Unified the changelog prompt logic and updated the OpenAI adapter to use the new freeform structure, fixing pipeline failures caused by stale prompt keys.

## Project Configuration & Initialization
- Replaced the legacy 'init' command with a new TUI wizard and simplified the configuration process, making it easier for new users to set up their environment without manual configuration errors.
- Added a configurable BaseBranch field to ProjectConfig and implemented auto-detection in the CLI/TUI; this allows the system to accurately compute MergeBase for any trunk branch (main, master, etc.) instead of relying on a hardcoded list.
- Removed the deprecated 'areas' field from project configurations to clean up the configuration schema and align with the new freeform categorization model.

## Stack Management & Release Grouping
- Introduced Stacks v2.2 to solve the problem of fragmented release notes; the system now detects and groups related changes by identifying branches that share a common MergeBase, allowing multiple PRs to be treated as a single logical unit.
- Implemented stack metadata injection into the commit lifecycle, ensuring that stackID and stackBranch are preserved from the handler through to the final release generation.
- Refined the release generation process to use stack-based grouping, which reduces LLM overhead by allowing a single prompt to process an entire group of commits rather than individual entries.

## System Reliability & Git Operations
- Fixed critical pipeline regressions where stack metadata was lost during the transition from PREVIEW to APPLY, and resolved issues where MergeBase resolution failed when a custom BaseBranch was active.
- Improved the reliability of Git plumbing operations by ensuring that 'amend' commands correctly re-stage the .git-courer/ directory and that the commit tree uses the correct parentHash during amendments.
- Enhanced the robustness of the commit history fallback by ensuring pending entries are properly cleaned before falling back to git-history.
- Cleaned up the codebase by removing the abandoned CustomTagMessage feature, which was causing interference during release execution.