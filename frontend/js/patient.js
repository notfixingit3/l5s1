import { api } from "./api.js";
import { ensureTagCatalog, formatWhen, painBadge, renderTagBadges } from "./tags.js";

const selectedTags = new Set();

function painLevelAttr(n) {
  if (n <= 3) return "low";
  if (n <= 6) return "mid";
  return "high";
}

function syncPainUI(slider, painValue) {
  if (!slider || !painValue) return;
  const n = Number(slider.value);
  painValue.textContent = String(n);
  painValue.dataset.level = painLevelAttr(n);
  slider.setAttribute("aria-valuenow", String(n));
}

export function initPatient() {
  const slider = document.getElementById("pain-slider");
  const painValue = document.getElementById("pain-value");
  syncPainUI(slider, painValue);
  slider?.addEventListener("input", () => {
    syncPainUI(slider, painValue);
  });

  document.getElementById("tag-picker")?.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-tag]");
    if (!btn) return;
    const tag = btn.dataset.tag;
    if (selectedTags.has(tag)) {
      selectedTags.delete(tag);
      btn.classList.remove("selected");
    } else {
      selectedTags.add(tag);
      btn.classList.add("selected");
    }
  });

  document.getElementById("btn-save-log")?.addEventListener("click", saveLog);
  loadTags().then(() => refreshLogs());
}

export async function loadTags() {
  const picker = document.getElementById("tag-picker");
  if (!picker) return;
  selectedTags.clear();
  try {
    const catalog = await ensureTagCatalog();
    const tags = [...catalog.values()];
    // Prefer API order: re-fetch full list for sort_order
    const data = await api("/api/tags");
    const ordered = data.tags || tags;
    if (!ordered.length) {
      picker.innerHTML = `<span class="muted">No tags configured. Ask an admin to add some.</span>`;
      return;
    }
    picker.innerHTML = ordered
      .map((t) => {
        const key = t.key;
        const label = t.label || key;
        return `<button type="button" data-tag="${escapeAttr(key)}">${escapeHtml(label)}</button>`;
      })
      .join("");
  } catch {
    picker.innerHTML = `<span class="muted">Could not load tags</span>`;
  }
}

async function saveLog() {
  const status = document.getElementById("log-status");
  status.textContent = "Saving…";
  status.classList.remove("error");
  try {
    await api("/api/logs", {
      method: "POST",
      body: {
        pain_level: Number(document.getElementById("pain-slider").value),
        short_notes: document.getElementById("log-notes").value,
        tags: [...selectedTags].join(","),
      },
    });
    status.textContent = "Saved";
    document.getElementById("log-notes").value = "";
    selectedTags.clear();
    document.querySelectorAll("#tag-picker button.selected").forEach((b) => b.classList.remove("selected"));
    await refreshLogs();
  } catch (err) {
    status.textContent = err.message;
    status.classList.add("error");
  }
}

export async function refreshLogs() {
  const list = document.getElementById("log-list");
  if (!list) return;
  await ensureTagCatalog();
  try {
    const data = await api("/api/logs?limit=30");
    const logs = data.logs || [];
    if (!logs.length) {
      list.innerHTML = `<li class="empty-state" style="border:none;background:transparent">No entries yet — save your first check-in above.</li>`;
      return;
    }
    list.innerHTML = logs.map((l) => renderEntryCard(l)).join("");
  } catch {
    list.innerHTML = `<li class="empty-state" style="border:none">Could not load entries</li>`;
  }
}

export function renderEntryCard(l) {
  const pain = Number(l.pain_level) || 0;
  const band = painLevelAttr(pain);
  const notes = (l.short_notes || "").trim();
  return `
    <li class="entry-card ${l.is_observation ? "obs" : ""}" data-pain-band="${band}">
      <div class="entry-top">
        <time class="entry-when">${escapeHtml(formatWhen(l.created_at))}</time>
        ${painBadge(pain)}
        ${l.is_observation ? '<span class="badge obs-badge">Observation</span>' : ""}
      </div>
      ${notes ? `<div class="entry-notes">${escapeHtml(notes)}</div>` : `<div class="entry-notes muted">No notes</div>`}
      ${l.tags ? renderTagBadges(l.tags) : ""}
    </li>`;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

function escapeAttr(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;");
}
