# Changelog

All notable changes to L5S1 are documented here.

Format inspired by [Keep a Changelog](https://keepachangelog.com/). Versions follow pre-release semver: `v0.0.1-beta.N`.

## [0.0.1-beta.29] — 2026-07-18

### Added
- **Summary pack filter** (“Focus for this visit”): All conditions, or one enabled pack (Heart, Stenosis, UC, …)
  - Recalculates overview, pain histogram, tag counts, timeline, and partner notes to entries tagged with that pack
  - Untagged entries only appear under All
  - Print/share include the focus label
- `GET /api/logs/summary?pack=heart` (+ `pack_filters` list for the UI)

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.29
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.28] — 2026-07-18

### Added
- **Visit summary (all four)**
  1. **Print / Share** — print-friendly layout + Share (or copy text)
  2. **Care partner notes** in the selected period (author name + notes)
  3. **Last visit date** — save on Summary; **Last visit** range preset
  4. **Tags grouped by pack** in frequency (Other for history outside enabled packs)
- `User.last_visit_at`; `PATCH /api/auth/profile` accepts `last_visit_at`
- `GET /api/logs/summary` returns `observations`, `tag_groups`, `last_visit_at`, `since_last_visit=1`

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.28
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.27] — 2026-07-18

### Changed
- **Cleanup / security hygiene**
  - Removed unused `POST /api/admin/tags/reorder` (UI uses move up/down)
  - Dropped dead helpers: `DecodeJSON`, `RequireRoles`, rate-limiter `Remaining`, `TagKeyPacks`
  - Renamed session context key so gosec no longer false-positives “hardcoded credentials”
- `make security` clean: vet + gosec (0 issues) + govulncheck + eslint

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.27
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.26] — 2026-07-18

### Added
- **Tag chips grouped by pack** on check-in and edit sheet (General / Stenosis / UC / …)
- Empty-state hint when only General is enabled, with link to Profile → Tag packs
- Edit sheet keeps tags already on an entry even if that pack is now off (**On this entry**)

### Fixed
- **Sessions survive restarts/deploys** — app sessions stored in SQLite (`app_sessions`); no more mass logout when the container recreates

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.26
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.25] — 2026-07-17

### Added
- **Tag packs:** UC / IBD, Heart, Sleep apnea (opt-in alongside Stenosis and Diabetes)
  - UC: flare, urgency, diarrhea, blood in stool, bloating, nausea, mucus, night stools, bathroom trips
  - Heart: BP high/ok, chest pain/tightness, palpitations, heart racing, SOB, dizziness, ankle swelling
  - Sleep apnea: morning headache, headache, daytime tired, unrefreshing sleep, snoring, gasping, dry mouth, brain fog, restless sleep, naps, insomnia
- New system catalog tags seeded automatically; `TAG_ORDER_VERSION` → 3

### Images
```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.25
docker pull ghcr.io/notfixingit3/l5s1:latest
```

## [0.0.1-beta.24] — 2026-07-17

### Added
- **Tag packs** (per-user) for the check-in picker
  - **General** — always on (left / right / both sides)
  - **Stenosis / spine** — regions, sensations, walking, stenosis (default on)
  - **Diabetes** — glucose high/low (opt-in)
- Profile → **Tag packs** toggles; `GET/PUT /api/packs`; `GET /api/tags` filters by enabled packs
- Custom admin tags and unassigned system tags (e.g. UC / BP) stay visible when active
- Historical log CSV keys unchanged when packs change

### Images
```bash
docker pull ghcr.io/nottellingit3/l5s1:v0.0.1-beta.24
docker pull ghcr.io/nottellingit3/l5s1:latest
```

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
