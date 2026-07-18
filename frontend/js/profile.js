import { api } from "./api.js";
import { detectDeviceLabel, formatCodeDisplay, me, registerPasskey, updateProfile } from "./auth.js";
import { appConfirm } from "./dialog.js";
import { invalidateTagCatalog } from "./tags.js";

let onProfileChange = null;
let latestActiveCodeId = null;
let onPacksChanged = null;

export function initProfile(opts = {}) {
  onProfileChange = opts.onChange || null;
  onPacksChanged = opts.onPacksChanged || null;
  document.getElementById("btn-add-passkey")?.addEventListener("click", addPasskeyHere);
  document.getElementById("btn-save-profile")?.addEventListener("click", saveProfile);
  document.getElementById("device-list")?.addEventListener("click", onDeviceListClick);
  document.getElementById("btn-mint-device-code")?.addEventListener("click", mintDeviceCode);
  document.getElementById("btn-copy-device-code")?.addEventListener("click", copyActiveDeviceCode);
  document.getElementById("btn-revoke-device-code")?.addEventListener("click", revokeActiveDeviceCode);
  document.getElementById("device-code-list")?.addEventListener("click", onDeviceCodeListClick);
  document.getElementById("pack-list")?.addEventListener("change", onPackToggle);
}

export async function refreshProfile() {
  const list = document.getElementById("device-list");
  const st = document.getElementById("profile-status");
  if (st) {
    st.textContent = "";
    st.classList.remove("error");
  }
  try {
    const user = await me();
    const uEl = document.getElementById("profile-username");
    const eEl = document.getElementById("profile-email");
    const rEl = document.getElementById("profile-role");
    const dn = document.getElementById("profile-display-name");
    const em = document.getElementById("profile-email-input");
    if (uEl) uEl.textContent = user.username || "—";
    if (eEl) eEl.textContent = user.email || "—";
    if (rEl) rEl.textContent = user.role || "—";
    if (dn) dn.value = user.display_name || "";
    if (em) em.value = user.email || "";

    const devices = user.devices || [];
    const currentId = user.current_credential_id || "";
    if (list) {
      if (!devices.length) {
        list.innerHTML = `<li class="empty-state">No passkeys yet.</li>`;
      } else {
        list.innerHTML = devices
          .map((d) => {
            const isCurrent = Boolean(d.is_current) || (currentId && d.id === currentId);
            const uses = Number(d.use_count ?? d.sign_count ?? 0);
            const lastUsed = d.last_used_at ? formatRelative(d.last_used_at) : "never";
            return `
      <li class="device-card ${isCurrent ? "is-current" : ""}" data-id="${escapeAttr(d.id)}">
        <div class="device-main">
          <div class="device-title-row">
            <input
              class="device-name-input"
              type="text"
              value="${escapeAttr(d.device_type || "Device")}"
              maxlength="64"
              aria-label="Device name"
            />
            ${isCurrent ? `<span class="device-badge" title="Passkey used for this browser session">This session</span>` : ""}
          </div>
          <div class="device-meta">
            Added ${d.created_at ? new Date(d.created_at).toLocaleString() : "—"}
            · used ${uses}×
            · last ${escapeAttr(lastUsed)}
          </div>
        </div>
        <div class="device-actions">
          <button type="button" class="secondary device-save" data-action="save">Save name</button>
          <button type="button" class="ghost device-remove" data-action="remove"${isCurrent ? " title=\"This is the passkey for your current session\"" : ""}>Remove</button>
        </div>
      </li>`;
          })
          .join("");
      }
    }

    await refreshDeviceCodes();
    await refreshPacks();
    return user;
  } catch (err) {
    if (list) list.innerHTML = `<li class="empty-state">Could not load devices</li>`;
    if (st) {
      st.textContent = err.message || "Failed to load profile";
      st.classList.add("error");
    }
    return null;
  }
}

