# Contributing

Contributions are welcome when they respect the project license, trademark
boundaries, and misuse policy.

## Development Rules

- Preserve upstream copyright and license notices.
- Do not import RustDesk Server Pro code, resources, or behavior copied from
  closed-source components.
- Keep RustDesk client/server patchsets small and reviewable.
- Add tests for access control, relay grants, strategies, and builder changes.
- Do not commit secrets, signing keys, Apple assets, Android keystores, or
  Windows code-signing certificates.

## Local Checks

```bash
cd api
go test ./...
cd ../admin
npm run build
```

