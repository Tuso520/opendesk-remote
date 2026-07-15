# Relay Auth

OpenDesk Remote does not require login for basic ID discovery, direct
connection, or hole punching. Relay usage is different: production relay
capacity is shared infrastructure and requires OpenDesk authorization.

## Default

```text
OPENDESK_RELAY_AUTH_REQUIRED=true
```

## Grant Rules

- Valid for 60 seconds to 5 minutes.
- Bound to user or managed device identity.
- Bound to controller device, target device or `target_rustdesk_id`, allowed
  relays, expiry, and nonce.
- Revocable.
- Audited.
- Nonce protected against replay.
- Persisted through the API repository so grant state survives service object
  recreation and can use MySQL in production deployments.

## API

- `POST /api/v1/relay-grants`
- `POST /api/v1/relay-grants/validate`
- `POST /api/v1/relay-grants/{id}/revoke`
- `POST /api/v1/relays/{id}/heartbeat`

Grant issuing through the HTTP route requires an authenticated OpenDesk
session. Browser Admin uses the httpOnly `opendesk_session` cookie; native or
automation clients may use:

```http
Authorization: Bearer <opendesk-session-or-access-token>
```

The API middleware verifies the signed token and binds the request to the
current local administrator. The Relay Grant service still supports explicit
managed-device/internal identity fields for future device authorization flows,
but the public issuing route is protected.

## Relay Rejection Message

`Relay requires OpenDesk login or managed-device authorization`

The API records failed validation attempts as connection logs with
`connection_type=relay` and `status=denied`. Missing grant tokens use
`deny_reason=relay_auth_required`; invalid, expired, replayed, revoked, or
relay-mismatched grants use `deny_reason=invalid_relay_grant` while preserving
the more specific validation reason in log metadata.

## Heartbeat

Relay nodes call `POST /api/v1/relays/{id}/heartbeat` with:

```json
{
  "current_sessions": 4,
  "status": "active"
}
```

The API updates relay health and `last_health_at`.

## Current Implementation Status

The Go API implements authenticated Relay Grant issuing, repository-backed
grant persistence, public validation, revoke, expiry, replay rejection, relay
heartbeat, denied connection log writes, and tests. It also accepts
`target_rustdesk_id` so hbbr can validate `RequestRelay.id` directly.

The open-source rustdesk-server hbbr hook patch is:

- `server/patches/rustdesk-server/0001-opendesk-relay-auth.patch`

It is documented in:

- `server/patches/rustdesk-server/relay-auth-hook-contract.md`

Use `scripts/prepare-rustdesk-server-patch.ps1` to clone upstream and apply the
patch locally.
