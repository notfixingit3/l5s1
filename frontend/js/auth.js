import { api } from "./api.js";

/** base64url helpers for WebAuthn binary fields */
function bufferToBase64url(buffer) {
  const bytes = new Uint8Array(buffer);
  let str = "";
  for (let i = 0; i < bytes.length; i++) {
    str += String.fromCharCode(bytes[i]);
  }
  return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64urlToBuffer(base64url) {
  if (base64url instanceof ArrayBuffer) return base64url;
  if (typeof base64url !== "string") {
    throw new Error("expected base64url string for binary field");
  }
  const pad = "=".repeat((4 - (base64url.length % 4)) % 4);
  const base64 = (base64url + pad).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const buf = new ArrayBuffer(raw.length);
  const view = new Uint8Array(buf);
  for (let i = 0; i < raw.length; i++) view[i] = raw.charCodeAt(i);
  return buf;
}

/**
 * PublicKeyCredential fields are prototype getters — Object.keys() is empty.
 * Build the JSON shape go-webauthn expects explicitly.
 */
function credentialToJSON(cred) {
  if (!cred) throw new Error("empty credential");

  const out = {
    id: cred.id,
    rawId: bufferToBase64url(cred.rawId),
    type: cred.type || "public-key",
  };

  if (cred.authenticatorAttachment) {
    out.authenticatorAttachment = cred.authenticatorAttachment;
  }

  const resp = cred.response;
  if (!resp) throw new Error("credential missing response");

  if (typeof resp.attestationObject !== "undefined") {
    out.response = {
      clientDataJSON: bufferToBase64url(resp.clientDataJSON),
      attestationObject: bufferToBase64url(resp.attestationObject),
    };
    if (typeof resp.getTransports === "function") {
      try {
        out.response.transports = resp.getTransports();
      } catch {
        /* ignore */
      }
    }
    return out;
  }

  out.response = {
    clientDataJSON: bufferToBase64url(resp.clientDataJSON),
    authenticatorData: bufferToBase64url(resp.authenticatorData),
    signature: bufferToBase64url(resp.signature),
    userHandle: resp.userHandle ? bufferToBase64url(resp.userHandle) : null,
  };
  return out;
}

function prepareCreationOptions(options) {
  const publicKey = options.publicKey || options;
  const copy = structuredClone
    ? structuredClone(publicKey)
    : JSON.parse(JSON.stringify(publicKey));

  copy.challenge = base64urlToBuffer(copy.challenge);
  if (copy.user?.id) {
    copy.user.id = base64urlToBuffer(copy.user.id);
  }
  if (Array.isArray(copy.excludeCredentials)) {
    copy.excludeCredentials = copy.excludeCredentials.map((c) => ({
      ...c,
      id: base64urlToBuffer(c.id),
    }));
  }
  return { publicKey: copy };
}

function prepareRequestOptions(options) {
  const publicKey = options.publicKey || options;
  const copy = structuredClone
    ? structuredClone(publicKey)
    : JSON.parse(JSON.stringify(publicKey));

  copy.challenge = base64urlToBuffer(copy.challenge);
  if (Array.isArray(copy.allowCredentials)) {
    copy.allowCredentials = copy.allowCredentials.map((c) => ({
      ...c,
      id: base64urlToBuffer(c.id),
    }));
  }
  return { publicKey: copy };
}

/** Best-effort device label for admin multi-device list — never shown in the form. */
export function detectDeviceLabel() {
  const ua = navigator.userAgent || "";
  if (/iPhone/i.test(ua)) return "iPhone";
  if (/iPad/i.test(ua)) return "iPad";
  if (/Android/i.test(ua)) return "Android";
  if (/Mac OS X|Macintosh/i.test(ua)) return "Mac";
  if (/Windows/i.test(ua)) return "Windows";
  if (/CrOS/i.test(ua)) return "Chromebook";
  if (/Linux/i.test(ua)) return "Linux";
  return "Device";
}

/** Normalize typed codes (accepts 12345678 or 1234-5678). */
export function normalizeCode(raw) {
  return String(raw || "").replace(/\D/g, "").slice(0, 8);
}

/** Display form xxxx-xxxx */
export function formatCodeDisplay(raw) {
  const d = normalizeCode(raw);
  if (d.length !== 8) return String(raw || "").trim();
  return `${d.slice(0, 4)}-${d.slice(4)}`;
}

/**
 * @param {{ username?: string, email?: string, display_name?: string, invite_code?: string, device_link_code?: string, device_type?: string }} profile
 */
export async function registerPasskey(profile) {
  const body = {
    username: profile.username || "",
    email: profile.email || "",
    display_name: profile.display_name || "",
    invite_code: profile.invite_code || "",
    device_link_code: profile.device_link_code || "",
    device_type: profile.device_type || detectDeviceLabel(),
  };
  const options = await api("/api/auth/register/begin", {
    method: "POST",
    body,
  });
  let cred;
  try {
    cred = await navigator.credentials.create(prepareCreationOptions(options));
  } catch (err) {
    throw new Error(err?.message || "Passkey creation failed or was cancelled");
  }
  if (!cred) throw new Error("Passkey creation cancelled");

  return api("/api/auth/register/finish", {
    method: "POST",
    body: credentialToJSON(cred),
  });
}

export async function loginPasskey(identifier) {
  const options = await api("/api/auth/login/begin", {
    method: "POST",
    body: { username: identifier },
  });
  let assertion;
  try {
    assertion = await navigator.credentials.get(prepareRequestOptions(options));
  } catch (err) {
    throw new Error(err?.message || "Passkey sign-in failed or was cancelled");
  }
  if (!assertion) throw new Error("Passkey sign-in cancelled");

  return api("/api/auth/login/finish", {
    method: "POST",
    body: credentialToJSON(assertion),
  });
}

export async function logout() {
  return api("/api/auth/logout", { method: "POST", body: {} });
}

export async function me() {
  return api("/api/auth/me");
}

export async function updateProfile(fields) {
  return api("/api/auth/profile", {
    method: "PATCH",
    body: fields,
  });
}
