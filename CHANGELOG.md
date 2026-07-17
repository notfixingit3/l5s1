# Changelog

All notable changes to L5S1 are documented here.

Format inspired by [Keep a Changelog](https://keepachangelog.com/). Versions follow pre-release semver: `v0.0.1-beta.N`.

## [0.0.1-beta.14] — 2026-07-17

### Added
- Passwordless WebAuthn multi-device passkeys (Gin + go-webauthn)
- Patient check-in (pain 1–10, notes, curated tags)
- Partner observation mode with grant/access matrix
- Clinician summary (presets, histogram, tag badges, timeline)
- Admin workspace: users (lock / roles / force re-reg), invites (8-digit, max uses), tags (CRUD + reorder)
- Invite-gated signup for new accounts; seeded shells for admin / demo patient / partner
- In-app confirm/prompt dialogs (no browser popups)
- Version endpoint (`/api/version`, `/api/healthz`) and UI version pill
- Docker multi-arch image publish to GHCR on `dev` and `v*` tags

### Security
- gosec clean; govulncheck clean for reachable code
- `golang.org/x/net` ≥ 0.38.0
- Data directory permissions 0750

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.14
# or
docker pull ghcr.io/notfixingit3/l5s1:latest   # tracks newest tag
```

## Unreleased

- (next beta)
