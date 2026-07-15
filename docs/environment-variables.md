# Environment Variables

This document is the project-level index for environment variable entrypoints.
It does not replace the concrete `.env.example` files.

## Confirmed Entrypoints

- Local API development: `api/.env.example`
- Region A deployment: `deploy/env/region-a.env.example`
- Relay-region deployment: `deploy/env/relay-region.env.example`
- Admin local API target: `VITE_OPENDESK_API_URL`

There is currently no root-level `.env.example`.

## API Runtime

The API configuration loader reads `.env` from the current working directory
before reading process environment variables. For normal local development:

```powershell
cd api
Copy-Item .env.example .env
```

Core API variables are defined in `api/.env.example`:

- `OPENDESK_ENV`
- `OPENDESK_HTTP_ADDR`
- `OPENDESK_PUBLIC_URL`
- `OPENDESK_API_URL`
- `OPENDESK_MYSQL_DSN`
- `OPENDESK_REDIS_ADDR`
- `OPENDESK_REDIS_PASSWORD`
- `OPENDESK_STORAGE_DRIVER`
- `OPENDESK_STORAGE_LOCAL_PATH`
- `OPENDESK_BRANDING_ASSET_MAX_BYTES`
- `OPENDESK_JWT_SECRET`
- `OPENDESK_RELAY_GRANT_SIGNING_KEY`
- `OPENDESK_RELAY_AUTH_REQUIRED`
- `OPENDESK_INITIAL_ADMIN_EMAIL`
- `OPENDESK_INITIAL_ADMIN_PASSWORD`
- `OPENDESK_SETUP_TOKEN`
- `OPENDESK_BRAND_DEFAULT_NAME`
- `OPENDESK_AUTH_TOKEN_TTL`
- `OPENDESK_RELAY_GRANT_TTL`
- `OPENDESK_CORS_ORIGINS`
- `OPENDESK_RATE_LIMIT_ENABLED`
- `OPENDESK_RATE_LIMIT_REQUESTS`
- `OPENDESK_RATE_LIMIT_WINDOW`

`OPENDESK_INITIAL_ADMIN_EMAIL` and `OPENDESK_INITIAL_ADMIN_PASSWORD` are
optional. When both are set, the API bootstraps the first local administrator
if no users exist. When they are empty, use `GET /api/v1/setup/status` and
`POST /api/v1/setup/admin` for first-run setup. The setup admin endpoint only
accepts loopback requests unless `OPENDESK_SETUP_TOKEN` is set and supplied.

When `OPENDESK_MYSQL_DSN` is empty in development, the API uses the local
in-memory repository path documented in `README.md` and
`docs/development-environment.md`. In production, `OPENDESK_MYSQL_DSN` is
required.

## Builder Worker Variables

The API worker configuration also reads builder-related variables from
`api/.env.example`:

- `OPENDESK_BUILDER_BIN`
- `OPENDESK_BUILDER_WORK_DIR`
- `OPENDESK_BUILDER_SOURCE_DIR`
- `OPENDESK_BUILDER_DRY_RUN`
- `OPENDESK_BUILDER_TIMEOUT`
- `OPENDESK_BUILDER_WINDOWS_COMMAND`
- `OPENDESK_BUILDER_WINDOWS_ARTIFACT_GLOB`

These variables control the API-side worker invocation of `opendesk-builder`.
The builder CLI itself also accepts explicit command-line flags; see
`docs/custom-client-builder.md`.

`OPENDESK_BUILDER_SOURCE_DIR` may point at `.upstream/rustdesk-client`, but
`.upstream/` remains a protected read-only input area for automated agents.

## Admin Variables

The Admin console uses:

- `VITE_OPENDESK_API_URL`

For local PowerShell development:

```powershell
cd admin
$env:VITE_OPENDESK_API_URL='http://127.0.0.1:21114'
npm run dev
```

If `VITE_OPENDESK_API_URL` is not set, the Vite proxy configuration falls back
to `http://localhost:21114` for `/api` during development.

## Deployment Variables

Deployment environment examples are split by role:

- `deploy/env/region-a.env.example` covers the control-plane region, MySQL,
  Redis, storage, initial admin, relay defaults, and image names.
- `deploy/env/relay-region.env.example` covers remote relay region heartbeat
  and Relay Grant validation settings.

Do not modify deployment env examples unless the task explicitly includes
deployment changes.

## 待确认

- Whether this standard product project should add a root-level `.env.example`
  as a unified one-person-company entrypoint.
- If a root-level `.env.example` is added, whether it should be a documentation
  index or a real runtime config file read by project processes.
- Whether relative path variables in `api/.env.example`, such as
  `OPENDESK_BUILDER_WORK_DIR` and `OPENDESK_BUILDER_SOURCE_DIR`, should be
  normalized for launches from different working directories.
