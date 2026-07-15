# OpenDesk Remote

OpenDesk Remote is a fully open source, self-hosted remote desktop suite for
individuals, teams, and enterprises. The repository name is `opendesk-remote`;
the product display name is **OpenDesk Remote**.

The project is an independent effort based on the open source remote desktop
ecosystem. It aims to stay compatible with the RustDesk protocol while adding a
production-grade control plane, relay authorization, access control, strategy
management, audit logging, and custom client builder foundations.

OpenDesk Remote is **not** an official RustDesk project. It does not include
RustDesk official logos, trademarks, RustDesk Server Pro code, closed-source
assets, or Web Client resources.

## Status

This first implementation is a control-plane foundation with real Go service
modules, local admin authentication, database migrations, evaluator tests,
Relay Grant tests, a Vue Admin baseline, deployment files, and patch design
documents. The Windows builder path can now claim queued jobs and dry-run the
client injection pipeline through `opendesk-builder`. Real RustDesk client and
hbbr/hbbs patch integration is intentionally documented under `client/patches`
and `server/patches`; it is not claimed as complete.

## License

OpenDesk Remote is licensed under AGPL-3.0-only. See `LICENSE` and `NOTICE`.

## MVP Capabilities

| Area | Status |
| --- | --- |
| Go API service | Implemented skeleton |
| Health/version endpoints | Implemented |
| MySQL migrations | Implemented |
| MySQL Users/Devices/Relays/Policy/Build/Logs/Settings repository | Implemented and live MySQL smoke passed |
| Redis connection | Implemented with readiness PING check |
| Local storage driver | Implemented |
| Local admin auth/session | Implemented with httpOnly cookie, Bearer session support, server-side session hashes, and logout revocation |
| API tokens | Implemented with hashed storage, one-time raw token return, revoke, audit, and Bearer auth |
| API rate limit middleware | Implemented with in-memory fixed-window limiter and env-based defaults |
| Users/devices/relays models | Implemented |
| Users API | Implemented with list/create/detail/update/disable and audit events |
| Devices API | Implemented with list/create/register/detail/update/disable and audit events |
| Relay management API | Implemented with list/create/update/heartbeat/disable and audit events |
| User Groups / Device Groups / Address Books API | Implemented with list/create baseline |
| Access Rules API | Implemented with list/create/update/delete and audit events |
| Control Roles / Strategies API | Implemented with list/create/update/delete and audit events |
| Logs / Settings API | Implemented with paginated/time/status-filtered login event logs, audit/connection/file-transfer logs, plus General/OIDC/LDAP/SMTP settings sections |
| AccessEvaluator | Implemented with tests |
| ControlRoleEvaluator | Implemented with tests |
| StrategyResolver | Implemented with tests |
| Relay Grant issue/validate/revoke | Implemented with repository-backed state, replay rejection, denied connection logs, and tests |
| BuildSpec validation | Implemented with tests |
| Build Profile / Job API | Implemented with profile CRUD, profile-scoped job queueing, run-next, cancel, retry, worker doctor, and artifact endpoints |
| BuildSpec injection files | Implemented in builder CLI |
| Branding asset upload and validation | Implemented with local storage, sha256, generated filenames, and tests |
| Admin Web | Arco Design Vue baseline with login, Users/Groups/Devices/Address Books/Relays/Policies CRUD, group membership and address book entry management, Logs, General/OIDC/LDAP/SMTP Settings, and Builder queue/run/artifact actions |
| RustDesk server Relay Auth patch | Patch artifact and apply script |
| RustDesk client relay grant patch | Patch artifact and apply script |
| Windows builder runner | Staging, command execution path, queued job worker, environment doctor, and artifact metadata baseline |
| Web Client | Disabled for first version |

## Quick Start

```bash
cd api
cp .env.example .env
go test ./...
go run ./cmd/opendesk-api
```

Without `OPENDESK_MYSQL_DSN`, the API uses an in-memory local development
repository. With `OPENDESK_MYSQL_DSN`, run migrations first:

```bash
go run ./cmd/opendeskctl migrate
```

On this Windows workstation, the documented project-local Go toolchain path is
`.tools/go`; if it is absent, use a temporary toolchain under `.run/` as this
workspace currently does. See `docs/development-environment.md`.

Health:

```bash
curl http://127.0.0.1:21114/api/v1/health
curl http://127.0.0.1:21114/api/v1/version
```

Admin skeleton:

```bash
cd admin
npm install
VITE_OPENDESK_API_URL=http://127.0.0.1:21114 npm run dev
```

On Windows PowerShell:

```powershell
$env:VITE_OPENDESK_API_URL='http://127.0.0.1:21114'
npm run dev
```

Current local development URLs on this workstation:

- API: `http://127.0.0.1:21114/api/v1/health`
- Admin: `http://127.0.0.1:5173`

First-run admin setup:

- If `OPENDESK_INITIAL_ADMIN_EMAIL` and `OPENDESK_INITIAL_ADMIN_PASSWORD` are
  set, the API bootstraps that local administrator when no users exist.
- If they are empty, call `GET /api/v1/setup/status`, then create the first
  admin with `POST /api/v1/setup/admin` from loopback or with
  `OPENDESK_SETUP_TOKEN`.

## Distributed Deployment Overview

Region A contains `opendesk-api`, `opendesk-admin`, `hbbs`, `hbbr-relay-a`,
MySQL, Redis, local storage, and a reverse proxy. Region B and Region C run
additional relay nodes that heartbeat to the Region A API and call Region A for
Relay Grant validation.

See `docs/deployment/distributed.md`.

## Security Defaults

- Relay authorization is required by default: `OPENDESK_RELAY_AUTH_REQUIRED=true`.
- Management API endpoints require an OpenDesk session, except health/version
  and relay validation/heartbeat service paths.
- Direct discovery and hole punching do not require login.
- Relay Grant issuing requires OpenDesk login or managed-device authorization.
- Web Client is disabled in the first version.
- Public RustDesk servers are not used as fallback.
- Terminal, TCP tunnel, and remote configuration modification default to disabled.

## Roadmap

The project is managed as a long-running M0-M6 program. See
`docs/roadmap-m0-m6.md`.

Current workspace status is tracked in `docs/status.md`.

Apply the hbbr Relay Auth patch to open-source rustdesk-server:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare-rustdesk-server-patch.ps1
```

Apply the BuildSpec/Relay Grant patch to open-source RustDesk client:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare-rustdesk-client-patch.ps1
```

1. M0: compliance and control repository.
2. M1: enterprise control plane foundation.
3. M2: Arco Design Admin CRUD surface.
4. M3: real rustdesk-server hbbr Relay Auth hook.
5. M4: real RustDesk client BuildSpec injection and Windows runner.
6. M5: macOS, Android, and iOS builder expansion.
7. M6: production hardening, identity, monitoring, backup, and upgrades.