async function refreshPacks() {
  const box = document.getElementById("pack-list");
  const st = document.getElementById("pack-status");
  if (!box) return;
  if (st) {
    st.textContent = "";
    st.classList.remove("error");
  }
  try {
    const data = await api("/api/packs");
    const packs = data.packs || [];
    if (!packs.length) {
      box.innerHTML = `<p class="muted">No packs available.</p>`;
      return;
    }
    box.innerHTML = packs
      .map((p) => {
        const locked = Boolean(p.always_on);
        const checked = locked || Boolean(p.enabled);
        return `
      <label class="pack-card ${locked ? "is-locked" : ""} ${checked ? "is-on" : ""}">
        <input type="checkbox" data-pack-key="${escapeAttr(p.key)}"
          ${checked ? "checked" : ""} ${locked ? "disabled" : ""} />
        <span class="pack-card-body">
          <span class="pack-card-title">
            <strong>${escapeAttr(p.label)}</strong>
            ${locked ? `<span class="device-badge">Always on</span>` : ""}
          </span>
          <span class="pack-card-desc">${escapeAttr(p.description || "")}</span>
          <span class="pack-card-meta">${Number(p.tag_count) || (p.tag_keys || []).length} tags</span>
        </span>
      </label>`;
      })
      .join("");
  } catch (err) {
    box.innerHTML = `<p class="muted">Could not load tag packs</p>`;
    if (st) {
      st.textContent = err.message || "Failed to load packs";
      st.classList.add("error");
    }
  }
}

async function onPackToggle(e) {
  const input = e.target.closest('input[type="checkbox"][data-pack-key]');
  if (!input || input.disabled) return;
  const box = document.getElementById("pack-list");
  const st = document.getElementById("pack-status");
  const selected = [];
  box?.querySelectorAll('input[type="checkbox"][data-pack-key]:not(:disabled)').forEach((el) => {
    if (el.checked) selected.push(el.dataset.packKey);
  });
  if (st) {
    st.textContent = "Saving…";
    st.classList.remove("error");
  }
  try {
    await api("/api/packs", { method: "PUT", body: { packs: selected } });
    invalidateTagCatalog();
    if (st) st.textContent = "Tag packs updated — check-in chips will match.";
    await refreshPacks();
    if (typeof onPacksChanged === "function") {
      try {
        await onPacksChanged();
      } catch {
        /* ignore */
      }
    }
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Could not save packs";
      st.classList.add("error");
    }
    await refreshPacks();
  }
}

async function refreshDeviceCodes() {
  const list = document.getElementById("device-code-list");
  const banner = document.getElementById("device-code-active");
  const valueEl = document.getElementById("device-code-value");
  const metaEl = document.getElementById("device-code-meta");
  try {
    const data = await api("/api/auth/device-codes");
    const codes = data.device_codes || [];
    const active = codes.find((c) => c.usable);
    latestActiveCodeId = active?.id || null;

    if (banner && valueEl && metaEl) {
      if (active) {
        banner.hidden = false;
        valueEl.textContent = active.code_display || formatCodeDisplay(active.code);
        const exp = active.expires_at ? new Date(active.expires_at) : null;
        metaEl.textContent = exp
          ? `Valid until ${exp.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })} · single use`
          : "Single use · expires soon";
      } else {
        banner.hidden = true;
        valueEl.textContent = "————";
        metaEl.textContent = "";
      }
    }

    if (!list) return;
    const others = codes.filter((c) => !c.usable).slice(0, 8);
    if (!others.length) {
      list.innerHTML = "";
      return;
    }
    list.innerHTML = others
      .map((c) => {
        const label = c.label ? escapeAttr(c.label) + " · " : "";
        return `<li class="device-code-item" data-id="${escapeAttr(c.id)}">
          <span class="mono">${escapeAttr(c.code_display || formatCodeDisplay(c.code))}</span>
          <span class="muted">${label}${escapeAttr(c.status)}</span>
        </li>`;
      })
      .join("");
  } catch {
    if (list) list.innerHTML = "";
  }
}

