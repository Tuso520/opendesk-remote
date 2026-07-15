# OpenDesk Remote Agent Guide

This file defines the default boundaries for Codex and other automated agents
working in this project. Unless the user explicitly authorizes a narrower or
broader task, agents must follow these directory responsibilities, protected
areas, and entrypoint notes.

## Project Role

OpenDesk Remote is a standard product project under the new one-person software
company architecture. The repository should keep product code, builder tooling,
admin UI, deployment materials, documentation, and upstream source boundaries
easy to understand.

This directory is currently not a Git repository. Do not initialize Git, add a
remote, commit, push, or rewrite repository history unless the user explicitly
asks for that work.

## Default Rules

- Read this file, `README.md`, `docs/development-environment.md`,
  `docs/environment-variables.md`, and relevant subdirectory docs before
  acting.
- Mark uncertain information as `待确认`; do not present assumptions as facts.
- Do not change business code unless the task explicitly asks for code changes.
- Do not change deployment files, CI/CD, Docker, or system service files unless
  the task explicitly includes those areas.
- Do not initialize Git, add a GitHub remote, commit, or push unless the task
  explicitly asks for source-control work.
- Put temporary outputs in `.run/`, in a subproject's documented build output
  directory, or in a task-specific output path.

## Directory Responsibilities

### `api/`

Go control-plane service. It owns the management API, authentication, policy
logic, Relay Grant flow, build job API, migrations, storage, and worker
coordination.

Confirmed entrypoints:

- API server: `go run ./cmd/opendesk-api`
- Database migrations: `go run ./cmd/opendeskctl migrate`
- Build worker single run: `go run ./cmd/opendesk-worker run-once`
- Tests: `go test ./...`
- Local environment example: `api/.env.example`

`api/internal/config/config.go` loads `.env` from the current working directory.
For local development from `api/`, copy `api/.env.example` to `api/.env`.

### `builder/`

Go custom-client builder CLI. It owns BuildSpec validation, branded injection
file generation, build environment checks, and platform build execution.

Confirmed entrypoints:

- Validate BuildSpec: `go run ./cmd/opendesk-builder validate --spec ./examples/buildspec.example.json`
- Generate injection files: `go run ./cmd/opendesk-builder inject --spec ./examples/buildspec.example.json --out ../.run/client-injection`
- Windows doctor: `go run ./cmd/opendesk-builder doctor --platform windows_x64 --source ../.upstream/rustdesk-client --dry-run`
- Windows dry-run build: `go run ./cmd/opendesk-builder run --platform windows_x64 --spec ./examples/buildspec.example.json --source ../.upstream/rustdesk-client --injection ../.run/client-injection --artifacts ../.run/artifacts --dry-run`
- Tests: `go test ./...`

`builder/` may read `.upstream/rustdesk-client` as an input source tree, but it
must not automatically modify `.upstream/`.

### `admin/`

Vue/Vite admin console for the OpenDesk Remote browser management UI.

Confirmed entrypoints from `admin/package.json`:

- Install dependencies: `npm install`
- Local development server: `npm run dev`
- Production build: `npm run build`
- Local preview: `npm run preview`

For local API integration in PowerShell:

```powershell
$env:VITE_OPENDESK_API_URL='http://127.0.0.1:21114'
npm run dev
```

### `client/` and `server/`

Patch artifacts, design notes, and upstream integration materials for the
RustDesk client and server.

Default mode is read-only. Do not modify these directories unless the task
explicitly asks for patch or integration documentation changes.

### `deploy/`

Deployment examples and reverse proxy/systemd/Docker Compose materials.

Default mode is read-only. Do not modify deployment files, Docker Compose,
Nginx, Caddy, systemd, or deployment environment examples unless the task
explicitly includes deployment changes.

Confirmed deployment references:

- Region A Compose: `deploy/docker-compose.distributed-region-a.yml`
- Relay Region Compose: `deploy/docker-compose.relay-region-b.yml`
- Relay Region Compose: `deploy/docker-compose.relay-region-c.yml`
- Region A env example: `deploy/env/region-a.env.example`
- Relay env example: `deploy/env/relay-region.env.example`
- Nginx reverse proxy docs: `docs/deployment/reverse-proxy-nginx.md`
- Caddy reverse proxy docs: `docs/deployment/reverse-proxy-caddy.md`
- Port docs: `docs/deployment/ports.md`
- Distributed deployment docs: `docs/deployment/distributed.md`

