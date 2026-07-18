# AGENTS.md — L5S1 working notes

Guidance for humans and coding agents. Prefer this file for **process and constraints**; deep design lives under `docs/`.

## What this is

Passwordless multi-condition health PWA: WebAuthn passkeys, patient logs, partner observation, clinician summary, admin invites/tags.

| Layer | Stack |
|-------|--------|
| API | Go, Gin, GORM |
| DB | SQLite (pure Go `glebarez/sqlite`) locally; Postgres later optional |
| Auth | `go-webauthn/webauthn` only — **no passwords** |
| Frontend | Vanilla SPA under `frontend/` (no heavy framework) |
| Images | Multi-arch Docker → GHCR `ghcr.io/notfixingit3/l5s1` |

Repo: **https://github.com/notfixingit3/l5s1**  
Branches: **`dev`** = app; **`main`** = thin landing README only.

## Local first — GitHub only on beta tags

**Do not rely on Actions for day-to-day iteration.** Test locally, then cut a beta.

```bash
make up        # Docker, passkeys in ./data
make test      # Go tests
make security  # vet + gosec + govulncheck + eslint
make run       # host process, same ./data
```

| Event | CI | Container (GHCR multi-arch) |
|-------|----|------------------------------|
| Push to `dev` | No | No |
| PR | No | No |
| Tag `v*-beta.*` | Yes | Yes (parallel native amd64 + arm64) |
| `workflow_dispatch` | Yes | Yes |

Container builds: **no QEMU** — matrix on `ubuntu-latest` + `ubuntu-24.04-arm`, then merge digests.  
Dockerfile skips `go test` (CI / `make test` cover that). Expect ~3–6 min vs ~15–18 min QEMU multi-arch.

When ready to release:

1. Bump `VERSION` (e.g. `0.0.1-beta.17`)
2. Update `CHANGELOG.md`, defaults in `Dockerfile` / `version.go` / docs as needed
3. Commit on `dev`
4. `git tag -a v0.0.1-beta.N -m "…" && git push origin dev && git push origin v0.0.1-beta.N`
5. Optionally refresh `main` landing from `README.main.md`

Image tags published on beta cut: `v0.0.1-beta.N`, `latest`, `dev` (alias of newest beta), `sha-<short>`.

## Version display

- Product version = clean string from `VERSION` / tag (e.g. `0.0.1-beta.17`)
- Commit stamped **separately** via ldflags / `version.js`
- Display: `v0.0.1-beta.17` or `v0.0.1-beta.17+abc1234`
- **Never** bake `-g{sha}` into the version string *and* append `+{sha}` (doubled hash)

## WebAuthn / passkeys (critical)

| Rule | Detail |
|------|--------|
| Local open URL | **http://localhost:8080** only — not `127.0.0.1` |
| `WEBAUTHN_RP_ID` | Stable per environment (`localhost` local; public hostname in hosted) |
| Origins | Exact match to browser origin |
| Storage | Credential public keys + users in SQLite under **`./data`** (bind mount) |
| Sessions | In-memory — re-login after restart; passkeys remain |
| Multi-device | One user → many credentials |

### Second device bootstrap

Users **cannot** log in on a new device until it has a passkey.

1. Device A (signed in): **Profile → Generate device code** (`xxxx-xxxx`, 20 min, single use)
2. Device B: Auth → **Add device** → username + code → register passkey

Admin **invite** codes (`xxxx-xxxx`) are for **new accounts**, not extra devices.  
See `docs/06-multi-device-codes.md`.

## Codes (invites + device links)

- Stored as **8 digits**; UI shows **`xxxx-xxxx`**
- Normalize: strip non-digits before validate
- Rate-limit failed attempts (in-memory limiter)
- Device codes: max 3 active / user, user-bound, redeem only after successful WebAuthn finish

## Hosted deploy

Template: `deploy/lazyapp/` (Traefik labels, host bind `./data`, GHCR image).

```bash
# on the host (paths stay local — do not commit them)
cp .env.example .env   # set PREVIEW_HOST, WebAuthn, CrowdSec key
mkdir -p data
docker compose pull && docker compose up -d
```

- Prefer **host bind** `./data` — not named Docker volumes for SQLite
- `SECURE_COOKIE=true` + HTTPS + matching RP ID/origin
- Default image pin: `IMAGE_TAG=latest` or a specific `v0.0.1-beta.N`

## Do **not** put in git

| Keep out of the repo | OK in repo |
|----------------------|------------|
| SSH users / private hostnames | Public GitHub org / image names |
| `/opt/stacks/...` and sibling stack paths | Generic compose under `deploy/` |
| Real `.env`, CrowdSec keys, tokens | `.env.example` with placeholders |
| Lab-only inventory | Public product docs |

## Data / safety

- Never commit `./data` or `*.db`
- `make data-backup` / `make data-reset` (reset requires typing `YES`)
- Seeded shells (admin / demo users) may exist without passkeys — first bind via Create account where allowed

## Layout (short)

```
backend/          Go module (cmd/server, internal/*)
frontend/         SPA + PWA assets
branding/         Source logos; app copies under frontend/assets/brand
deploy/lazyapp/   Hosted compose template
docs/             Design + ops notes (01…06, ARCHITECTURE)
VERSION           Product version string
CHANGELOG.md      Release notes
```

## Commands cheatsheet

| Goal | Command |
|------|---------|
| Local stack | `make up` |
| Tests | `make test` |
| Security suite | `make security` |
| GHCR locally | `make pull-ghcr` (`IMAGE_TAG=…`) |
| Logs | `make logs` |
| Stop (keep data) | `make down` |

## Docs index

| Topic | Path |
|-------|------|
| Architecture | `docs/ARCHITECTURE.md` |
| WebAuthn brainstorm | `docs/01-brainstorm-webauthn.md` |
| Docker + passkeys | `docs/04-docker-passkeys.md` |
| Security scans | `docs/05-security-scan.md` |
| Multi-device codes | `docs/06-multi-device-codes.md` |
| Changelog | `CHANGELOG.md` |

When unsure: **local test → small commits on `dev` → beta tag for CI/images**. Keep secrets and private lab topology out of the tree.
