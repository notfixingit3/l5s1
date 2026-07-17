# L5S1 — development branch

> Active development lives on **`dev`**.  
> **`main`** only hosts a pointer README until we cut a stable release.

**Current version:** see [`VERSION`](./VERSION) → **v0.0.1-beta.14**

<p align="center">
  <img src="branding/logo-lockup-readme.png" alt="L5S1 Health Registry" width="360" />
</p>

[![CI](https://github.com/notfixingit3/l5s1/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/notfixingit3/l5s1/actions/workflows/ci.yml)
[![Container](https://github.com/notfixingit3/l5s1/actions/workflows/container.yml/badge.svg)](https://github.com/notfixingit3/l5s1/actions/workflows/container.yml)

Progressive Web App for multi-condition health tracking with passwordless multi-device WebAuthn, partner observation, clinician summary, and admin controls.

## Container images (GHCR)

Images are built by GitHub Actions on every push to `dev` and on every `v*` tag.

| Tag | Meaning |
|-----|---------|
| `ghcr.io/notfixingit3/l5s1:v0.0.1-beta.14` | Immutable release (this beta) |
| `ghcr.io/notfixingit3/l5s1:latest` | Latest **tagged** release |
| `ghcr.io/notfixingit3/l5s1:dev` | Latest build from `dev` branch |
| `ghcr.io/notfixingit3/l5s1:sha-<short>` | Exact commit |

### Run a tagged beta

```bash
# Pull
docker pull ghcr.io/notfixingit3/l5s1:v0.0.1-beta.14

# Run (local WebAuthn — open http://localhost:8080 only)
docker run --rm -p 8080:8080 \
  -e WEBAUTHN_RP_ID=localhost \
  -e WEBAUTHN_ORIGINS=http://localhost:8080 \
  -e SEED_ADMIN_USERNAME=admin \
  -v l5s1-data:/data \
  ghcr.io/notfixingit3/l5s1:v0.0.1-beta.14
```

Private packages may require:

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
```

### Compose (from this branch)

```bash
make up          # build local image
# or point docker-compose image at GHCR (see docker-compose.ghcr.yml)
```

Version is shown in the app footer / header pill and via:

```bash
curl -s http://localhost:8080/api/version
```

## Local development

```bash
make up       # Docker
make test     # Go tests
make security # gosec + govulncheck + eslint
make run      # host Go process, shared ./data
```

**Important:** Use **http://localhost:8080** (not `127.0.0.1`) so WebAuthn passkeys stay valid.

## Docs

| Doc | Path |
|-----|------|
| Changelog | [CHANGELOG.md](./CHANGELOG.md) |
| Architecture | [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) |
| Docker + passkeys | [docs/04-docker-passkeys.md](./docs/04-docker-passkeys.md) |
| Security scans | [docs/05-security-scan.md](./docs/05-security-scan.md) |

## Sponsors

Support development via [GitHub Sponsors](https://github.com/sponsors/notfixingit3).

## License

TBD — all rights reserved until a license is chosen.
