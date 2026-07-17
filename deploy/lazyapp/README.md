# L5S1 lab stack (`l5s1_lazyapp`)

Deploy path on the lab host:

```text
house@notfixingit:/opt/stacks/l5s1_lazyapp
```

Mirrors the pattern used by `askaway_lazyapp` / other `*_lazyapp` stacks: Traefik on `lazyapp_me`, Let's Encrypt, CrowdSec bouncer, GHCR image.

| Item | Value |
|------|--------|
| URL | https://l5s1.lazyapp.me |
| Image | `ghcr.io/notfixingit3/l5s1:dev` (override with `IMAGE_TAG`) |
| Data | **Host bind** `./data` → `/data` (SQLite at `./data/l5s1.db`) — not a named Docker volume |
| Network | external `lazyapp_me` |

## First-time setup (lab host)

```bash
ssh house@notfixingit
sudo mkdir -p /opt/stacks/l5s1_lazyapp   # if needed; chown house
cd /opt/stacks/l5s1_lazyapp

# copy compose + env from repo (or rsync deploy/lazyapp/)
cp /path/to/L5S1/deploy/lazyapp/docker-compose.yml .
cp /path/to/L5S1/deploy/lazyapp/.env.example .env
# set CROWDSEC_BOUNCER_KEY (same as askaway_lazyapp)
grep CROWDSEC /opt/stacks/askaway_lazyapp/.env >> .env   # or paste manually

mkdir -p data
docker compose pull
docker compose up -d
docker compose ps
curl -fsS https://l5s1.lazyapp.me/api/healthz
```

Open **https://l5s1.lazyapp.me** (must match `WEBAUTHN_RP_ID` / origins for passkeys).

## Refresh to latest `dev` build

```bash
cd /opt/stacks/l5s1_lazyapp
docker compose pull
docker compose up -d
```

`pull_policy: always` also re-pulls on recreate. SQLite lives on the host at  
`/opt/stacks/l5s1_lazyapp/data/l5s1.db` (bind mount). Pull/up does not touch it.  
Do **not** switch this to a named `volumes:` block — lab stacks keep data on disk next to compose.

Pin a release instead of rolling dev:

```bash
# in .env
IMAGE_TAG=v0.0.1-beta.15
docker compose pull && docker compose up -d
```

## Notes

- Passkeys registered against `localhost` will **not** work on the lab host; register new ones under `l5s1.lazyapp.me`.
- Package may be private — lab host needs `docker login ghcr.io` (already configured for other stacks).
- CrowdSec bouncer key is required or Traefik middleware config will be empty.
