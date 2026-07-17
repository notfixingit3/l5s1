# L5S1 Health Registry

Passwordless multi-condition health tracking (PWA) — WebAuthn passkeys, partner observation, clinician summary, admin invites/tags.

| Branch | Contents |
|--------|----------|
| **`main`** | This landing page only (stable pointer) |
| **`dev`** | Active development, CI, and container builds |

**Current pre-release:** [`v0.0.1-beta.14`](https://github.com/notfixingit3/l5s1/releases/tag/v0.0.1-beta.14) on `dev`

## Get the app

### Docker (recommended)

```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.14

docker run --rm -p 8080:8080 \
  -e WEBAUTHN_RP_ID=localhost \
  -e WEBAUTHN_ORIGINS=http://localhost:8080 \
  -v l5s1-data:/data \
  ghcr.io/notfixingit3/l5s1:v0.0.1-beta.14
```

Then open **http://localhost:8080** (must be `localhost` for passkeys).

| Image tag | Notes |
|-----------|--------|
| `v0.0.1-beta.14` | This beta |
| `latest` | Newest git tag |
| `dev` | Rolling build from development branch |

Packages: [ghcr.io/notfixingit3/l5s1](https://github.com/notfixingit3/l5s1/pkgs/container/l5s1)

## Develop

```bash
git clone -b dev git@github.com:notfixingit3/l5s1.git
cd l5s1
make up
```

Full docs live on the [`dev` branch README](https://github.com/notfixingit3/l5s1/blob/dev/README.md).

## Sponsor

[Sponsor @nottellingit3](https://github.com/sponsors/nottellingit3)

## Status

Pre-release / beta. APIs and schema may change without notice until `v0.1.0`.
