import { api } from "./api.js";
import { appConfirm, appPrompt } from "./dialog.js";

export function initAdmin() {
  document.querySelectorAll(".admin-tab").forEach((t) => {
    t.addEventListener("click", () => setAdminTab(t.dataset.adminTab));
  });
  document.getElementById("btn-create-invite")?.addEventListener("click", createInvite);
  document.getElementById("btn-create-tag")?.addEventListener("click", createTag);
  document.getElementById("btn-tags-recommended")?.addEventListener("click", applyRecommendedTagOrder);
  document.getElementById("admin-users-list")?.addEventListener("click", onUsersClick);
  document.getElementById("admin-invites-list")?.addEventListener("click", onInvitesClick);
  document.getElementById("admin-tags-list")?.addEventListener("click", onTagsClick);
}

export async function refreshAdmin() {
  await Promise.all([loadUsers(), loadInvites(), loadTagsAdmin()]);
}

function setAdminTab(which) {
  document.querySelectorAll(".admin-tab").forEach((t) => {
    t.classList.toggle("active", t.dataset.adminTab === which);
  });
  document.getElementById("admin-panel-users").hidden = which !== "users";
  document.getElementById("admin-panel-invites").hidden = which !== "invites";
  document.getElementById("admin-panel-tags").hidden = which !== "tags";
}

async function loadUsers() {
  const box = document.getElementById("admin-users-list");
  const st = document.getElementById("admin-users-status");
  if (!box) return;
  try {
    const data = await api("/api/admin/users");
    const users = data.users || [];
    if (!users.length) {
      box.innerHTML = `<p class="empty-state">No users</p>`;
      return;
    }
    box.innerHTML = users
      .map((u) => {
        const lockLabel = u.is_active ? "Lock" : "Unlock";
        const status = u.is_active
          ? `<span class="badge-ok">Active</span>`
          : `<span class="badge-locked">Locked</span>`;
        const roleBadge =
          u.role === "admin"
            ? `<span class="badge-ok">admin</span>`
            : u.role === "partner"
              ? `<span class="badge role-partner">partner</span>`
              : `<span class="badge role-patient">patient</span>`;
        const makeAdminBtn =
          u.role === "admin"
            ? `<button type="button" class="ghost" data-action="role-patient">Remove admin</button>`
            : `<button type="button" class="secondary" data-action="role-admin">Make admin</button>`;
        return `
        <div class="admin-card" data-user-id="${esc(u.id)}" data-role="${esc(u.role)}">
          <h4>${esc(u.display_name || u.username)} ${status} ${roleBadge}</h4>
          <div class="meta">@${esc(u.username)} · ${esc(u.email || "no email")} · ${u.device_count} device(s)</div>
          <div class="actions">
            <button type="button" class="secondary" data-action="toggle-lock">${lockLabel}</button>
            ${makeAdminBtn}
            <button type="button" class="ghost" data-action="role-patient">Set patient</button>
            <button type="button" class="ghost" data-action="role-partner">Set partner</button>
            <button type="button" class="ghost" data-action="force-rereg">Force re-register</button>
          </div>
        </div>`;
      })
      .join("");
    if (st) {
      st.textContent = "";
      st.classList.remove("error");
    }
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Failed to load users";
      st.classList.add("error");
    }
  }
}

async function onUsersClick(e) {
  const btn = e.target.closest("button[data-action]");
  const card = e.target.closest(".admin-card");
  if (!btn || !card) return;
  const id = card.dataset.userId;
  const who = card.querySelector("h4")?.textContent?.replace(/\s+/g, " ").trim() || "this user";
  const st = document.getElementById("admin-users-status");
  st?.classList.remove("error");
  const action = btn.dataset.action;
  let body = null;

  if (action === "toggle-lock") {
    const locked = card.querySelector(".badge-locked");
    const unlocking = !!locked;
    const ok = await appConfirm({
      title: unlocking ? "Unlock account?" : "Lock account?",
      message: unlocking
        ? `${who} will be able to sign in again.`
        : `${who} will not be able to sign in until unlocked.`,
      confirmLabel: unlocking ? "Unlock" : "Lock account",
      variant: unlocking ? "primary" : "danger",
    });
    if (!ok) return;
    body = { is_active: unlocking };
  } else if (action === "force-rereg") {
    const ok = await appConfirm({
      title: "Force re-registration?",
      message: `This wipes all passkeys for ${who}. They must create a new passkey before signing in again.`,
      confirmLabel: "Wipe passkeys",
      variant: "danger",
    });
    if (!ok) return;
    body = { force_re_register: true };
  } else if (action === "role-patient") {
    const wasAdmin = card.dataset.role === "admin";
    const ok = await appConfirm({
      title: wasAdmin ? "Remove admin access?" : "Set as patient?",
      message: wasAdmin
        ? `${who} will lose admin access to users, invites, and tags.`
        : `${who} will be set to the patient role.`,
      confirmLabel: wasAdmin ? "Remove admin" : "Set patient",
      variant: wasAdmin ? "danger" : "primary",
    });
    if (!ok) return;
    body = { role: "patient" };
  } else if (action === "role-partner") {
    const ok = await appConfirm({
      title: "Set as partner?",
      message: `${who} will be set to the partner role (care observation).`,
      confirmLabel: "Set partner",
    });
    if (!ok) return;
    body = { role: "partner" };
  } else if (action === "role-admin") {
    const ok = await appConfirm({
      title: "Make admin?",
      message: `${who} will get full admin access: manage users, invites, tags, and lock accounts.`,
      confirmLabel: "Make admin",
      variant: "danger",
    });
    if (!ok) return;
    body = { role: "admin" };
  }
  if (!body) return;
  btn.disabled = true;
  try {
    await api(`/api/admin/users/${encodeURIComponent(id)}`, { method: "PATCH", body });
    if (st) {
      if (action === "role-admin") st.textContent = "User is now an admin";
      else if (action === "role-patient" && card.dataset.role === "admin") st.textContent = "Admin role removed";
      else if (action === "toggle-lock") st.textContent = body.is_active ? "Account unlocked" : "Account locked";
      else st.textContent = "User updated";
    }
    await loadUsers();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Update failed";
      st.classList.add("error");
    }
  } finally {
    btn.disabled = false;
  }
}

