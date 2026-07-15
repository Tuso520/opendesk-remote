# Backup and Restore

## Backup

- MySQL dump from Region A.
- Local storage directory from Region A.
- Deployment `.env` files from secure secret storage.

## Restore

1. Restore MySQL.
2. Restore local storage to the configured path.
3. Start Redis.
4. Start OpenDesk API.
5. Start Admin and relay services.

Do not store backups only inside containers.

