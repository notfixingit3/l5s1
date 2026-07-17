# Multi-device passkeys (device link codes)

## Problem

Login needs a passkey **on this device**. Adding a passkey used to require an
already-signed-in session — so a second phone/laptop had no way to enroll.

## Solution

Signed-in users mint a short-lived **device link code** (8 digits, display
`xxxx-xxxx`). On the new device they open **Add device**, enter username +
code, and register a passkey.

| | Invite codes | Device link codes |
|--|--------------|-------------------|
| Who mints | Admin | Account owner |
| Purpose | New account | Extra passkey on existing user |
| Format | 8 digits → `1234-5678` | Same |
| Default TTL | Optional (days) | **20 minutes** |
| Uses | Configurable | **1** |
| Max active | — | **3** per user |

## Security

- Codes stored as 8 digits (hyphen is display-only).
- Device codes are bound to `user_id`; redeem requires matching username.
- Single-use; redeem only after successful WebAuthn registration.
- Max 3 concurrent usable codes; user can revoke unused ones.
- Attempt limiter: **12 failures / 15 min** per IP (and per username+IP for
  device codes). Generic error messages avoid user enumeration.
- Codes unique across invite + device-link tables to avoid mix-ups.

## API

| Method | Path | Auth |
|--------|------|------|
| POST | `/api/auth/device-codes` | Session |
| GET | `/api/auth/device-codes` | Session |
| DELETE | `/api/auth/device-codes/:id` | Session |
| POST | `/api/auth/register/begin` | Body: `device_link_code` + `username` |

## UX flow

1. **Device A** — Profile → *Generate device code* → show `1234-5678`
2. **Device B** — Auth → *Add device* → username + code → create passkey
3. Device B is signed in; both passkeys appear under Profile
