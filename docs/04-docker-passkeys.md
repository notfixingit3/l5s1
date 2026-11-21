# Docker + passkeys (local development)

## Goal

Run L5S1 in Docker for reproducible local testing **without losing registered passkeys** every time the image rebuilds.

## What is persisted vs not

| State | Storage | Survives container restart? | Survives image rebuild? |
|-------|---------|-----------------------------|-------------------------|
| Passkeys (credential ID + public key + sign count) | `./data/l5s1.db` | Yes | Yes |
| Users, health logs, partner grants, admin flags | `./data/l5s1.db` | Yes | Yes |
| Logged-in browser session | `./data/l5s1.db` (`app_sessions`) + in-memory cache | Yes | Yes |
| WebAuthn ceremony (begin→finish) | In-memory | No (seconds-lived) | No |

**Passkeys and sessions are durable.** After `docker compose restart`, you stay signed in; after a rebuild, unlock with the same passkey.

## Container user (non-root)

The image runs as a non-root `l5s1` user (uid set by the `APP_UID` build arg, default `10001`). `make up` / `make dev` pass `APP_UID=$(id -u)` so the container owns the bind-mounted `./data`. With plain `docker compose build` or a fresh host dir, chown it first:

```bash
chown -R 10001:10001 ./data   # or set APP_UID to your uid at build time
```

## First-boot flags (local vs hosted)

| Env | Default | Local compose | Purpose |
|-----|---------|---------------|---------|
| `ALLOW_SIGNUPS` | `false` | `true` | Allow binding a zero-credential shell's first passkey via Create account |
| `SEED_DEMO_ACCOUNTS` | `false` | `true` | Seed the tom/jess demo accounts |

- Brand-new accounts always need an **invite code**, regardless of these flags.
- A zero-credential shell (seeded admin, pre-created account) is claimable only while `ALLOW_SIGNUPS=true`.
- Hosted bootstrap: start with `ALLOW_SIGNUPS=true`, bind the admin passkey via Create account, then set the flag to `false` (Admin → config) or restart without the env. Never enable `SEED_DEMO_ACCOUNTS` on a hosted instance.
- At startup the server logs a warning while any admin account has no passkey bound.

## Why RPID must stay stable

WebAuthn binds credentials to a **Relying Party ID** and origin:

| Setting | Local value | Rule |
|---------|-------------|------|
| `WEBAUTHN_RP_ID` | `localhost` | Never change on a machine that already has passkeys |
| `WEBAUTHN_ORIGINS` | `http://localhost:8080` | Must match the URL bar exactly |

### Do

- Open **http://localhost:8080**
- Keep compose env at `WEBAUTHN_RP_ID=localhost`

### Don’t

| Anti-pattern | Result |
|--------------|--------|
| Open `http://127.0.0.1:8080` | Origin ≠ RP domain rules → registration/login fails or creates a different identity surface |
| Change RP ID to `l5s1.local` mid-project | Existing authenticator credentials become unusable for this RP |
| Store DB only inside the container (no volume) | `docker compose down` / rebuild loses all passkeys |
| Commit `./data` to git | Secrets/PII/passkey material in VCS |

## Commands

```bash
make up          # build + run, data in ./data
make logs
make down        # stop; keeps ./data
make data-backup # copy SQLite snapshot
make data-reset  # wipe passkeys (explicit YES)
make dev         # mount ./frontend for UI edits
```

Host and Docker can share the same DB file if both use `./data/l5s1.db` and the same RPID/origin:

```bash
make run   # host process, same ./data as Docker
```

Avoid running host + Docker against the same SQLite file **at the same time**.

## Backup / move machine

```bash
make data-backup
# copy data/backups/*.db or entire data/ folder
```

Restore: stop app, replace `data/l5s1.db`, start again. Passkeys work if the browser still has the private key material (platform authenticator) **and** RPID/origin still match.

## Hosted / lab deploy

A Traefik-oriented compose file lives under [`deploy/lazyapp/`](../deploy/lazyapp/).  
Copy it to your host, set `.env` (public hostname, WebAuthn, CrowdSec), bind `./data`, then:

```bash
docker compose pull && docker compose up -d
```

Use HTTPS, a real domain as `WEBAUTHN_RP_ID`, and `SECURE_COOKIE=true`.  
Passkeys are **not** shared with local `localhost` registrations.

## Production note

For real deploys: HTTPS, real domain as RPID, `SECURE_COOKIE=true`, `ALLOW_SIGNUPS=false` (default), demo seeding off, and a managed volume or Postgres — not `localhost`.
