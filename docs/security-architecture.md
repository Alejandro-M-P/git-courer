
# Security-First Architecture

git-courer follows a **Multi-Layered Proactive Defense** strategy to prevent sensitive data leaks and malicious commits.

## Defense Layers

| Layer | Type | Mechanism | Goal |
| :--- | :--- | :--- | :--- |
| **Layer 1** | Binary | Magic Bytes Header Scan | Block executables (ELF, PE, Mach-O) regardless of extension. |
| **Layer 2** | Binary | Statistical Null-Byte Detection | Identify disguised binary payloads in text files. |
| **Layer 3** | Metadata | Filename/Path Blacklist | Block `.env`, `credentials.json`, `id_rsa`, etc. |
| **Layer 4** | Static | Content-Based Regex Scan | Detect known patterns (AWS, Stripe, Google) in the staged diff. |
| **Layer 5** | AI | LLM Proactive Audit | A "Paranoid Security Auditor" agent scans the diff for unknown secrets. |
| **Layer 6** | Human | Explicit Confirmation | A final preview is shown to the user before any execution. |

## Key Security Features

### 🛡️ Memory-First Scanning
We don't just trust the files on disk. The security engine audits the **Git Diff in memory** before it's committed. If a secret is fragmental or obfuscated, the AI layer attempts to reconstruct the threat.

### 🧠 Model-Agnostic Intelligence
Thanks to highly refined "Accuracy-First" prompts, the security audit works reliably across models:
- **Large Models (26B+)**: Context-aware deep audit and complex intent extraction.
- **Small Models (<1B)**: Pattern detection and JSON-strict response stability.

### 🚫 Social Engineering Protection
The AI Auditor is explicitly instructed to ignore in-code comments like `// This is a test key`. It treats any match as a real threat unless proven otherwise.

## Torture Tested
Our architecture is continuously verified by the **Torture Tests**, which simulate:
- Command injection attempts.
- Fragmented secrets across multiple lines.
- Disguised binaries.
- Context window overflow attacks (10,000+ lines).
