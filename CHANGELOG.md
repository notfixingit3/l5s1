# Changelog

All notable changes to L5S1 are documented here.

Format inspired by [Keep a Changelog](https://keepachangelog.com/). Versions follow pre-release semver: `v0.0.1-beta.N`.

## [0.0.1-beta.16] — 2026-07-17

### Added
- **Admin user email edit** — correct or clear a user’s email (unique check)
- **Admin passkey management** — view devices (name, added, use count); revoke one passkey or wipe all
- **Admin tags: system vs custom** — default catalog tags marked **Default**; enable/disable only (never deleted)
- **Custom tag delete with replacement** — if a custom tag is on logs, admin must pick a replacement key before delete
- **AGENTS.md** — local-first workflow, beta-tag CI, WebAuthn and secrets policy

### Changed
- GitHub Actions: **CI + multi-arch container only on `v*-beta.*` tags** (no build on every `dev` push)
- Container builds: **parallel native amd64 + arm64** (no QEMU), tests outside Docker image build
- Lab/deploy docs scrubbed of private host paths; generic `deploy/lazyapp` template

### Fixed
- Clearer Default/Custom tag badges; delete control hidden for system tags

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.16
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.15] — 2026-07-17

### Added
- **Device link codes** — signed-in users mint a one-time 8-digit code (`xxxx-xxxx`) so another phone/laptop can register a passkey without already being logged in
- Auth **Add device** tab + Profile **Generate device code** UI
- Lab/hosted compose template `deploy/lazyapp` (GHCR, host bind `./data`, Traefik labels)
- Invite codes displayed as `xxxx-xxxx` (same format as device codes)

### Fixed
- Version display no longer double-stamps the git SHA
- README lockup: transparent tight crop; favicon transparent corners
- CI: pin Go **1.25.12** for govulncheck
- Repository rename to `notfixingit3/l5s1`

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.15
```

## [0.0.1-beta.14] — 2026-07-17

### Added
- Passwordless WebAuthn multi-device passkeys (Gin + go-webauthn)
- Patient check-in, partner mode, clinician summary, admin workspace
- Docker multi-arch image publish to GHCR on beta tags

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.14
```

## Unreleased

- (next beta)
