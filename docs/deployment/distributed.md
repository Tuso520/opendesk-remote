# Distributed Deployment

OpenDesk Remote uses distributed deployment as the production reference
architecture. Region A is authoritative for the control plane, database,
Redis, local storage, Admin UI, hbbs, and the first hbbr relay. Region B and
Region C are relay-only regions.

## Region A

Region A is the control plane and the first relay region.

Services:

- `opendesk-api`
- `opendesk-admin`
- `hbbs`
- `hbbr-relay-a`
- `mysql`
- `redis`
- local storage volume
- reverse proxy

Region A owns:

- MySQL schema and migrations.
- Redis session/cache/rate-limit foundation and API readiness PING check.
- Local filesystem storage for branding assets, build logs, and artifacts.
- Relay Grant issuing and validation API.
- Reverse proxy routes for API/Admin and hbbs/hbbr WebSocket ports.

## Region B and Region C

Region B/C run relay-only services:

- `hbbr-relay-b`
- `hbbr-relay-c`

Relay regions heartbeat to Region A API and use Region A for Relay Grant
validation.

Each relay region must use a stable `OPENDESK_RELAY_NAME` that matches the
grant allow-list. Recommended names:

- Region A: `hbbr-relay-a`
- Region B: `hbbr-relay-b`
- Region C: `hbbr-relay-c`

Relay heartbeat contract:

```http
POST /api/v1/relays/{id}/heartbeat
```

```json
{
  "current_sessions": 4,
  "status": "active"
}
```

Relay grant validation contract:

```http
POST /api/v1/relay-grants/validate
```

hbbr must fail closed when relay auth is required and validation cannot be
completed.

## Environment Variables

Use the concrete examples instead of real secrets:

- Region A: `deploy/env/region-a.env.example`
- Relay regions: `deploy/env/relay-region.env.example`

Region A must define the API, MySQL, Redis, storage, initial admin, and relay
auth defaults:

- `OPENDESK_ENV=production`
- `OPENDESK_HTTP_ADDR=:21114`
- `OPENDESK_PUBLIC_URL`
- `OPENDESK_API_URL`
- `OPENDESK_MYSQL_DSN`
- `OPENDESK_REDIS_ADDR`
- `OPENDESK_REDIS_PASSWORD`
- `OPENDESK_STORAGE_DRIVER=local`
- `OPENDESK_STORAGE_LOCAL_PATH`
- `OPENDESK_JWT_SECRET`
- `OPENDESK_RELAY_GRANT_SIGNING_KEY`
- `OPENDESK_RELAY_AUTH_REQUIRED=true`
- `OPENDESK_INITIAL_ADMIN_EMAIL`
- `OPENDESK_INITIAL_ADMIN_PASSWORD`
- `OPENDESK_SETUP_TOKEN`
- `OPENDESK_BRAND_DEFAULT_NAME=OpenDesk Remote`
- `OPENDESK_BRANDING_ASSET_MAX_BYTES`
- `OPENDESK_CORS_ORIGINS`
- `OPENDESK_RATE_LIMIT_ENABLED`
- `OPENDESK_RATE_LIMIT_REQUESTS`
- `OPENDESK_RATE_LIMIT_WINDOW`

Relay-only regions must point back to Region A:

- `OPENDESK_REGION`
- `OPENDESK_RELAY_NAME`
- `OPENDESK_RELAY_HOST`
- `OPENDESK_REGION_A_API_URL`
- `OPENDESK_RELAY_HEARTBEAT_URL`
- `OPENDESK_RELAY_VALIDATE_URL`
- `OPENDESK_RELAY_AUTH_REQUIRED=true`

`待确认`: the final production secret rotation procedure for
`OPENDESK_JWT_SECRET` and `OPENDESK_RELAY_GRANT_SIGNING_KEY`.

## Ports

| Port | Service |
| --- | --- |
| 21114 | Web Console / API behind reverse proxy |
| 21115 | hbbs TCP |
| 21116 | hbbs TCP/UDP |
| 21117 | hbbr TCP |
| 21118 | hbbs WebSocket |
| 21119 | hbbr WebSocket |
| 80/443 | reverse proxy |

## Startup Order

1. MySQL.
2. Redis.
3. OpenDesk API migrations.
4. OpenDesk API.
5. Admin.
6. hbbs.
7. Region A/B/C relay nodes.
8. Reverse proxy.

## Backup and Restore

Back up MySQL and Region A local storage. Relay-only regions do not hold the
authoritative database.

Minimum backup set:

- MySQL logical dump or physical backup.
- Region A local storage directory.
- Deployment env files stored in a secure secret manager or encrypted backup.
- Reverse proxy configuration.

Restore order:

1. Restore MySQL.
2. Restore Region A local storage to `OPENDESK_STORAGE_LOCAL_PATH`.
3. Restore Region A environment and reverse proxy config.
4. Run `opendeskctl migrate` before starting a newer API image.
5. Start Region A API/Admin/hbbs/hbbr.
6. Start relay-only regions and verify heartbeat.

## Upgrade Notes

- Apply database migrations before starting a newer API service.
- Keep client/server RustDesk patchsets small and review them before upstream
  upgrades.
- Run API unit tests, migration tests, Relay Grant tests, evaluator tests,
  builder BuildSpec validation tests, Admin build, and Compose config parsing.
- Do not enable Web Client assets in the first version.
- Keep `OPENDESK_RELAY_AUTH_REQUIRED=true` in production unless a documented
  migration window explicitly requires compatibility mode.
- Verify Region A still contains `hbbr-relay-a`; Region A must not become only
  a control plane.
