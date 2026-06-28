# Security Architecture

git-courer implements a **6-Layer Proactive Security Audit** strategy to prevent sensitive data leaks, credentials exposure, and unauthorized binary uploads.

---

## Defense Layers

The security service (`internal/security/security.go`) audits all staged files and memory diffs sequentially before any commit plan is shown to the user or applied:

| Layer | Type | Mechanism | Purpose |
| :--- | :--- | :--- | :--- |
| **Layer 1** | Binary | Magic Bytes + AI Audit | Checks file headers for executables (ELF, PE, Mach-O) using `IsBinary` and runs an LLM binary noise check (`AuditBinaryContent`) for suspicious text files. |
| **Layer 2** | Metadata | Folder Blacklist | Blocks staging changes in system directories (`.git/`, `.github/`, `vendor/`, `test/`). |
| **Layer 3** | Metadata | Filename Blacklist | Prevents tracking of credential files (`.env`, `credentials.json`, `id_rsa`, `auth.key`). |
| **Layer 4** | Static | External Static Analyzers | Runs local security tools (like `trufflehog` or `gosec`) on the staged files if they are installed and available in the system path. |
| **Layer 5** | Static | Content Regex Scan | Performs regex pattern matching on filenames and staged diff content to identify potential AWS, Stripe, Google Cloud, and generic token matches. |
| **Layer 6** | AI | LLM Auditor Verification | Sends the cleaned diff to a paranoid security agent (`VerifySecrets`). If the LLM validates the diff as safe, it can override low-confidence regex matches to prevent false positives. |

---

## Key Security Features

### 🛡️ Memory-First Scanning
The security service operates on the **Git Diff in memory** instead of reading files blindly from disk. This ensures that uncommitted edits or temporary files cannot bypass the check.

### 🚫 Test File Exceptions
Go test files (`*_test.go`) and files under the `test/` directory are explicitly exempted from regex secret blocks. This allows developers to commit mock keys, fake certificates, or structural tests without trigger blocks.

### 🧠 Model-Agnostic prompts
Refined "accuracy-first" system instructions ensure that the paranoid AI auditor works reliably across different model sizes:
- **Large Models (26B+)**: Perform a deep semantic analysis of the code context to determine if a matched string is a real credential or structural code.
- **Small Models (<1B)**: Adhere strictly to classification instructions and output format validations.

---

## Verification and Safety

The security architecture is validated through E2E pipeline tests (`test/pipeline/e2e_test.go`), checking that:
- Binary file commits are blocked.
- Blacklisted paths are intercepted.
- Fake keys in normal code are blocked, while identical fake keys in test packages are successfully committed.
