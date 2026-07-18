# L5S1 Health Registry

**Live:** [https://l5s1.com](https://l5s1.com)

Passwordless multi-condition health tracking (PWA) — WebAuthn passkeys, partner observation, clinician summary, admin invites/tags.

Built for people managing overlapping conditions (e.g. stenosis, UC, blood pressure, glucose) who want a **low-friction private log** without passwords.

| | |
|--|--|
| **App** | [l5s1.com](https://l5s1.com) |
| **Code** | Active work on [`dev`](https://github.com/notfixingit3/l5s1/tree/dev) · this branch is the landing pointer |
| **Images** | [ghcr.io/notfixingit3/l5s1](https://github.com/notfixingit3/l5s1/pkgs/container/l5s1) |
| **Current beta** | [`v0.0.1-beta.29`](https://github.com/notfixingit3/l5s1/releases/tag/v0.0.1-beta.29) |

## What it does

- **Passwordless sign-in** with multi-device passkeys (add a phone/laptop via a one-time device code)
- **Daily check-in** — pain 1–10, notes, curated tags (swipe to edit or delete)
- **Partner mode** — care partners can observe and leave notes
- **Clinician summary** — averages, tag frequency, timeline for visits
- **Admin** — invites, users, passkeys, tag catalog

## Run with Docker

```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.29

docker run --rm -p 8080:8080 \
  -e WEBAUTHN_RP_ID=localhost \
  -e WEBAUTHN_ORIGINS=http://localhost:8080 \
  -v l5s1-data:/data \
  ghcr.io/notfixingit3/l5s1:v0.0.1-beta.29
```

Open **http://localhost:8080** (must be `localhost` for local passkeys).

| Image tag | Notes |
|-----------|--------|
| `v0.0.1-beta.29` | This beta |
| `latest` | Newest beta tag |

## Develop

```bash
git clone -b dev git@github.com:notfixingit3/l5s1.git
cd l5s1
make up
```

Full docs: [`dev` branch README](https://github.com/notfixingit3/l5s1/blob/dev/README.md) · [AGENTS.md](https://github.com/notfixingit3/l5s1/blob/dev/AGENTS.md)

## Sponsor

[Sponsor @nottellingit3](https://github.com/sponsors/notfixingit3)

## Status

Pre-release / beta. APIs and schema may change until `v0.1.0`.
