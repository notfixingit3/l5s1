# Gate 1 — Brainstorm: WebAuthn Go Server Initialization

## Library Choice

| Option | Status | Decision |
|--------|--------|----------|
| `github.com/duo-labs/webauthn` | **Deprecated** | Do not use |
| `github.com/go-webauthn/webauthn` | Active, FIDO2-conformant | **Selected** |

L5S1 uses **passwordless multi-device passkeys**. The community fork `go-webauthn/webauthn` is the supported path.

## Relying Party Initialization (validated)

```go
config := &webauthn.Config{
    RPDisplayName: "L5S1 Health Registry",
    RPID:          "localhost",                          // prod: real domain, no scheme
    RPOrigins:     []string{"http://localhost:8080"},    // prod: https://app.example.com
}
wa, err := webauthn.New(config)
```

### Critical constraints

1. **RPID** must match the effective domain (no `https://`, no port for production domains).
2. **RPOrigins** must exactly match the browser origin(s) making WebAuthn API calls.
3. Local dev often needs `localhost` + HTTP; production requires HTTPS (except localhost).
4. `SessionData` from `Begin*` must be stored **server-side** (opaque cookie / memory store), never trusted from the client.
5. After every successful login, **persist updated `SignCount`** (and clone-warning flags) or clone detection breaks.

## User interface contract

`User` must implement `webauthn.User`:

| Method | L5S1 mapping |
|--------|----------------|
| `WebAuthnID()` | Stable UUID bytes for the user (User Handle) |
| `WebAuthnName()` | Email |
| `WebAuthnDisplayName()` | Email or display name |
| `WebAuthnCredentials()` | All stored `Credential` rows for multi-device |

## Ceremony endpoints (blueprint)

| Step | Method | Path | Library call |
|------|--------|------|--------------|
| Register begin | POST | `/api/auth/register/begin` | `BeginRegistration` + resident-key opts |
| Register finish | POST | `/api/auth/register/finish` | `FinishRegistration` → persist credential |
| Login begin | POST | `/api/auth/login/begin` | `BeginLogin` or discoverable login |
| Login finish | POST | `/api/auth/login/finish` | `FinishLogin` → update sign count + session |

## Multi-device model

- One `User` → many `Credential` rows (phone, laptop, tablet).
- Registration uses **exclude credentials** so the same authenticator is not re-registered.
- Admin can **revoke a single credential** (device) without killing the account.
- Force re-registration = deactivate all credentials + set user flag / require new passkey.

## Credential storage fields (minimum)

| Field | Purpose |
|-------|---------|
| `ID` (credential ID bytes) | Lookup key on assertion |
| `UserID` | App user FK |
| `PublicKey` | Verify signature |
| `SignCount` | Clone detection |
| `Attestation` / format | Audit / policy |
| `DeviceType` | UX label for admin revoke UI |

## Security notes for L5S1

- No passwords anywhere — auth is WebAuthn only.
- Admin routes (`/api/admin/*`) require `role == admin` session middleware.
- Partner access is **relationship-scoped** (`PartnerAccess`), not global by role alone.
- Signup gated by config flag `ALLOW_SIGNUPS` (in-memory cache + DB-backed `AppConfig`).

## Validation outcome

**PASS** — Initialization steps are clear and implementable with Gin + GORM + go-webauthn. Proceed to file tree and scaffolding.
