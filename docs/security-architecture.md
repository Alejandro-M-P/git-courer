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
| **Layer 4** | Static | External Static Analyzers | Runs local security tools (`trufflehog` or `gosec`) on the staged files if they are installed and available in `PATH`. Wrapped in a **30-second timeout** (`context.WithTimeout`) so a hung analyzer cannot block the commit indefinitely. |
| **Layer 5** | Static | Content Regex Scan | Performs regex pattern matching on filenames and staged diff content to identify potential AWS, Stripe, Google Cloud, and generic token matches. |
| **Layer 6** | AI | LLM Auditor Verification | Sends the **filtered** diff (see `filterDiff` below) to a paranoid security agent (`VerifySecrets`). If the LLM validates the diff as safe, it can **override low-confidence regex matches** to prevent false positives (see AI Override below). |

---

## Key Security Features

### 🛡️ Memory-First Scanning
The security service operates on the **Git Diff in memory** instead of reading files blindly from disk. This ensures that uncommitted edits or temporary files cannot bypass the check.

### 🧹 filterDiff — Cleaning the Diff Before the LLM Sees It

Before the diff is sent to the LLM auditor (Layer 6), `filterDiff()` (`internal/security/security.go`) strips out chunks that would cause false positives or leak prompt scaffolding:

- **Go test files** (`*_test.go`) — mock keys and fake credentials are expected there.
- **Files under `test/`** — E2E and pipeline fixtures.
- **Files under `internal/shared/prompts/txt/`** — the prompt templates themselves contain token-like strings that regex would flag.

The function splits the diff on the `diff --git ` boundary, inspects each chunk's header, and keeps only the non-excluded chunks. The LLM therefore only audits real production changes.

### 🚫 Test File Exceptions

Go test files (`*_test.go`) receive a **double exemption** — they are skipped in *both* the binary audit and the regex scan:

1. **Binary AI audit (Layer 1)**: the `ShouldUseLLMScan() && !_test.go` guard means `AuditBinaryContent` is never called on test files, so a test fixture containing binary-looking bytes does not trigger a false block.
2. **Regex scan (Layer 5)**: findings whose file ends in `_test.go` or lives under `test/` are filtered out of `filteredFindings` before they can become a block.

This lets developers commit mock keys, fake certificates, and structural tests without being blocked.

### 🤖 AI Override of Low-Confidence Regex Matches

When the LLM auditor (Layer 6) is active and the regex scan (Layer 5) produced findings, the LLM's verdict is used as the tiebreaker:

- **LLM says real secret** → the commit is blocked (`ai_auditor` result).
- **LLM says not a secret** → all regex findings are **cleared** and the commit is unblocked. This is the override: the paranoid AI vouches that the regex matches were false positives, so the low-confidence regex block is discarded rather than left as a warning.

When the LLM is **not** active (`llm.enabled: false` or no LLM configured), any regex finding blocks the commit — there is no override path.

### 🧠 Model Size Gating — `ParseModelSize`

`ParseModelSize()` (`internal/security/model.go`) parses the model name to extract its parameter count and classify it into a size category:

| Parameter count in name | Classification | Example |
|--------------------------|-----------------|---------|
| `>= 14b` | `ModelSizeLarge` | `qwen3.5:14b`, `gemma:26b` |
| `>= 7b` and `< 14b` | `ModelSizeMedium` | `qwen3.5:7b` |
| `< 7b` (or unparseable) | `ModelSizeSmall` | `qwen3.5:0.8b`, `tinyllama` |

The size is stored on the `Service` struct (`modelSize`) and is used to select prompt strategies: larger models get deep-semantic-analysis prompts, while smaller models get strict classification-and-format prompts to stay reliable within their capacity.

> **Note on `ShouldUseLLMScan()`**: this method is currently hardcoded to `return true` ("auto" mode) — all models are sent to the LLM auditor regardless of size. The `modelSize` field still influences which prompt the auditor receives. If a future change gates the LLM scan on `ModelSizeLarge` (the 14B+ threshold), this method is the single switch point.

---

## Verification and Safety

The security architecture is validated through E2E pipeline tests (`test/pipeline/e2e_test.go`), checking that:
- Binary file commits are blocked.
- Blacklisted paths are intercepted.
- Fake keys in normal code are blocked, while identical fake keys in test packages are successfully committed.