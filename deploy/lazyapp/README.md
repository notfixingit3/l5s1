# L5S1 reverse-proxy / lab compose

Generic stack for hosting the published GHCR image behind Traefik (HTTPS + optional CrowdSec).

| Item | Value |
|------|--------|
| Image | `ghcr.io/notfixingit3/l5s1:dev` (override with `IMAGE_TAG`) |
| Data | **Host bind** `./data` → `/data` (SQLite) — not a named Docker volume |
| Network | external Docker network (default name `lazyapp_me`; change if yours differs) |

Copy this directory to your host, fill `.env` from `.env.example`, then:

```bash
mkdir -p data
cp .env.example .env   # set PREVIEW_HOST, WebAuthn origin, CROWDSEC_BOUNCER_KEY
docker compose pull
docker compose up -d
```

Refresh to the latest image:

```bash
docker compose pull && docker compose up -d
```

### WebAuthn

Set `WEBAUTHN_RP_ID` and `WEBAUTHN_ORIGINS` to your **public HTTPS hostname** (not `localhost`).  
Passkeys created on localhost will not work on the public host and vice versa.

### Data

Keep SQLite on a host path next to compose (`./data`). Do not switch to a named Docker volume unless you intentionally want that.

### Secrets

- Never commit `.env` (CrowdSec keys, etc.).
- Do not document private SSH hosts, usernames, or internal filesystem layouts in this repo.
