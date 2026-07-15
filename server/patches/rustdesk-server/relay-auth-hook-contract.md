# hbbr Relay Auth Hook Contract

This contract defines the boundary the future rustdesk-server `hbbr` patch must
use. It is intentionally small so the upstream merge can remain easy to audit.

## Environment

```text
OPENDESK_RELAY_AUTH_REQUIRED=true
OPENDESK_RELAY_VALIDATE_URL=https://remote.example.com/api/v1/relay-grants/validate
OPENDESK_RELAY_HEARTBEAT_URL=https://remote.example.com/api/v1/relays/1/heartbeat
OPENDESK_RELAY_NAME=hbbr-relay-a
```

## Handshake Hook

Before accepting relay traffic, hbbr must extract the OpenDesk relay grant token
from `RequestRelay.token` and validate it with the control plane. The target is
sent as `RequestRelay.id` using `target_rustdesk_id`.

```http
POST /api/v1/relay-grants/validate
Content-Type: application/json
```

```json
{
  "grant_token": "opaque-token",
  "relay": "hbbr-relay-a",
  "target_rustdesk_id": "100000001"
}
```

Expected successful response:

```json
{
  "data": {
    "valid": true,
    "grant_id": "rg_example",
    "status": "used",
    "expires_at": "2026-07-07T12:30:00Z"
  }
}
```

When validation fails, hbbr must reject relay setup with:

```text
Relay requires OpenDesk login or managed-device authorization
```

## Heartbeat Hook

hbbr must report health on startup and periodically while running.

```http
POST /api/v1/relays/{id}/heartbeat
Content-Type: application/json
```

```json
{
  "current_sessions": 4,
  "status": "active"
}
```

The API responds with the relay record and updates `last_health_at`.

## Patch Artifact

The current patch against open-source `rustdesk-server` master is:

```text
server/patches/rustdesk-server/0001-opendesk-relay-auth.patch
```

Apply it with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare-rustdesk-server-patch.ps1
```

## Compatibility Rules

- Do not change hbbs rendezvous behavior.
- Do not require login for direct discovery or hole punching.
- Only relay setup is gated by OpenDesk authorization.
- Keep the patch feature-gated with `OPENDESK_RELAY_AUTH_REQUIRED`.
- If the validation API is unreachable and relay auth is required, fail closed.