### `docs/`

Product, architecture, deployment, security, roadmap, and development
documentation. Prefer existing docs as the source of truth for structure,
behavior, status, and operational assumptions.

Key docs for agent orientation:

- `docs/architecture.md`
- `docs/development-environment.md`
- `docs/environment-variables.md`
- `docs/custom-client-builder.md`
- `docs/deployment/distributed.md`

### `scripts/`

Development, migration, patch preparation, and platform build scripts. Read the
script and related documentation before running anything. Confirm that the
script does not write to protected areas or deployment configuration unless that
is part of the requested task.

### `.tools/`

Project-local toolchain area used by this workstation for Go/Rust and related
tools. Treat it as read-only unless the task explicitly asks for toolchain
maintenance.

### `.run/`

Local runtime and build output area. This is the preferred location for
temporary binaries, worker scratch state, generated injection files, and local
artifacts.

### Local runtime artifacts

`.run/` and files such as `api/tmp-api.*.log` are local runtime or smoke-test
outputs. Treat them as evidence for a specific local run only, not as source of
truth. Do not copy them into company-control documents, deployment materials, or
source-controlled guidance unless the user explicitly asks for a run log.

### `.upstream/`

Protected area: do not automatically modify.

`.upstream/` contains upstream RustDesk client/server source trees or working
copies. Agents may read it to understand integration points, run read-only
checks, or pass it as builder input. Agents must not edit, format, clean,
delete, move, patch, or generate files inside `.upstream/` without explicit user
approval. When upstream patching is approved, prefer the existing
`scripts/prepare-rustdesk-*.ps1` workflows.

## Build and Test Entrypoints

On this Windows workstation, prefer the project-local Go toolchain documented in
`docs/development-environment.md`.

API:

```powershell
cd api
..\.tools\go\bin\go.exe test ./...
..\.tools\go\bin\go.exe run ./cmd/opendesk-api
```

API migrations:

```powershell
cd api
..\.tools\go\bin\go.exe run ./cmd/opendeskctl migrate
```

Builder:

```powershell
cd builder
..\.tools\go\bin\go.exe test ./...
..\.tools\go\bin\go.exe run ./cmd/opendesk-builder validate --spec .\examples\buildspec.example.json
```

Admin:

```powershell
cd admin
npm install
$env:VITE_OPENDESK_API_URL='http://127.0.0.1:21114'
npm run dev
npm run build
```

Docker image entrypoints are documented by:

- `api/Dockerfile`
- `builder/Dockerfile`
- `admin/Dockerfile`

Do not change Dockerfiles unless the task explicitly includes Docker changes.

## Environment Entrypoints

Confirmed environment examples:

- Local API: `api/.env.example`
- Region A deployment: `deploy/env/region-a.env.example`
- Relay deployment: `deploy/env/relay-region.env.example`

Use `docs/environment-variables.md` as the project-level index for environment
variable entrypoints and unresolved environment questions.

There is currently no root-level `.env.example`.

`待确认`:

- Whether this standard product project should add a root-level `.env.example`
  as a unified one-person-company project entrypoint.
- If a root-level `.env.example` is added, whether it should be a documentation
  index or a real runtime config file read by processes.
- Whether relative path variables in `api/.env.example`, such as
  `OPENDESK_BUILDER_WORK_DIR` and `OPENDESK_BUILDER_SOURCE_DIR`, should be
  normalized for launches from different working directories.

## Safe Task Boundaries

Allowed by default:

- Read code and documentation.
- Add or update project-level guidance documentation.
- Run read-only checks, tests, and builds.
- Generate temporary artifacts under `.run/`.

Requires explicit authorization:

- Changing business code.
- Changing `.upstream/`.
- Changing deployment files, Dockerfiles, CI/CD, or systemd configuration.
- Adding or changing database migrations.
- Reading, copying, or summarizing real `.env` files outside documented example
  files.
- Deleting, moving, or bulk-formatting files.
- Initializing Git, adding a remote, committing, or pushing.

## Verification Expectations

After completing a task, report:

- Files changed.
- Whether business code was touched.
- Whether `.upstream/` was touched.
- Tests or checks run.
- Any information still marked as `待确认`.
