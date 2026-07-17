# Docker + passkeys (local development)

## Goal

Run L5S1 in Docker for reproducible local testing **without losing registered passkeys** every time the image rebuilds.

## What is persisted vs not

| State | Storage | Survives container restart? | Survives image rebuild? |
|-------|---------|-----------------------------|-------------------------|
| Passkeys (credential ID + public key + sign count) | `./data/l5s1.db` | Yes | Yes |
| Users, health logs, partner grants, admin flags | `./data/l5s1.db` | Yes | Yes |
| Logged-in browser session cookie | In-memory in process | **No** — re-login | No |
| WebAuthn ceremony (begin→finish) | In-memory | No (seconds-lived) | No |

**Passkeys are durable. Sessions are not.** After `docker compose restart`, unlock with the same passkey — you do not re-register.

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

## Production note

For real deploys: HTTPS, real domain as RPID, `SECURE_COOKIE=true`, and a managed volume or Postgres — not `localhost`.
