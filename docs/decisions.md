# Decisions

## Repository Detection

The `opendesk-remote` folder was cleared by explicit request. The previous
contents resembled a RustDesk Server fork, but the new implementation path is an
empty OpenDesk Remote control repository with documented client/server patch
areas for future RustDesk integration.

## Key Decisions

- **AGPL-3.0-only**: required because RustDesk OSS and rustdesk-server are AGPL
  and OpenDesk Remote is intended to remain fully open source.
- **MySQL only**: first version avoids PostgreSQL to keep production operations
  and migrations aligned with the confirmed requirement.
- **Local storage first**: branding assets and build artifacts use filesystem
  storage by default; a future `StorageDriver` can add object storage.
- **No Web Client in v1**: avoids importing `webclient` or `webclient2` assets
  and keeps scope focused on native clients and relay authorization.
- **Relay requires authorization**: direct and hole-punching can be anonymous,
  but relay consumes shared infrastructure and must require login or managed
  device authorization.
- **Platform priority**: Windows > macOS > Android > iOS follows confirmed
  product priority and realistic build complexity.
- **Region A includes relay**: Region A is both control plane and relay region;
  it must include `hbbr-relay-a` for production completeness.
- **Patch documentation before patch import**: client/server forks are not
  present yet, so this repository records patch design instead of pretending
  integration is complete.

