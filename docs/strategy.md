# Strategy

Strategy controls managed client configuration.

## Priority

1. Device Strategy
2. User Strategy
3. Device Group Strategy
4. Default Strategy

The resolver merges default settings with higher-priority overrides and returns
a previewable effective strategy.

## API Status

- `GET /api/v1/strategies`
- `POST /api/v1/strategies`
- `PUT /api/v1/strategies/{id}`
- `DELETE /api/v1/strategies/{id}`
- `POST /api/v1/strategies/{id}/assignments`
- `DELETE /api/v1/strategies/{id}/assignments/{assignment_id}`
- `GET /api/v1/devices/{id}/effective-strategy`

Create, update, and delete operations write audit events. Updating a strategy
replaces its assignment set in the current API implementation.

The effective-strategy endpoint currently uses the managed device owner as the
optional user context unless `?user_id=` is provided. It includes a default
secure fallback that disables terminal, TCP tunnel, and remote configuration
modification.
