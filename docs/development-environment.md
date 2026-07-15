# Development Environment

This workstation uses a project-local Go toolchain under `.tools/` so the
project can be tested without changing the global Windows environment.

## Local Toolchain

Go:

```powershell
.\.tools\go\bin\go.exe version
```

API tests:

```powershell
cd api
..\.tools\go\bin\go.exe test ./...
```

Use the following normalized form in PowerShell:

```powershell
$go = (Resolve-Path '..\.tools\go\bin\go.exe').Path
& $go test ./...
```

Admin:

```powershell
cd admin
npm install
$env:VITE_OPENDESK_API_URL='http://127.0.0.1:21114'
npm run dev
npm run build
```

Rust:

```powershell
$env:RUSTUP_HOME = (Resolve-Path '.\.tools\rustup').Path
$env:CARGO_HOME = (Resolve-Path '.\.tools\cargo').Path
$env:Path = "$env:CARGO_HOME\bin;$env:Path"
rustc --version
cargo --version
```

Windows builder doctor:

```powershell
.\.run\bin\opendesk-builder.exe doctor --platform windows_x64 --source .\.upstream\rustdesk-client --dry-run
.\.run\bin\opendesk-builder.exe doctor --platform windows_x64 --source .\.upstream\rustdesk-client --build-command "cargo build --release --features flutter" --artifact-glob "target/release/*.exe"
```

Apply the open-source rustdesk-server hbbr patch:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare-rustdesk-server-patch.ps1
```

## Current Local Environment

- Node.js is available globally.
- npm is available globally.
- Go is installed locally in `.tools`.
- Rust is installed locally in `.tools`.
- Rust hbbr compilation on Windows still needs MSVC `link.exe` from Visual
  Studio Build Tools with the C++ workload, or a Linux container build.
- Docker CLI is present, but Docker daemon may need to be started before image
  builds or containerized tests.

## Smoke Test

For local memory-backed development, leave `OPENDESK_MYSQL_DSN` unset.

```powershell
cd api
$go = (Resolve-Path '..\.tools\go\bin\go.exe').Path
& $go build ./cmd/opendesk-api
$env:OPENDESK_RELAY_GRANT_SIGNING_KEY='local-dev-signing-key'
$env:OPENDESK_JWT_SECRET='local-dev-session-signing-key-with-enough-length'
$env:OPENDESK_HTTP_ADDR='127.0.0.1:21114'
.\opendesk-api.exe
```

Then:

```powershell
Invoke-RestMethod http://127.0.0.1:21114/api/v1/health
$setupBody = @{ email='admin@example.com'; password='local-admin-password-12345' } | ConvertTo-Json
Invoke-RestMethod http://127.0.0.1:21114/api/v1/setup/admin -Method Post -ContentType 'application/json' -Body $setupBody
$loginBody = @{ email='admin@example.com'; password='local-admin-password-12345' } | ConvertTo-Json
Invoke-RestMethod http://127.0.0.1:21114/api/v1/auth/login -Method Post -ContentType 'application/json' -Body $loginBody -SessionVariable od
Invoke-RestMethod http://127.0.0.1:21114/api/v1/users -WebSession $od
```

The Admin console uses the local account created through setup in development:

- email: `admin@example.com`
- password: `local-admin-password-12345`

For local browser verification, prefer `VITE_OPENDESK_API_URL` over the Vite
proxy so Admin calls the API directly through the API CORS allowlist.

For MySQL-backed development, start MySQL, set `OPENDESK_MYSQL_DSN`, run
`opendeskctl migrate`, then start `opendesk-api`. If
`OPENDESK_INITIAL_ADMIN_EMAIL` and `OPENDESK_INITIAL_ADMIN_PASSWORD` are set,
the API seeds the first local administrator when no users exist. Otherwise use
the setup endpoint.

On this workstation, Scoop MySQL 9.7.1 can be used without Docker. Use an
ASCII-only data path because MySQL startup can misread the Chinese workspace
path:

```powershell
mysqld --initialize-insecure --datadir=C:\Users\zhou\opendesk-remote-mysql-data --console
mysqld --datadir=C:\Users\zhou\opendesk-remote-mysql-data --port=33306 --bind-address=127.0.0.1 --mysqlx=0 --console
mysql --protocol=tcp --host=127.0.0.1 --port=33306 --user=root -e "CREATE DATABASE IF NOT EXISTS opendesk_remote CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"
```

Then set:

```powershell
$env:OPENDESK_MYSQL_DSN='root@tcp(127.0.0.1:33306)/opendesk_remote?parseTime=true&multiStatements=true'
```