async function loadInvites() {
  const box = document.getElementById("admin-invites-list");
  const st = document.getElementById("admin-invites-status");
  if (!box) return;
  try {
    const data = await api("/api/admin/invites");
    const invites = data.invites || [];
    if (!invites.length) {
      box.innerHTML = `<p class="empty-state">No invites yet — generate one above.</p>`;
      return;
    }
    box.innerHTML = invites
      .map((inv) => {
        const display = inv.code_display || formatInviteCode(inv.code);
        const active = inv.is_active && inv.remaining > 0;
        const badge = active
          ? `<span class="badge-ok">${inv.remaining} left</span>`
          : `<span class="badge-locked">Exhausted/off</span>`;
        return `
        <div class="admin-card" data-invite-id="${esc(inv.id)}">
          <div class="admin-code">${esc(display)}</div>
          <div class="meta">${esc(inv.label || "Invite")} · max ${inv.max_uses} · used ${inv.used_count} ${badge}</div>
          <div class="actions">
            <button type="button" class="secondary" data-action="copy">Copy code</button>
            <button type="button" class="ghost" data-action="toggle">${inv.is_active ? "Disable" : "Enable"}</button>
          </div>
        </div>`;
      })
      .join("");
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Failed to load invites";
      st.classList.add("error");
    }
  }
}

function formatInviteCode(raw) {
  const d = String(raw || "").replace(/\D/g, "");
  if (d.length !== 8) return String(raw || "");
  return `${d.slice(0, 4)}-${d.slice(4)}`;
}

