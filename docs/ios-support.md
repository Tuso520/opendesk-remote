# iOS Support

iOS is a target platform for OpenDesk Remote, but it has signing, permission,
distribution, and remote control capability constraints.

## Requirements

- macOS runner.
- Apple Developer account.
- Bundle ID.
- Signing certificate.
- Provisioning profile.

## First Stage Scope

- BuildSpec model.
- Build job model.
- Signing configuration model.
- Build pipeline interface.
- Documentation and clear failure messages.

## Capability Matrix

| Capability | First-stage status |
| --- | --- |
| App build model | Planned/implemented in builder skeleton |
| Signing config model | Planned/implemented in builder skeleton |
| Remote control as controller | Requires verification |
| Full unattended controlled device | Not promised until verified |
| App Store / enterprise distribution | Requires customer signing assets |

