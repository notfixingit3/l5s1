# L5S1 Health Registry — development branch

> **Live app:** [https://l5s1.com](https://l5s1.com)  
> Active development lives on **`dev`**. **`main`** is a short landing README.

**Current version:** see [`VERSION`](./VERSION) → **v0.0.1-beta.23**

<p align="center">
  <img src="branding/logo-lockup-readme.png" alt="L5S1" width="220" />
</p>

[![CI](https://github.com/notfixingit3/l5s1/actions/workflows/ci.yml/badge.svg)](https://github.com/notfixingit3/l5s1/actions/workflows/ci.yml)
[![Container](https://github.com/notfixingit3/l5s1/actions/workflows/container.yml/badge.svg)](https://github.com/notfixingit3/l5s1/actions/workflows/container.yml)
[![Website](https://img.shields.io/badge/web-l5s1.com-0d9488)](https://l5s1.com)

## About

L5S1 is a **passwordless progressive web app** for multi-condition health tracking. It is designed for people managing overlapping issues (lumbar stenosis / L5–S1 focus, IBD flares, blood pressure, glucose, and similar) who want a **private, low-friction log** they can use daily and share selectively with a care partner or clinician.

| | |
|--|--|
| **Production** | [https://l5s1.com](https://l5s1.com) |
| **Auth** | WebAuthn passkeys only — no passwords stored on the server |
| **Stack** | Go (Gin) + SQLite · vanilla SPA/PWA · multi-arch images on GHCR |
| **Multi-device** | User-minted 8-digit device codes (`xxxx-xxxx`) to enroll another phone/laptop |

### Product surface

- **Patient** — pain 1–10, notes, curated tags; swipe recent entries to edit or delete  
- **Partner** — observation access granted by the patient  
- **Clinician summary** — averages, histograms, tag frequency, timeline  
- **Admin** — invite codes, user lock/roles/email, passkey revoke, tag catalog (default vs custom)

## Container images (GHCR)

Images build on **`v*-beta.*` tags only** (not every `dev` push).

| Tag | Meaning |
|-----|---------|
| `ghcr.io/notfixingit3/l5s1:v0.0.1-beta.23` | Immutable beta tag |
| `ghcr.io/notfixingit3/l5s1:latest` | Newest beta tag |
| `ghcr.io/notfixingit3/l5s1:dev` | Alias of newest beta |
| `ghcr.io/notfixingit3/l5s1:sha-<short>` | Exact commit from that tag build |

### Run a tagged beta locally

```bash
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.23

docker run --rm -p 8080:8080 \
  -e WEBAUTHN_RP_ID=localhost \
  -e WEBAUTHN_ORIGINS=http://localhost:8080 \
  -e SEED_ADMIN_USERNAME=admin \
  -v l5s1-data:/data \
  ghcr.io/notfixingit3/l5s1:v0.0.1-beta.23
```

Open **http://localhost:8080** only (not `127.0.0.1`) so WebAuthn works.

Private packages may require:

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
```

### Hosted (production domain)

Template under [`deploy/lazyapp/`](./deploy/lazyapp/) — Traefik, Let's Encrypt, WebAuthn for **`l5s1.com`** / `www.l5s1.com`, SQLite on a host bind `./data`.

## Local development

Test **before** cutting a beta tag — GitHub Actions run only on `v*-beta.*`.

```bash
make up       # Docker (local image)
make test     # Go tests
make security # gosec + govulncheck + eslint
make run      # host Go process, shared ./data
```

```bash
curl -s http://localhost:8080/api/version
```

When ready: bump `VERSION` / `CHANGELOG.md`, commit,  
`git tag v0.0.1-beta.N && git push origin v0.0.1-beta.N` → CI + multi-arch GHCR.

## Docs

| Doc | Path |
|-----|------|
| **Agent / process notes** | [AGENTS.md](./AGENTS.md) |
| Changelog | [CHANGELOG.md](./CHANGELOG.md) |
| Architecture | [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) |
| Docker + passkeys | [docs/04-docker-passkeys.md](./docs/04-docker-passkeys.md) |
| Multi-device codes | [docs/06-multi-device-codes.md](./docs/06-multi-device-codes.md) |
| Security scans | [docs/05-security-scan.md](./docs/05-security-scan.md) |
| Hosted deploy | [deploy/lazyapp/README.md](./deploy/lazyapp/README.md) |

## Sponsors

Support development via [GitHub Sponsors](https://github.com/sponsors/notfixingit3).

## License

TBD — all rights reserved until a license is chosen.
