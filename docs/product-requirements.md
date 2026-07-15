# Product Requirements

## Product

- Repository: `opendesk-remote`
- Display name: `OpenDesk Remote`
- License: AGPL-3.0-only

## Users

- Individuals hosting their own remote desktop service.
- Teams needing address books, device groups, and audit logs.
- Enterprises needing access control, relay governance, strategies, and custom
  branded clients.

## Non-Goals for v1

- Web Client.
- Hidden installation.
- Covert remote control.
- RustDesk Server Pro code, assets, or proprietary behavior.
- Default fallback to public RustDesk servers.

## First Release Scope

- API control plane with MySQL, Redis, local storage, health checks, models, and
  migrations.
- Relay Grant issuing and validation interfaces.
- Access Control, Control Role, and Strategy evaluators.
- BuildSpec and build job foundations for Windows, macOS, Android, and iOS.
- Distributed deployment files for Region A, B, and C.
- Admin Web skeleton with complete menu plan.

