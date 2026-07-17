# L5S1 Architecture

## System intent

Progressive Web App for multi-condition health tracking (lumbar spinal stenosis, UC, blood pressure, type 2 diabetes) with:

- Ultra-low-friction mobile log entry
- Passwordless multi-device WebAuthn
- Partner observation access
- Clinician “since last appointment” presentation mode
- Admin lifecycle controls for users and passkeys

## Stack

| Layer | Choice |
|-------|--------|
| API | Go + Gin |
| ORM / DB | GORM + SQLite (dev) / PostgreSQL (prod-ready models) |
| Auth | `github.com/go-webauthn/webauthn` (no passwords) |
| UI | Static SPA + PWA service worker |

## Domain model

```
User 1──* Credential          (multi-device passkeys)
User 1──* PartnerAccess       (as patient)
User 1──* PartnerAccess       (as partner, via PartnerID)
User 1──* HealthLog           (subject of log)
User 1──* HealthLog           (author via AuthorID)
AppConfig *                   (key/value runtime flags)
```

### Roles

| Role | Capabilities |
|------|----------------|
| `patient` | Own logs, grant partners, multi-device passkeys |
| `partner` | View authorized patient timeline; write observations if `CanWrite` |
| `admin` | `/api/admin/*`: toggles, revoke devices, activate/deactivate users |

## API surface (v1)

### Auth

- `POST /api/auth/register/begin`
- `POST /api/auth/register/finish`
- `POST /api/auth/login/begin`
- `POST /api/auth/login/finish`
- `POST /api/auth/logout`
- `GET  /api/auth/me`

### Health logs

- `POST /api/logs` — patient (or write-enabled partner) create
- `GET  /api/logs` — list for self
- `GET  /api/logs/summary?since=` — clinician aggregates

### Partner

- `GET  /api/partner/patients` — patients this partner can see
- `GET  /api/partner/patients/:id/logs`
- `POST /api/partner/patients/:id/observations`

### Admin

- `GET/PUT /api/admin/config`
- `GET     /api/admin/users`
- `PATCH   /api/admin/users/:id` — active / force re-register
- `DELETE  /api/admin/users/:id/credentials/:credId` — revoke device

## Frontend modes

1. **Patient** — pain slider 1–10, short notes, tags (`uc-flare`, `bp-high`, …)
2. **Partner** — timeline + “Observations for Doctor”
3. **Clinician** — filter since last appointment; averages and trends

## Config cache

`services.ConfigCache` loads `AppConfig` rows into memory; admin PUT updates DB + cache atomically for hot flags such as `ALLOW_SIGNUPS`.
