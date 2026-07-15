# Architecture

OpenDesk Remote separates the control plane from the data plane.

## Control Plane

- **OpenDesk API**: REST API, authentication, Relay Grants, users, devices,
  groups, access rules, control roles, strategies, relays, builder jobs, logs,
  and audit events.
- **OpenDesk Admin**: web console for administrators.
- **MySQL**: authoritative relational database.
- **Redis**: session/cache/rate-limit foundation plus API readiness PING check.
- **Local Storage**: branding assets, build logs, and artifacts.
- **Builder**: platform runner interfaces and artifact pipeline.

## Data Plane

- **hbbs**: RustDesk-compatible ID/rendezvous service.
- **hbbr relay nodes**: relay traffic service. Production policy requires relay
  authorization through OpenDesk Relay Grants.
- **Region A relay**: `hbbr-relay-a` runs alongside the control plane.
- **Region B/C relays**: extra regional relay nodes heartbeat to Region A API.

## Relay Grant Flow

1. Client attempts direct connection or hole punching without login requirement.
2. When relay is needed, client asks OpenDesk API for a Relay Grant.
3. API checks user/device authorization and access control.
4. API signs a short-lived grant with nonce and allowed relay list.
5. hbbr relay auth hook validates grant through API or local verification.
6. Invalid or missing grants are rejected with:
   `Relay requires OpenDesk login or managed-device authorization`.

## Builder Flow

1. Admin creates a Build Profile with BuildSpec.
2. Branding assets are uploaded to local storage after validation.
3. Build jobs are queued by platform.
4. Platform runner fails clearly when not configured.
5. Artifacts are stored locally with sha256 and size metadata.
