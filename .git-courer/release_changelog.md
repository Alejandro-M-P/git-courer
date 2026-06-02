## Core

* Implemented an auto-staging system for the `.git-courer` metadata directory. This is critical because agents often treat this folder as disposable metadata, but by silently committing it we ensure that the information is not lost during squash operations, allowing change history to remain recoverable for changelog generation.

## Installer

* Added support for integrating Pi agents and Antigravity clients, enabling smoother collaboration between the MCP server and these automation tools.

## MCP

* Improved staging area management to prevent file conflicts and ensure the metadata directory is properly cleaned after each commit is processed, keeping repositories clean and free of artifacts from previous operations.

## General

* Added support for log ranges, allowing developers to inspect specific periods of history without navigating the entire commit log.
* Optimized changelog generation and version analysis, making release note creation faster and more accurate.
* Improved duplicate detection and release branch aggregation to ensure each version contains exactly the changes that belong to it, without matching errors.
* Migrated commit storage to a JSON array format and added a branch listing command, making repository structure management and inspection easier.
* Updated documentation instructions to include support for the new Pi and Antigravity clients, as well as the new branch-related commands.
* Optimized the build process for Windows users, ensuring CGO dependencies and the Rust toolchain work correctly without manual configuration issues.
* Antigravity clients can now be configured in both CLI and IDE environments. To simplify the experience, the TUI interface only displays the Antigravity installation option.
