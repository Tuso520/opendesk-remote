# Control Role

Control Role answers: once connected, what can the controller do?

Default production-sensitive permissions:

- `terminal = disable`
- `tcp_tunnel = disable`
- `remote_config_modification = disable`
- `file_transfer = use_client_settings`
- `clipboard = use_client_settings`
- `keyboard_mouse = use_client_settings`

Supported permission keys:

- keyboard_mouse
- remote_printer
- clipboard
- file_transfer
- audio
- camera
- terminal
- tcp_tunnel
- remote_restart
- recording_session
- block_user_input
- remote_config_modification

## API Status

- `GET /api/v1/control-roles`
- `POST /api/v1/control-roles`
- `PUT /api/v1/control-roles/{id}`
- `DELETE /api/v1/control-roles/{id}`

Create, update, and delete operations write audit events. Updating a role
replaces its permission set.
