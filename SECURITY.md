# Security Policy

## Supported Versions

We release patches to fix security issues. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **Email**: security@kube-zen.io (preferred)
2. **GitHub Security Advisory**: Use the "Report a vulnerability" button on the repository's Security tab

### What to Include

When reporting a vulnerability, please include:

- Type of vulnerability
- Full paths of source file(s) related to the vulnerability
- Location of the affected code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**: Depends on severity (Critical: ASAP, High: 30 days, Medium: 90 days)

## Security Best Practices

- Keep dependencies up to date
- Run security scans regularly
- Follow secure coding practices
- Review security advisories for dependencies

