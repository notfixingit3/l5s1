# Security scan report (L5S1)

Run locally:

```bash
make security
```

## Backend

| Tool | Result | Notes |
|------|--------|--------|
| **go vet** | Pass | No issues |
| **gosec** | Pass (0 issues) | Fixed G301: data dir `MkdirAll` now `0750` |
| **govulncheck** | Pass (0 called vulns) | Bumped `golang.org/x/net` **v0.25.0 → v0.38.0** (GO-2025-3595 HTML neutralization) |
| **go test** | Pass | All package tests |

### Residual / accepted

- govulncheck still reports **transitive** vulns in unused import paths of dependencies (not reachable from this app’s call graph). Re-check after major dependency upgrades.
- In-memory session store is fine for local/dev; production should use a shared store and HTTPS (`SECURE_COOKIE=true`).

## Frontend (vanilla SPA)

| Check | Result | Notes |
|-------|--------|--------|
| **eslint + eslint-plugin-security** | Pass | Config in `frontend/eslint.config.js` |
| **npm audit** (eslint tooling) | 0 vulns on install | Dev-only deps |
| **Manual pattern scan** | Review | Heavy use of `innerHTML` for UI rendering |

### `innerHTML` note

The SPA builds HTML strings for lists/cards. User-controlled fields (notes, names, tags) go through `escapeHtml` / `esc` / `escapeAttr` helpers in `js/tags.js`, `admin.js`, `patient.js`, `partner.js`, `clinician.js`, `profile.js`. Prefer keeping that discipline; consider `textContent` + DOM APIs for new features.

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
