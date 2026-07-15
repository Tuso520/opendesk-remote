# Access Control

Access Control answers: who can connect to which device?

## Inputs

- `user_id`
- `controller_device_id`
- `target_device_id`

## Rules

1. Disabled or locked user denies.
2. Disabled target device denies.
3. Explicit deny wins over allow.
4. Direct user to device is strongest.
5. User group to device follows.
6. User to device group follows.
7. User group to device group follows.
8. Higher `priority` wins within the same category.
9. No matching rule defaults to deny.
10. Every decision returns a reason.

## API Status

- `GET /api/v1/access-rules`
- `POST /api/v1/access-rules`
- `PUT /api/v1/access-rules/{id}`
- `DELETE /api/v1/access-rules/{id}`
- `POST /api/v1/access/evaluate`

Access rule create, update, and delete operations write audit events.
