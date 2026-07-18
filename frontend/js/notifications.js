import { api } from "./api.js";

let panelOpen = false;
let pollTimer = null;

export function initNotifications() {
  document.getElementById("btn-notifications")?.addEventListener("click", (e) => {
    e.stopPropagation();
    togglePanel();
  });
  document.getElementById("btn-notif-mark-all")?.addEventListener("click", async (e) => {
    e.stopPropagation();
    try {
      await api("/api/notifications/read-all", { method: "POST", body: {} });
      await refreshNotifications();
    } catch {
      /* ignore */
    }
  });
  document.getElementById("notif-list")?.addEventListener("click", onNotifClick);
  document.addEventListener("click", (e) => {
    const panel = document.getElementById("notif-panel");
    const btn = document.getElementById("btn-notifications");
    if (!panel || panel.hidden) return;
    if (panel.contains(e.target) || btn?.contains(e.target)) return;
    closePanel();
  });
}

export function startNotificationPolling() {
  stopNotificationPolling();
  refreshNotifications();
  pollTimer = setInterval(() => {
    if (document.visibilityState === "visible") {
      refreshUnreadBadge();
    }
  }, 45000);
}

export function stopNotificationPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  closePanel();
  setBadge(0);
}

async function togglePanel() {
  const panel = document.getElementById("notif-panel");
  if (!panel) return;
  if (panelOpen) {
    closePanel();
    return;
  }
  panel.hidden = false;
  panelOpen = true;
  await refreshNotifications();
}

function closePanel() {
  const panel = document.getElementById("notif-panel");
  if (panel) panel.hidden = true;
  panelOpen = false;
}

export async function refreshUnreadBadge() {
  try {
    const data = await api("/api/notifications/unread-count");
    setBadge(Number(data.unread_count) || 0);
  } catch {
    /* not signed in */
  }
}

export async function refreshNotifications() {
  const list = document.getElementById("notif-list");
  try {
    const data = await api("/api/notifications?limit=30");
    setBadge(Number(data.unread_count) || 0);
    const rows = data.notifications || [];
    if (!list) return;
    if (!rows.length) {
      list.innerHTML = `<li class="notif-empty">No notifications yet</li>`;
      return;
    }
    list.innerHTML = rows.map(renderNotif).join("");
  } catch (err) {
    if (list) list.innerHTML = `<li class="notif-empty">Could not load</li>`;
  }
}

function renderNotif(n) {
  const unread = !n.read_at;
  const when = n.created_at ? formatWhen(n.created_at) : "";
  return `<li class="notif-item ${unread ? "is-unread" : ""}" data-id="${esc(n.id)}" data-kind="${esc(n.kind)}" data-patient="${esc(n.patient_id || "")}">
    <div class="notif-item-main">
      <strong class="notif-title">${esc(n.title)}</strong>
      <span class="notif-body">${esc(n.body || "")}</span>
      <time class="notif-when">${esc(when)}</time>
    </div>
    ${unread ? '<span class="notif-dot" aria-label="Unread"></span>' : ""}
  </li>`;
}

async function onNotifClick(e) {
  const item = e.target.closest(".notif-item");
  if (!item) return;
  const id = item.dataset.id;
  if (id && item.classList.contains("is-unread")) {
    try {
      await api(`/api/notifications/${encodeURIComponent(id)}/read`, { method: "POST", body: {} });
      item.classList.remove("is-unread");
      const dot = item.querySelector(".notif-dot");
      if (dot) dot.remove();
      await refreshUnreadBadge();
    } catch {
      /* ignore */
    }
  }
  // Navigate based on kind
  const kind = item.dataset.kind;
  closePanel();
  if (kind === "patient_log" || kind === "partner_granted") {
    document.querySelector('#tabbar button[data-mode="partner"]')?.click();
  } else if (kind === "observation") {
    document.querySelector('#tabbar button[data-mode="patient"]')?.click();
  }
}

function setBadge(n) {
  const badge = document.getElementById("notif-badge");
  const btn = document.getElementById("btn-notifications");
  if (!badge) return;
  if (n > 0) {
    badge.hidden = false;
    badge.textContent = n > 99 ? "99+" : String(n);
    if (btn) btn.setAttribute("aria-label", `Notifications, ${n} unread`);
  } else {
    badge.hidden = true;
    badge.textContent = "0";
    if (btn) btn.setAttribute("aria-label", "Notifications");
  }
}

function formatWhen(iso) {
  try {
    const d = new Date(iso);
    const now = new Date();
    const diff = (now - d) / 1000;
    if (diff < 60) return "Just now";
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
  } catch {
    return "";
  }
}

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/"/g, "&quot;");
}
