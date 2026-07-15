# Custom Client Builder

The builder creates branded native client build jobs from BuildSpec. First
version implements schema validation, asset storage, Build Profile and Build
Job APIs, platform interfaces, Windows staging/command execution, and clear
not-configured failures.

## API

Authenticated administrators can create a Build Profile from a BuildSpec,
queue Build Jobs, and trigger the next queued job:

- `GET /api/v1/client-build-configs` (Admin path; `/api/v1/build-profiles` remains a compatibility alias)
- `POST /api/v1/client-build-configs` (Admin path; `/api/v1/build-profiles` remains a compatibility alias)
- `GET /api/v1/client-build-configs/{id}` (Admin path; `/api/v1/build-profiles/{id}` remains a compatibility alias)
- `PUT /api/v1/client-build-configs/{id}` (Admin path; `/api/v1/build-profiles/{id}` remains a compatibility alias)
- `DELETE /api/v1/client-build-configs/{id}` (Admin path; `/api/v1/build-profiles/{id}` remains a compatibility alias)
- `POST /api/v1/client-build-configs/{id}/jobs` (Admin path; `/api/v1/build-profiles/{id}/jobs` remains a compatibility alias)
- `POST /api/v1/assets/branding`
- `GET /api/v1/build-jobs`
- `POST /api/v1/build-jobs`
- `GET /api/v1/build-worker/doctor`
- `POST /api/v1/build-jobs/run-next`
- `GET /api/v1/build-jobs/{id}`
- `GET /api/v1/build-jobs/{id}/logs`
- `POST /api/v1/build-jobs/{id}/cancel`
- `POST /api/v1/build-jobs/{id}/retry`
- `GET /api/v1/build-jobs/{id}/artifacts`
- `GET /api/v1/build-artifacts/{id}/download`

`run-next` is protected by the same administrator session as the management
API. It claims one queued job, invokes `opendesk-builder run`, then persists the
job status, log path, and artifact metadata returned by the builder. The
standalone worker command below is intended for persistent deployments backed by
MySQL; the in-memory development repository is process-local and is best tested
through the API trigger.

Queued jobs can be canceled before a worker claims them. Finished jobs can be
retried, which creates a new queued job with the same profile and platform.
Log, artifact listing, and download endpoints are administrator-protected.
Build Profiles can be deleted only when no Build Jobs reference them; profiles
with existing jobs return HTTP 409 to preserve build history.

`build-worker/doctor` runs the same preflight used by the local
`opendesk-builder doctor` command. It reports source tree markers, configured
build command and artifact glob, local Rust/Cargo availability, and required
Windows build tools such as CMake, Flutter, and MSVC `cl.exe`/`link.exe`.

## BuildSpec

```json
{
  "app": {
    "name": "OpenDesk Remote",
    "vendor": "OpenDesk",
    "bundle_id": "com.example.opendeskremote",
    "windows_product_name": "OpenDesk Remote",
    "description": "Open-source self-hosted remote desktop"
  },
  "branding": {
    "logo_png": "asset://logo.png",
    "icon_ico": "asset://app.ico",
    "icon_icns": "asset://app.icns",
    "android_adaptive_icon": "asset://android-icon.png",
    "ios_app_icon": "asset://ios-icon.png",
    "tray_icon": "asset://tray.png",
    "installer_banner": "asset://banner.png"
  },
  "server": {
    "id_server": "rd.example.com:21116",
    "relay_server": "rd.example.com:21117",
    "relay_name": "hbbr-relay-a",
    "api_server": "https://rd.example.com",
    "key": "PUBLIC_KEY",
    "websocket": true,
    "relay_grant_required": true
  },
  "policy": {
    "profile": "default-secure",
    "override_settings": {
      "enable-file-transfer": "N",
      "enable-terminal": "N",
      "allow-remote-config-modification": "N"
    },
    "default_settings": {
      "verification-method": "use-both-passwords"
    }
  },
  "platforms": {
    "windows_x64": true,
    "macos_x64": true,
    "macos_arm64": true,
    "android_arm64": true,
    "ios_arm64": true
  },
  "signing": {
    "windows": "bring-your-own-cert",
    "macos": "bring-your-own-apple-developer",
    "android": "bring-your-own-keystore",
    "ios": "bring-your-own-apple-developer"
  }
}
```

