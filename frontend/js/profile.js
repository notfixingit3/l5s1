import { api } from "./api.js";
import { detectDeviceLabel, me, registerPasskey, updateProfile } from "./auth.js";
import { appConfirm } from "./dialog.js";

let onProfileChange = null;

export function initProfile(opts = {}) {
  onProfileChange = opts.onChange || null;
  document.getElementById("btn-add-passkey")?.addEventListener("click", addPasskeyHere);
  document.getElementById("btn-save-profile")?.addEventListener("click", saveProfile);
  document.getElementById("device-list")?.addEventListener("click", onDeviceListClick);
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
    if (!list) return user;
    if (!devices.length) {
      list.innerHTML = `<li class="empty-state">No passkeys yet.</li>`;
      return user;
    }
    list.innerHTML = devices
      .map(
        (d) => `
      <li class="device-card" data-id="${escapeAttr(d.id)}">
        <div class="device-main">
          <input
            class="device-name-input"
            type="text"
            value="${escapeAttr(d.device_type || "Device")}"
            maxlength="64"
            aria-label="Device name"
          />
          <div class="device-meta">
            Added ${d.created_at ? new Date(d.created_at).toLocaleString() : "—"}
            · used ${d.sign_count ?? 0}×
          </div>
        </div>
        <div class="device-actions">
          <button type="button" class="secondary device-save" data-action="save">Save name</button>
          <button type="button" class="ghost device-remove" data-action="remove">Remove</button>
        </div>
      </li>`
      )
      .join("");
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

function escapeAttr(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}