async function mintDeviceCode() {
  const st = document.getElementById("profile-status");
  const btn = document.getElementById("btn-mint-device-code");
  st?.classList.remove("error");
  if (btn) btn.disabled = true;
  try {
    const label = document.getElementById("device-code-label")?.value.trim() || "";
    const data = await api("/api/auth/device-codes", {
      method: "POST",
      body: { label },
    });
    const dc = data.device_code;
    if (st) {
      st.textContent = `Code ${dc?.code_display || ""} ready — enter it on the other device under Add device`;
    }
    if (document.getElementById("device-code-label")) {
      document.getElementById("device-code-label").value = "";
    }
    await refreshDeviceCodes();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Could not create device code";
      st.classList.add("error");
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function copyActiveDeviceCode() {
  const valueEl = document.getElementById("device-code-value");
  const st = document.getElementById("profile-status");
  const text = (valueEl?.textContent || "").trim();
  if (!text || text.includes("—")) return;
  try {
    await navigator.clipboard.writeText(text.replace(/\s/g, ""));
    if (st) {
      st.textContent = `Copied ${text}`;
      st.classList.remove("error");
    }
  } catch {
    if (st) {
      st.textContent = "Could not copy — select the code manually";
      st.classList.add("error");
    }
  }
}

async function revokeActiveDeviceCode() {
  if (!latestActiveCodeId) return;
  const ok = await appConfirm({
    title: "Revoke device code?",
    message: "This code will stop working immediately. You can generate a new one anytime.",
    confirmLabel: "Revoke",
    variant: "danger",
  });
  if (!ok) return;
  const st = document.getElementById("profile-status");
  try {
    await api(`/api/auth/device-codes/${encodeURIComponent(latestActiveCodeId)}`, {
      method: "DELETE",
    });
    if (st) {
      st.textContent = "Device code revoked";
      st.classList.remove("error");
    }
    await refreshDeviceCodes();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Revoke failed";
      st.classList.add("error");
    }
  }
}

async function onDeviceCodeListClick() {
  /* list is informational for used/expired codes */
}

async function saveProfile() {
  const st = document.getElementById("profile-status");
  const btn = document.getElementById("btn-save-profile");
  st?.classList.remove("error");
  const display_name = document.getElementById("profile-display-name")?.value.trim() || "";
  const email = document.getElementById("profile-email-input")?.value.trim() || "";
  if (btn) btn.disabled = true;
  try {
    await updateProfile({ display_name, email });
    if (st) st.textContent = "Profile saved";
    const user = await refreshProfile();
    if (user && onProfileChange) onProfileChange(user);
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Save failed";
      st.classList.add("error");
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function onDeviceListClick(e) {
  const btn = e.target.closest("button[data-action]");
  if (!btn) return;
  const card = btn.closest(".device-card");
  if (!card) return;
  const id = card.dataset.id;
  const st = document.getElementById("profile-status");
  st?.classList.remove("error");

  if (btn.dataset.action === "save") {
    const input = card.querySelector(".device-name-input");
    const name = (input?.value || "").trim();
    if (!name) {
      if (st) {
        st.textContent = "Enter a device name";
        st.classList.add("error");
      }
      return;
    }
    btn.disabled = true;
    try {
      await api(`/api/auth/devices/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: { device_type: name },
      });
      if (st) st.textContent = "Device name saved";
      const user = await refreshProfile();
      if (user && onProfileChange) onProfileChange(user);
    } catch (err) {
      if (st) {
        st.textContent = err.message || "Rename failed";
        st.classList.add("error");
      }
    } finally {
      btn.disabled = false;
    }
    return;
  }

  if (btn.dataset.action === "remove") {
    const ok = await appConfirm({
      title: "Remove passkey?",
      message:
        "You will not be able to sign in from that device until you add a new passkey. Your other devices keep working.",
      confirmLabel: "Remove passkey",
      variant: "danger",
    });
    if (!ok) return;
    btn.disabled = true;
    try {
      await api(`/api/auth/devices/${encodeURIComponent(id)}`, { method: "DELETE" });
      if (st) st.textContent = "Device removed";
      const user = await refreshProfile();
      if (user && onProfileChange) onProfileChange(user);
    } catch (err) {
      if (st) {
        st.textContent = err.message || "Remove failed";
        st.classList.add("error");
      }
    } finally {
      btn.disabled = false;
    }
  }
}

async function addPasskeyHere() {
  const st = document.getElementById("profile-status");
  const btn = document.getElementById("btn-add-passkey");
  st?.classList.remove("error");
  if (st) st.textContent = "Creating passkey…";
  if (btn) btn.disabled = true;
  try {
    // Logged-in add: empty username uses session on server
    await registerPasskey({
      username: "",
      device_type: detectDeviceLabel(),
    });
    if (st) st.textContent = "Passkey added on this device";
    const next = await refreshProfile();
    if (next && onProfileChange) onProfileChange(next);
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Could not add passkey";
      st.classList.add("error");
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

function formatRelative(iso) {
  try {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return "—";
    const sec = Math.round((Date.now() - d.getTime()) / 1000);
    if (sec < 45) return "just now";
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    if (sec < 86400 * 14) return `${Math.floor(sec / 86400)}d ago`;
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
  } catch {
    return "—";
  }
}

function escapeAttr(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}
