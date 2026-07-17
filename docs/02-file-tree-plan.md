# Gate 2 — Plan: Local File Tree

```
L5S1/
├── README.md
├── docs/
│   ├── 01-brainstorm-webauthn.md
│   ├── 02-file-tree-plan.md
│   ├── 03-execution-walkthrough.md
│   └── ARCHITECTURE.md
├── backend/
│   ├── go.mod
│   ├── cmd/
│   │   └── server/
│   │       └── main.go                 # Process entry: config, DB migrate, Gin listen
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go               # Env + defaults (DB, RP, port)
│   │   ├── models/
│   │   │   └── models.go               # User, Credential, PartnerAccess, HealthLog, AppConfig
│   │   ├── database/
│   │   │   └── database.go             # GORM open + AutoMigrate
│   │   ├── auth/
│   │   │   ├── webauthn.go             # RP init, User adapter, ceremony helpers
│   │   │   └── session.go              # In-memory ceremony + app session store
│   │   ├── middleware/
│   │   │   └── auth.go                 # RequireAuth, RequireAdmin, RequireRole
│   │   ├── handlers/
│   │   │   ├── auth.go                 # register/login begin/finish
│   │   │   ├── health.go               # HealthLog CRUD + clinician summary
│   │   │   ├── partner.go              # Partner observations + timeline
│   │   │   └── admin.go                # Config toggles, users, revoke passkeys
│   │   ├── services/
│   │   │   └── config_cache.go         # Global flag cache (ALLOW_SIGNUPS, etc.)
│   │   └── router/
│   │       └── router.go               # Gin route groups
│   └── tests/
│       ├── multidevice_test.go         # Multi-credential attach + sign-count
│       └── admin_toggle_test.go        # ALLOW_SIGNUPS + revoke credential
└── frontend/
    ├── index.html                      # SPA shell
    ├── manifest.webmanifest            # PWA manifest
    ├── sw.js                           # Offline cache service worker
    ├── css/
    │   └── app.css
    └── js/
        ├── app.js                      # Router / mode switch (patient|partner|clinician)
        ├── auth.js                     # WebAuthn client ceremonies
        ├── api.js                      # Fetch helpers
        ├── patient.js                  # 1–10 pain + quick tags
        ├── partner.js                  # Observations for doctor
        └── clinician.js                # Since-last-appointment summary
```

## Layering rules

1. **handlers** call **services/auth/database** only; no SQL in handlers beyond GORM via models.
2. **models** stay free of HTTP types.
3. **frontend** is a static SPA; backend serves it in production or CORS for dev.
4. SQLite by default; `DATABASE_URL` can switch to PostgreSQL later without model changes.
