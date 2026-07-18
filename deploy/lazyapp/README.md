# L5S1 hosted stack (Traefik + GHCR)

Serves the published image at **https://l5s1.com** (and `www`).

| Item | Value |
|------|--------|
| Image | `ghcr.io/notfixingit3/l5s1:latest` or pin `v0.0.1-beta.N` |
| Data | Host bind `./data` → `/data` (SQLite) |
| Network | external `lazyapp_me` (Traefik edge) |
| TLS | Let's Encrypt via Traefik `certResolver=lets-encrypt` |

## Setup

```bash
mkdir -p data
cp .env.example .env
# set CROWDSEC_BOUNCER_KEY; confirm APP_HOST / WebAuthn values

docker compose pull
docker compose up -d
curl -fsS https://l5s1.com/api/healthz
```

Refresh after a new beta tag:

```bash
docker compose pull && docker compose up -d
```

## DNS

Point both names at the Traefik host:

| Name | Type | Value |
|------|------|--------|
| `l5s1.com` | A | edge server public IP |
| `www.l5s1.com` | A or CNAME | same IP / `l5s1.com` |

Until DNS resolves, Traefik cannot issue certificates.

## WebAuthn

- `WEBAUTHN_RP_ID=l5s1.com` (domain only)
- `WEBAUTHN_ORIGINS` must include every origin users use (`https://l5s1.com` and `https://www.l5s1.com` if both are served)
- Passkeys registered under an old host (e.g. a lab subdomain) **will not work** after the RP ID change — users re-register on the new domain

## Web Push

- Optional env: `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`
- If unset, the app generates VAPID keys on first start and stores them in SQLite (`app_configs`)
- Users: **Profile → Push notifications → Enable on this device** (HTTPS; iOS needs Home Screen PWA)

## Secrets

Never commit real `.env` (CrowdSec keys, etc.). Keep this template generic.