## Upload Rules

- Accept `.png`, `.ico`, and `.icns`.
- Enforce size limits.
- Store in local storage.
- Generate server-side filenames.
- Reject executable extensions and path traversal.
- Record sha256.

## Local Builder Commands

Validate a BuildSpec:

```powershell
cd builder
..\.tools\go\bin\go.exe run ./cmd/opendesk-builder validate --spec .\examples\buildspec.example.json
```

Generate client injection files:

```powershell
cd builder
..\.tools\go\bin\go.exe run ./cmd/opendesk-builder inject --spec .\examples\buildspec.example.json --out ..\.run\client-injection
```

Inspect the Windows build environment:

```powershell
cd builder
..\.run\bin\opendesk-builder.exe doctor --platform windows_x64 --source ..\.upstream\rustdesk-client --dry-run
..\.run\bin\opendesk-builder.exe doctor --platform windows_x64 --source ..\.upstream\rustdesk-client --build-command "cargo build --release --features flutter" --artifact-glob "target/release/*.exe"
```

Stage injection files into a patched RustDesk Windows source tree without
building:

```powershell
cd builder
..\.tools\go\bin\go.exe run ./cmd/opendesk-builder run --platform windows_x64 --spec .\examples\buildspec.example.json --source ..\.upstream\rustdesk-client --injection ..\.run\windows-runner\injection --artifacts ..\.run\windows-runner\artifacts --dry-run
```

Run a configured Windows build command:

```powershell
cd builder
..\.tools\go\bin\go.exe run ./cmd/opendesk-builder run --platform windows_x64 --spec .\examples\buildspec.example.json --source C:\src\rustdesk --injection C:\build\opendesk-injection --artifacts C:\build\opendesk-artifacts --build-command "cargo build --release --features flutter" --artifact-glob "target/release/*.exe"
```

## Worker Execution

Build the worker and run one queued job:

```powershell
cd api
..\.tools\go\bin\go.exe build -o ..\.run\opendesk-worker.exe .\cmd\opendesk-worker
..\.run\opendesk-worker.exe run-once
```

Relevant environment variables:

- `OPENDESK_BUILDER_BIN`: path to `opendesk-builder`.
- `OPENDESK_BUILDER_WORK_DIR`: per-job worker scratch directory.
- `OPENDESK_BUILDER_SOURCE_DIR`: patched RustDesk client source directory.
- `OPENDESK_BUILDER_DRY_RUN`: `true` stages generated files without compiling.
- `OPENDESK_BUILDER_TIMEOUT`: builder timeout, for example `2h`.
- `OPENDESK_BUILDER_WINDOWS_COMMAND`: real Windows build command.
- `OPENDESK_BUILDER_WINDOWS_ARTIFACT_GLOB`: artifact glob copied after build.

Generated files:

- `opendesk/buildspec.normalized.json`
- `opendesk/branding.json`
- `opendesk/policy.json`
- `opendesk/manifest.json`
- `rust/src/opendesk_generated.rs`

The Rust file contains generated constants for ID server, relay server, relay
name, API server, public key, WebSocket mode, relay grant requirement, and
policy defaults. The future RustDesk client patch must include this generated
module at build time.

The Windows runner stages `rust/src/opendesk_generated.rs` into
`src/opendesk_generated.rs` inside the patched RustDesk source tree before it
runs the configured build command. If `--artifact-glob` is provided, matching
artifacts are copied to the artifact directory and returned with SHA256 and byte
size metadata.

## Platform Order

1. Windows
2. macOS
3. Android
4. iOS
