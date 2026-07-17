# Changelog

All notable changes to L5S1 are documented here.

Format inspired by [Keep a Changelog](https://keepachangelog.com/). Versions follow pre-release semver: `v0.0.1-beta.N`.

## [0.0.1-beta.15] — 2026-07-17

### Added
- **Device link codes** — signed-in users mint a one-time 8-digit code (`xxxx-xxxx`) so another phone/laptop can register a passkey without already being logged in
- Auth **Add device** tab + Profile **Generate device code** UI
- Lab stack template `deploy/lazyapp` for `notfixingit:/opt/stacks/l5s1_lazyapp` (GHCR `:dev`, host bind `./data`)
- Invite codes displayed as `xxxx-xxxx` (same format as device codes)

### Fixed
- Version display no longer double-stamps the git SHA (`v…-gsha+sha` → `v…+sha`)
- README lockup: transparent tight crop (no white tile)
- Favicon / app-icon: transparent corners outside the squircle; dedicated SVG/PNG/ICO favicons
- CI: pin Go **1.25.12** so govulncheck passes on current stdlib advisories
- Repository rename to `notfixingit3/l5s1` (docs + GHCR image name)

### Security
- Device-link + invite code attempt rate limits (12 failures / 15 min)
- Device codes: 20-minute TTL, single use, max 3 active per user, bound to username

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.15
# or
docker pull ghcr.io/notfixingit3/l5s1:latest   # tracks newest tag
docker pull ghcr.io/notfixingit3/l5s1:dev       # rolling lab/dev
```

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
```

## Unreleased

- (next beta)
