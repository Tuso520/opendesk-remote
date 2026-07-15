# Upgrade Strategy

OpenDesk Remote should keep upstream patchsets small.

## Upstream Remotes

- RustDesk client upstream.
- rustdesk-server upstream.
- lejianwen/rustdesk-api reference, if imported.

## Client Patch Layers

1. Branding patch.
2. BuildSpec config injection patch.
3. OpenDesk API login patch.
4. Relay Grant hook patch.
5. Platform build patch.

## Server Patch Layers

1. Relay auth hook.
2. Relay heartbeat.
3. Connection log callback.
4. WebSocket configuration documentation.

## Required Checks on Upgrade

- Compile tests.
- Unit tests.
- Migration tests.
- Access control tests.
- Relay grant tests.
- Strategy resolver tests.
- BuildSpec validation tests.
- Docker compose smoke test.

