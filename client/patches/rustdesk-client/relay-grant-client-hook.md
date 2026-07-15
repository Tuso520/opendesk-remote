# Relay Grant Client Hook

When relay is needed, the client should request:

```http
POST /api/v1/relay-grants
```

The request must use an authenticated user session or managed-device identity.
The request should include `target_rustdesk_id` and the relay name selected by
hbbs/hbbr:

```json
{
  "target_rustdesk_id": "100000001",
  "allowed_relays": ["hbbr-relay-a"],
  "ttl_seconds": 120
}
```

The returned `grant_token` must be placed in `RequestRelay.token` before hbbr
accepts relay traffic. Relay grants are one-time tokens; each hbbr
`RequestRelay` that reaches the relay auth hook needs its own short-lived grant.

The current upstream patch:

- requests a grant before the controlling side connects to hbbr
- writes the grant into `RequestRelay.token`
- requests a grant for the controlled side using its local `Config::get_id()`
- leaves direct and hole-punched paths untouched

If no grant is available, the UI should show:

```text
Relay requires OpenDesk login or managed-device authorization
```

Direct and hole-punched connections must remain usable without login.
