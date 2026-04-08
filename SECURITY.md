# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :warning: Beta     |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do NOT** open a public GitHub issue
2. Send a private report via [GitHub Security Advisories](https://github.com/Alejandro-M-P/git-courer/security/advisories)
3. Or contact the maintainer directly

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Security Best Practices

When contributing to git-courer:

- Never commit sensitive data (keys, tokens, credentials)
- Use `go vet` and `gosec` to scan for vulnerabilities
- Validate all user inputs
- Follow the principle of least privilege

## Known Limitations

- Secret detection may have false negatives in edge cases
- Preview commit workflow is still being validated
