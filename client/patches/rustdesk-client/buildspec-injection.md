# BuildSpec Injection

BuildSpec injection writes server configuration, branding, and policy defaults
at build time.

Injected server fields:

- ID Server
- Relay Server
- Relay Name
- API Server
- Server public key
- WebSocket setting
- Relay Grant required flag

Current generated artifact:

```text
rust/src/opendesk_generated.rs
```

Generate it with:

```powershell
cd builder
..\.tools\go\bin\go.exe run ./cmd/opendesk-builder inject --spec .\examples\buildspec.example.json --out ..\.run\client-injection
```

The patch must avoid changing core remote-control behavior unless a narrow
OpenDesk feature flag requires it.

Apply the current upstream client patch with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare-rustdesk-client-patch.ps1
```

The patch adds default `src/opendesk_generated.rs` values and expects the
builder output to replace that file before platform builds.
