# Security Policy

## Defaults

- Relay authorization required by default.
- Public registration disabled by default.
- No implicit default administrator is created. First-run setup requires
  explicit initial admin environment variables, a loopback setup request, or a
  configured setup token while no users exist.
- Web Client disabled by default.
- No public RustDesk fallback.
- Terminal, TCP tunnel, and remote configuration modification disabled by default.
- API rate limiting is enabled by default with an in-memory fixed-window limiter.
  `待确认`: final production thresholds and Redis-backed distributed limiter
  wiring for multi-replica API deployments.

## Token Storage

API tokens are generated as `odrt_` bearer tokens and are shown only once at
creation time. Persistence stores only HMAC-SHA256 token hashes, never raw API
tokens.

Admin sessions are transported with an httpOnly `opendesk_session` cookie, and
the API also accepts signed Bearer session tokens and user-bound Bearer API
tokens for native or automation clients. Session persistence stores only
HMAC-SHA256 token hashes in the `sessions` table, and logout revokes the stored
session hash so an old Bearer session token cannot be reused. Browser code must
not persist session tokens in `localStorage`.

## Upload Safety

Branding uploads must:

- Accept only known image/icon formats.
- Enforce size limits.
- Generate server-side filenames.
- Reject executable content.
- Reject path traversal.
- Record sha256.

## Audit Events

Implemented audit writes currently cover Relay Grant issue/revoke/validation
failure, API token creation/revocation, branding uploads, Build Profile/Job
creation/update/delete, artifact downloads, relay create/update/heartbeat/disable,
user create/update/disable, device create/update/disable, user/device group
changes, authenticated device registration, access rule/control role/strategy
create/update/delete, strategy assignment changes, and settings updates.
First-run setup and environment bootstrap of the initial administrator also
write system audit events.

Successful, failed, and denied local login attempts are written to `login_logs`
with status, email, IP, User-Agent, and failure reason where applicable. The
management Logs API supports pagination plus email, status, and time filters for
login events.

Relay Grant validation failures are also written to `connection_logs` as
`connection_type=relay`, `status=denied`. Missing grants use
`deny_reason=relay_auth_required`; invalid grants use
`deny_reason=invalid_relay_grant` with the precise validation failure in
metadata.

`待确认`: system-scoped API tokens with nullable `user_id` are modeled but not
enabled for authentication.
