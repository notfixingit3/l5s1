# Security scan report (L5S1)

Run locally:

```bash
make security
```

CI runs the same suite on every `dev` push and PR (see `.github/workflows/ci.yml`),
plus on beta tags before images publish.

## Backend

| Tool | Result | Notes |
|------|--------|--------|
| **go vet** | Pass | No issues |
| **gosec** | Pass (0 issues) | Now runs in CI (pinned v2.30.0), not just locally |
| **govulncheck** | Pass (0 called vulns) | Pinned v1.10.0; dependencies refreshed 2026-11-20 (gin 1.12.0, gorm 1.31.2, go-webauthn 0.18.2, x/net 0.63.0, x/crypto 0.59.0, modernc.org/sqlite 1.62.0) |
| **go test** | Pass | All package tests, incl. CORS/rate-limit/session-invalidation coverage |

### Hardening posture (2026-11-20 review)

- CORS: responses carry CORS headers only for origins in `WEBAUTHN_ORIGINS` (previously any Origin was reflected with credentials)
- Headers: CSP (`script-src 'self'`), `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` on every response; no inline JS
- HTTP server: explicit read/write/idle timeouts + graceful SIGINT/SIGTERM shutdown
- Rate limiting: register/login failures all count (invalid attempts, failed assertions, no-passkey/deactivated/force-re-reg responses); limiter keys expire
- Sessions: force re-registration and account lock invalidate sessions immediately
- Secrets: `VAPID_PRIVATE_KEY` (and any `*PRIVATE*/*SECRET*/*TOKEN*/*PASSWORD*` config key) is redacted from the admin config API
- First boot: `ALLOW_SIGNUPS`/`SEED_DEMO_ACCOUNTS` default off; demo seeding dev-only; startup warns on passkey-less admins
- Container: non-root `l5s1` user (uid `APP_UID`), `debian:trixie-slim` runtime, Go 1.27.4
- Transactions: register/finish redeem, tag rename/delete/move, and admin user updates are atomic

### Residual / accepted

- govulncheck may still report **transitive** vulns in unused import paths of dependencies (not reachable from this app’s call graph). Re-check after major dependency upgrades.
- The ceremony store, session hot-cache, and rate limiter are in-memory — single-node only. Production should stay single-replica or move to a shared store (plus HTTPS, `SECURE_COOKIE=true`).
- SQLite data at rest is unencrypted; rely on host disk encryption, or add application-layer encryption if policy requires.

## Frontend (vanilla SPA)

| Check | Result | Notes |
|-------|--------|--------|
| **eslint + eslint-plugin-security** | Pass | Pinned devDependencies (eslint 10.15.0, plugin 4.2.1) with committed `package-lock.json` |
| **npm audit** | Dev-only deps | Install-time audit via `npm ci` |
| **Manual pattern scan** | Review | Heavy use of `innerHTML` for UI rendering |

### `innerHTML` note

The SPA builds HTML strings for lists/cards. User-controlled fields (notes, names, tags) go through `escapeHtml` / `esc` / `escapeAttr` helpers in `js/tags.js`, `admin.js`, `patient.js`, `partner.js`, `clinician.js`, `profile.js`, `notifications.js`, `dialog.js`. Prefer keeping that discipline; consider `textContent` + DOM APIs for new features.

No `eval`, `new Function`, or hardcoded secrets found in app JS.

## How to re-run

```bash
# All
make security

# Individually
make govulncheck
make gosec
make lint-fe
```
