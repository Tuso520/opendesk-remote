# Security Policy

OpenDesk Remote is remote access software. Security, consent, auditability, and
deployment transparency are mandatory project constraints.

## Supported Security Boundary

- No hidden installation.
- No covert remote control.
- No bypass of operating system permission prompts.
- No malware evasion, silent control, process disguise, or persistence tricks.
- No default fallback to public RustDesk servers.
- No RustDesk Server Pro closed-source code or assets.

## Reporting Vulnerabilities

Report vulnerabilities privately to the project maintainers before public
disclosure. Include reproduction steps, affected version, impact, and suggested
mitigation when possible.

## Production Defaults

- `OPENDESK_RELAY_AUTH_REQUIRED=true`
- Short-lived Relay Grants
- Token hashes stored instead of raw tokens
- Local storage path traversal protection
- Upload allow-list for branding assets
- Audit logs for security-sensitive operations
- Web Client disabled in first version

