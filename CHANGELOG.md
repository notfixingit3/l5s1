# Changelog

All notable changes to L5S1 are documented here.

Format inspired by [Keep a Changelog](https://keepachangelog.com/). Versions follow pre-release semver: `v0.0.1-beta.N`.

## [0.0.1-beta.23] — 2026-07-17

### Fixed
- **Passkey use counter** — track our own `use_count` + `last_used_at` on login (synced passkeys like Bitwarden always report authenticator `sign_count` 0)
- **This session badge** on the Profile device list for the passkey that signed you in

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.23
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.22] — 2026-07-17

### Fixed
- **Tab bar docks to viewport bottom** while content scrolls (flex shell instead of fragile `position:fixed`)
- **Admin tab** reliably shown for `role=admin` (harder visibility toggle; works with 4 equal tabs)

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.22
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.21] — 2026-07-17

### Added
- **In-app notifications** (bell in top bar)
  - Patient check-in → all linked care partners
  - Partner observation → the patient
  - New partner access grant → the partner
- APIs: `GET /api/notifications`, `GET /api/notifications/unread-count`, `POST /api/notifications/:id/read`, `POST /api/notifications/read-all`
- Unread badge + panel; poll while the tab is visible

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.21
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.20] — 2026-07-18

### Changed
- Footer simplified to **l5s1.com · version** (dropped “Passkeys only” fluff); still shown when signed in above the tab bar

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.20
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.19] — 2026-07-18

### Fixed
- **Mobile tab bar** pinned to the viewport (no longer scrolls away); Admin tab tappable with equal flex hit targets
- Version pill removed from top bar (footer only)

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.19
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.18] — 2026-07-18

### Added
- **Swipe recent entries** — swipe right to edit (pain/notes/tags sheet), swipe left to delete (with confirm)
- `PATCH` / `DELETE` `/api/logs/:id` for the patient’s own check-ins
- Production domain **[l5s1.com](https://l5s1.com)** in deploy template, READMEs, and GitHub About/homepage
- Partner access verification tests (view patient logs only after grant)

### Changed
- README hero uses large app-icon mark (Imagine-refined); no lockup wordmark text
- Hosted stack WebAuthn / Traefik hosts: `l5s1.com` + `www.l5s1.com`

### Fixed
- Header version pill short form so “Daily check-in” stays one line (from beta.17 follow-ups)

### Security
- `go test ./...`, `go vet`, **govulncheck** (0 reachable), **gosec** (0 issues)

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.18
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.17] — 2026-07-18

### Fixed
- Header version pill shows product version only; full `+commit` on tooltip

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.17
```

## [0.0.1-beta.16] — 2026-07-17

### Added
- Admin email edit; view/revoke passkeys
- System vs custom tags; custom delete with replacement
- AGENTS.md; tag-only CI multi-arch builds

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.16
```

## [0.0.1-beta.15] — 2026-07-17

### Added
- Device link codes for multi-device passkeys
- Lab/hosted compose template

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.15
```

## [0.0.1-beta.14] — 2026-07-17

### Added
- Initial beta: WebAuthn PWA, patient/partner/clinician/admin, GHCR images

## Unreleased

- (next beta)
