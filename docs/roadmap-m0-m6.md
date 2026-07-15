# M0-M6 Roadmap

OpenDesk Remote is a long-running project. Work is planned as milestone gates,
not as a single one-shot delivery. Each milestone must leave behind code,
tests, documentation, and an honest status report.

## M0: Compliance and Control Repository

Goal: establish the legal, security, and architectural foundation.

Deliverables:

- AGPL-3.0 license and NOTICE.
- README, SECURITY, CONTRIBUTING, CODE_OF_CONDUCT.
- Misuse policy and security policy.
- Architecture, decisions, product requirements, upgrade strategy.
- Client/server patch directories with explicit "design only" status.
- Web Client disabled by decision.

Exit gate:

- No RustDesk official branding assets.
- No RustDesk Server Pro code or proprietary resource.
- Product naming is consistently `OpenDesk Remote` / `opendesk-remote`.

## M1: Enterprise Control Plane Foundation

Goal: create a production-shaped API foundation.

Deliverables:

- Go API service.
- MySQL migration set.
- Redis and local storage configuration.
- Health/version endpoints.
- Local administrator login/session endpoints and auth middleware.
- User, device, group, relay, session, token, audit, connection, file transfer,
  strategy, access, control role, build profile/job/artifact models.
- Relay Grant issue/validate/revoke logic.
- AccessEvaluator, ControlRoleEvaluator, StrategyResolver.
- BuildSpec validation and asset safety validation.
- OpenAPI skeleton.
- Unit and basic integration tests.

Exit gate:

- `go test ./...` passes in an environment with Go installed.
- `/api/v1/health` returns OK.
- Relay auth defaults to required.

## M2: Admin Console and CRUD Surface

Goal: turn the Arco Design admin skeleton into usable management screens.

Deliverables:

- Login flow.
- Users and groups CRUD.
- Devices and device groups CRUD.
- Individual user/device group membership and address book entry management.
- Relays CRUD and heartbeat display.
- Access Control editor and evaluator preview.
- Control Role editor.
- Strategy editor and effective strategy preview.
- Logs pages with baseline tables and query filters.
- Base system Settings editor.

Exit gate:

- Admin build passes.
- All pages have loading, empty, error, and permission states.

## M3: Relay Auth Server Integration

Goal: connect real rustdesk-server hbbr relay auth.

Deliverables:

- Feature-gated hbbr RelayAuthProvider patch.
- API validation callback integration.
- Relay denial connection logs.
- Relay heartbeat path.
- Compatibility mode for migration only.

Exit gate:

- Direct/hole punching remains compatible.
- Relay without grant is rejected with the required clear message.
- Relay with valid short grant succeeds.

## M4: Custom Client Builder Integration

Goal: connect real RustDesk client fork build customization.

Deliverables:

- BuildSpec injection patch.
- Branding asset injection.
- Built-in ID Server, Relay Server, API Server, Key, WebSocket settings.
- Windows runner first.
- Artifact storage and download authorization.

Exit gate:

- Unconfigured runner fails clearly.
- Windows runner can produce a branded artifact in CI or a documented runner.

## M5: Multi-Platform Production Builder

Goal: extend builder across macOS, Android, and iOS.

Deliverables:

- macOS x64/arm64 runner.
- Android arm64 runner.
- iOS runner interface with signing model.
- iOS capability matrix verified against real workspace capability.
- Build logs, retries, cancellation, artifact retention.

Exit gate:

- Platform status is truthful in Admin and docs.
- iOS does not promise unverified controlled-device capability.

## M6: Production Hardening and Operations

Goal: make OpenDesk Remote suitable for enterprise production.

Deliverables:

- OIDC, LDAP, TOTP/email 2FA, backup codes.
- Rate limiting, secure cookies, CORS hardening, audit coverage.
- Backup/restore automation.
- Migration rollback strategy.
- Monitoring and alerting guidance.
- Relay health scoring and geo selection.
- Upgrade playbook for RustDesk upstream sync.

Exit gate:

- Security review passes.
- Distributed Region A/B/C deployment is smoke tested.
- Upgrade process is documented and repeatable.

## PR Sequence

- PR-001: finish M1 API repository layer and migration runner. Status: complete in current workspace, including memory/MySQL Users, Devices, Relays, Policy, and Build repositories plus live MySQL smoke.
- PR-002: Admin CRUD with Arco Design. Status: complete for Users, User Groups, Devices, Device Groups, Address Books, group membership, address book entries, Relays, Access Rules, Control Roles, Strategies, Logs, base Settings, OIDC/LDAP/SMTP settings, and Builder lifecycle baseline.
- PR-003: rustdesk-server hbbr Relay Auth hook. Status: API heartbeat, `target_rustdesk_id`, and upstream patch artifact complete; patched hbbr compile/test next.
- PR-004: RustDesk client BuildSpec injection. Status: builder-side injection generation and upstream client `RequestRelay.token` patch artifact complete; patched client compile/test next.
- PR-005: Windows builder runner. Status: staging, dry-run, configured command execution, environment doctor, artifact metadata, Build Profile API, Build Job lifecycle API, API/CLI worker execution, and Admin queue/run/artifact actions complete; CMake, Flutter, and MSVC setup plus real RustDesk artifact build next.
- Security baseline insertion: local admin login/session and protected management API. Status: complete for local admin cookie/Bearer session; OIDC/LDAP/2FA later in M6.
- PR-006: macOS builder runner.
- PR-007: Android builder runner.
- PR-008: iOS runner and capability verification.
- PR-009: OIDC/LDAP/2FA.
- PR-010: relay health, geo selection, and production monitoring.

## Current PR Cursor

The next implementation target is PR-005 continuation: install or wire CMake,
Flutter, and MSVC Build Tools into the Windows runner environment, produce the
first unsigned Windows artifact, and verify artifact persistence against live
MySQL/object storage. PR-003 and PR-004 remain blocked by real hbbr/client
compilation until MSVC Build Tools or a Linux container build path is available.
Work must preserve the current control-plane contract:

- Direct and hole-punch flows remain compatible.
- Relay access requires a short-lived grant.
- Grant validation rejects expired, revoked, mismatched, and replayed tokens.
- The hbbr patch remains feature-gated until compatibility is verified.
