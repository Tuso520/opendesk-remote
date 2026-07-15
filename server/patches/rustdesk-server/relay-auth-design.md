# Relay Auth Hook Design

## Goal

Keep hbbs/rendezvous behavior compatible while requiring Relay Grant
authorization for hbbr relay usage.

## Feature Gate

```text
OPENDESK_RELAY_AUTH_REQUIRED=true
```

Compatibility mode can set the value to `false` only for migration or testing.

## Hook Location

The hook must be placed in the hbbr relay handshake or connection establishment
path before relay traffic is accepted. If a valid grant is missing, hbbr rejects
with:

```text
Relay requires OpenDesk login or managed-device authorization
```

## Validation

hbbr should call:

```http
POST /api/v1/relay-grants/validate
```

Validation checks:

- signature
- expiry
- nonce
- target device
- relay allow-list
- revoke status

## Audit

OpenDesk API records validation failures and relay denials in audit and
connection logs.