async function createInvite() {
  const st = document.getElementById("admin-invites-status");
  const btn = document.getElementById("btn-create-invite");
  st?.classList.remove("error");
  if (btn) btn.disabled = true;
  try {
    const data = await api("/api/admin/invites", {
      method: "POST",
      body: {
        label: document.getElementById("invite-label")?.value || "",
        max_uses: Number(document.getElementById("invite-max-uses")?.value || 1),
        expires_in_days: Number(document.getElementById("invite-expires-days")?.value || 0),
      },
    });
    const shown = data.code_display || formatInviteCode(data.invite?.code || "");
    if (st) st.textContent = `Created code ${shown}`;
    document.getElementById("invite-label").value = "";
    await loadInvites();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Create failed";
      st.classList.add("error");
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function onInvitesClick(e) {
  const btn = e.target.closest("button[data-action]");
  const card = e.target.closest(".admin-card");
  if (!btn || !card) return;
  const id = card.dataset.inviteId;
  const code = card.querySelector(".admin-code")?.textContent || "";
  if (btn.dataset.action === "copy") {
    try {
      await navigator.clipboard.writeText(code.trim());
      const st = document.getElementById("admin-invites-status");
      if (st) st.textContent = `Copied ${code.trim()}`;
    } catch {
      /* ignore */
    }
    return;
  }
  if (btn.dataset.action === "toggle") {
    const enable = btn.textContent.includes("Enable");
    const ok = await appConfirm({
      title: enable ? "Enable invite?" : "Disable invite?",
      message: enable
        ? `Code ${code.trim()} can be used again (if uses remain).`
        : `Code ${code.trim()} will stop working for new signups.`,
      confirmLabel: enable ? "Enable" : "Disable",
      variant: enable ? "primary" : "danger",
    });
    if (!ok) return;
    try {
      await api(`/api/admin/invites/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: { is_active: enable },
      });
      await loadInvites();
    } catch (err) {
      const st = document.getElementById("admin-invites-status");
      if (st) {
        st.textContent = err.message || "Update failed";
        st.classList.add("error");
      }
    }
  }
}

async function loadTagsAdmin() {
  const box = document.getElementById("admin-tags-list");
  const st = document.getElementById("admin-tags-status");
  if (!box) return;
  try {
    const data = await api("/api/admin/tags");
    const tags = data.tags || [];
    if (!tags.length) {
      box.innerHTML = `<p class="empty-state">No tags</p>`;
      return;
    }
    box.innerHTML = tags
      .map((t, i) => {
        const badge = t.is_active ? `<span class="badge-ok">On</span>` : `<span class="badge-locked">Off</span>`;
        return `
        <div class="admin-card" data-tag-id="${esc(t.id)}" data-tag-key="${esc(t.key)}" data-tag-label="${esc(t.label)}">
          <div class="tag-row-head">
            <span class="tag-pos">${i + 1}</span>
            <h4>${esc(t.label)} ${badge}</h4>
          </div>
          <div class="meta">key: ${esc(t.key)}</div>
          <div class="actions">
            <button type="button" class="secondary" data-action="up" title="Move up">↑</button>
            <button type="button" class="secondary" data-action="down" title="Move down">↓</button>
            <button type="button" class="ghost" data-action="rename">Rename</button>
            <button type="button" class="ghost" data-action="toggle">${t.is_active ? "Disable" : "Enable"}</button>
            <button type="button" class="ghost" data-action="delete">Delete</button>
          </div>
        </div>`;
      })
      .join("");
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Failed to load tags";
      st.classList.add("error");
    }
  }
}

async function applyRecommendedTagOrder() {
  const st = document.getElementById("admin-tags-status");
  const ok = await appConfirm({
    title: "Reset tag order?",
    message:
      "Apply the recommended fast-entry order for default tags (side → region → sensation → conditions). Your custom order for those keys will be overwritten.",
    confirmLabel: "Reset order",
  });
  if (!ok) return;
  st?.classList.remove("error");
  try {
    await api("/api/admin/tags/apply-recommended", { method: "POST", body: {} });
    if (st) st.textContent = "Recommended order applied";
    await loadTagsAdmin();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Failed";
      st.classList.add("error");
    }
  }
}

async function createTag() {
  const st = document.getElementById("admin-tags-status");
  const btn = document.getElementById("btn-create-tag");
  st?.classList.remove("error");
  const label = document.getElementById("tag-label")?.value.trim() || "";
  const key = document.getElementById("tag-key")?.value.trim() || "";
  if (!label) {
    if (st) {
      st.textContent = "Label required";
      st.classList.add("error");
    }
    return;
  }
  if (btn) btn.disabled = true;
  try {
    await api("/api/admin/tags", { method: "POST", body: { label, key } });
    if (st) st.textContent = "Tag created";
    document.getElementById("tag-label").value = "";
    document.getElementById("tag-key").value = "";
    await loadTagsAdmin();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Create failed";
      st.classList.add("error");
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function onTagsClick(e) {
  const btn = e.target.closest("button[data-action]");
  const card = e.target.closest(".admin-card");
  if (!btn || !card) return;
  const id = card.dataset.tagId;
  const st = document.getElementById("admin-tags-status");
  st?.classList.remove("error");
  try {
    if (btn.dataset.action === "up" || btn.dataset.action === "down") {
      await api(`/api/admin/tags/${encodeURIComponent(id)}/move`, {
        method: "POST",
        body: { direction: btn.dataset.action },
      });
    } else if (btn.dataset.action === "rename") {
      const next = await appPrompt({
        title: "Rename tag",
        label: "Display name",
        defaultValue: card.dataset.tagLabel || "",
        confirmLabel: "Save name",
      });
      if (next === null || !next.trim()) return;
      await api(`/api/admin/tags/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: { label: next.trim() },
      });
    } else if (btn.dataset.action === "toggle") {
      const enable = btn.textContent.includes("Enable");
      const label = card.dataset.tagLabel || "this tag";
      const ok = await appConfirm({
        title: enable ? "Enable tag?" : "Disable tag?",
        message: enable
          ? `"${label}" will appear on the check-in screen again.`
          : `"${label}" will be hidden from check-in (existing logs keep the text).`,
        confirmLabel: enable ? "Enable" : "Disable",
        variant: enable ? "primary" : "danger",
      });
      if (!ok) return;
      await api(`/api/admin/tags/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: { is_active: enable },
      });
    } else if (btn.dataset.action === "delete") {
      const label = card.dataset.tagLabel || "this tag";
      const ok = await appConfirm({
        title: "Delete tag?",
        message: `Remove "${label}" from the catalog? Existing log entries keep their stored tag text.`,
        confirmLabel: "Delete tag",
        variant: "danger",
      });
      if (!ok) return;
      await api(`/api/admin/tags/${encodeURIComponent(id)}`, { method: "DELETE" });
    }
    await loadTagsAdmin();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Action failed";
      st.classList.add("error");
    }
  }
}

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/"/g, "&quot;");
}
