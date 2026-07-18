import { api } from "./api.js";
import { appConfirm } from "./dialog.js";
import { ensureTagCatalog, formatWhen, painBadge, renderTagBadges } from "./tags.js";

const selectedTags = new Set();
const editSelectedTags = new Set();
let editingLogId = null;
let openSwipeEl = null;

const SWIPE_THRESHOLD = 72;
const SWIPE_MAX = 96;

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
  wireLogListGestures();
  wireEditSheet();
  loadTags().then(() => refreshLogs());
}

export async function loadTags() {
  const picker = document.getElementById("tag-picker");
  if (!picker) return;
  selectedTags.clear();
  try {
    const catalog = await ensureTagCatalog();
    const tags = [...catalog.values()];
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
  closeOpenSwipe();
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
  const id = l.id;
  const swipable = !l.is_observation;

  const cardInner = `
      <div class="entry-top">
        <time class="entry-when">${escapeHtml(formatWhen(l.created_at))}</time>
        ${painBadge(pain)}
        ${l.is_observation ? '<span class="badge obs-badge">Observation</span>' : ""}
      </div>
      ${notes ? `<div class="entry-notes">${escapeHtml(notes)}</div>` : `<div class="entry-notes muted">No notes</div>`}
      ${l.tags ? renderTagBadges(l.tags) : ""}`;

  if (!swipable) {
    return `
    <li class="entry-card obs" data-pain-band="${band}" data-id="${id}">
      ${cardInner}
    </li>`;
  }

  // Encode log payload for edit without re-fetch
  const payload = escapeAttr(
    JSON.stringify({
      id,
      pain_level: pain,
      short_notes: l.short_notes || "",
      tags: l.tags || "",
      created_at: l.created_at || "",
    })
  );

  return `
    <li class="entry-swipe" data-id="${id}" data-log="${payload}">
      <div class="entry-swipe-rail entry-swipe-rail--edit" aria-hidden="true">
        <span class="swipe-action-label">Edit</span>
      </div>
      <div class="entry-swipe-rail entry-swipe-rail--delete" aria-hidden="true">
        <span class="swipe-action-label">Delete</span>
      </div>
      <div class="entry-card entry-swipe-front" data-pain-band="${band}" tabindex="0">
        ${cardInner}
      </div>
    </li>`;
}

/* ——— Swipe gestures ——— */

function wireLogListGestures() {
  const list = document.getElementById("log-list");
  if (!list || list.dataset.swipeWired) return;
  list.dataset.swipeWired = "1";

  list.addEventListener("click", onListClick);

  let startX = 0;
  let startY = 0;
  let active = null;
  let startTx = 0;
  let tracking = false;
  let axis = null; // 'x' | 'y'

  const onStart = (x, y, target) => {
    const row = target.closest(".entry-swipe");
    if (!row) return;
    if (openSwipeEl && openSwipeEl !== row) closeOpenSwipe();
    active = row;
    startX = x;
    startY = y;
    startTx = getTranslateX(row);
    tracking = true;
    axis = null;
    row.classList.add("is-dragging");
  };

  const onMove = (x, y, e) => {
    if (!tracking || !active) return;
    const dx = x - startX;
    const dy = y - startY;
    if (!axis) {
      if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return;
      axis = Math.abs(dx) > Math.abs(dy) ? "x" : "y";
      if (axis === "y") {
        tracking = false;
        active.classList.remove("is-dragging");
        active = null;
        return;
      }
    }
    if (axis !== "x") return;
    e.preventDefault?.();
    let next = startTx + dx;
    next = Math.max(-SWIPE_MAX, Math.min(SWIPE_MAX, next));
    setTranslateX(active, next, false);
  };

  const onEnd = () => {
    if (!active) {
      tracking = false;
      return;
    }
    const row = active;
    row.classList.remove("is-dragging");
    const tx = getTranslateX(row);
    tracking = false;
    active = null;
    axis = null;

    if (tx <= -SWIPE_THRESHOLD) {
      // Swipe left → reveal delete, then confirm
      setTranslateX(row, -SWIPE_MAX, true);
      openSwipeEl = row;
      void confirmDelete(row);
    } else if (tx >= SWIPE_THRESHOLD) {
      // Swipe right → edit
      setTranslateX(row, SWIPE_MAX, true);
      openSwipeEl = row;
      openEditFromRow(row);
      // snap closed after opening sheet
      setTimeout(() => closeOpenSwipe(), 180);
    } else {
      setTranslateX(row, 0, true);
      if (openSwipeEl === row) openSwipeEl = null;
    }
  };

  list.addEventListener(
    "touchstart",
    (e) => {
      const t = e.touches[0];
      if (!t) return;
      onStart(t.clientX, t.clientY, e.target);
    },
    { passive: true }
  );
  list.addEventListener(
    "touchmove",
    (e) => {
      const t = e.touches[0];
      if (!t) return;
      onMove(t.clientX, t.clientY, e);
    },
    { passive: false }
  );
  list.addEventListener("touchend", onEnd);
  list.addEventListener("touchcancel", onEnd);

  // Pointer (desktop trackpad / mouse drag)
  list.addEventListener("pointerdown", (e) => {
    if (e.pointerType === "mouse" && e.button !== 0) return;
    const row = e.target.closest(".entry-swipe");
    if (!row) return;
    onStart(e.clientX, e.clientY, e.target);
    row.setPointerCapture?.(e.pointerId);
  });
  list.addEventListener("pointermove", (e) => {
    if (!tracking) return;
    onMove(e.clientX, e.clientY, e);
  });
  list.addEventListener("pointerup", onEnd);
  list.addEventListener("pointercancel", onEnd);
}

function onListClick(e) {
  const row = e.target.closest(".entry-swipe");
  if (!row) return;
  // Click rails if already open
  if (e.target.closest(".entry-swipe-rail--delete") || Math.abs(getTranslateX(row)) > 40) {
    // ignore accidental clicks while open; delete goes through swipe end
  }
}

function getTranslateX(row) {
  const front = row.querySelector(".entry-swipe-front");
  if (!front) return 0;
  const m = /translate3d\(([-\d.]+)px/.exec(front.style.transform || "");
  return m ? Number(m[1]) : 0;
}

function setTranslateX(row, x, animate) {
  const front = row.querySelector(".entry-swipe-front");
  if (!front) return;
  front.style.transition = animate ? "transform 0.2s ease" : "none";
  front.style.transform = `translate3d(${x}px,0,0)`;
  row.classList.toggle("is-open-delete", x < -20);
  row.classList.toggle("is-open-edit", x > 20);
}

function closeOpenSwipe() {
  if (!openSwipeEl) return;
  setTranslateX(openSwipeEl, 0, true);
  openSwipeEl = null;
}

async function confirmDelete(row) {
  const id = row.dataset.id;
  const ok = await appConfirm({
    title: "Delete entry?",
    message: "This check-in will be removed from your history. This cannot be undone.",
    confirmLabel: "Delete",
    variant: "danger",
  });
  if (!ok) {
    closeOpenSwipe();
    return;
  }
  try {
    await api(`/api/logs/${encodeURIComponent(id)}`, { method: "DELETE" });
    row.classList.add("entry-swipe-removing");
    setTimeout(() => {
      row.remove();
      const list = document.getElementById("log-list");
      if (list && !list.querySelector(".entry-swipe, .entry-card")) {
        list.innerHTML = `<li class="empty-state" style="border:none;background:transparent">No entries yet — save your first check-in above.</li>`;
      }
    }, 200);
    openSwipeEl = null;
  } catch (err) {
    closeOpenSwipe();
    const st = document.getElementById("log-status");
    if (st) {
      st.textContent = err.message || "Delete failed";
      st.classList.add("error");
    }
  }
}

/* ——— Edit sheet ——— */

function wireEditSheet() {
  const sheet = document.getElementById("entry-edit-sheet");
  if (!sheet || sheet.dataset.wired) return;
  sheet.dataset.wired = "1";

  const editSlider = document.getElementById("edit-pain-slider");
  const editPain = document.getElementById("edit-pain-value");
  syncPainUI(editSlider, editPain);
  editSlider?.addEventListener("input", () => syncPainUI(editSlider, editPain));

  document.getElementById("edit-tag-picker")?.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-tag]");
    if (!btn) return;
    const tag = btn.dataset.tag;
    if (editSelectedTags.has(tag)) {
      editSelectedTags.delete(tag);
      btn.classList.remove("selected");
    } else {
      editSelectedTags.add(tag);
      btn.classList.add("selected");
    }
  });

  sheet.querySelectorAll("[data-edit-dismiss]").forEach((el) => {
    el.addEventListener("click", closeEditSheet);
  });
  document.getElementById("btn-entry-edit-save")?.addEventListener("click", saveEditSheet);
}

function openEditFromRow(row) {
  let data;
  try {
    data = JSON.parse(row.dataset.log || "{}");
  } catch {
    return;
  }
  openEditSheet(data);
}

async function openEditSheet(log) {
  editingLogId = log.id;
  editSelectedTags.clear();
  const when = document.getElementById("entry-edit-when");
  if (when) when.textContent = log.created_at ? formatWhen(log.created_at) : "";

  const slider = document.getElementById("edit-pain-slider");
  const painValue = document.getElementById("edit-pain-value");
  if (slider) {
    slider.value = String(log.pain_level || 5);
    syncPainUI(slider, painValue);
  }
  const notes = document.getElementById("edit-log-notes");
  if (notes) notes.value = log.short_notes || "";

  const keys = String(log.tags || "")
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
  keys.forEach((k) => editSelectedTags.add(k));

  await fillEditTagPicker();
  const st = document.getElementById("entry-edit-status");
  if (st) {
    st.textContent = "";
    st.classList.remove("error");
  }
  const sheet = document.getElementById("entry-edit-sheet");
  if (sheet) {
    sheet.hidden = false;
    document.body.classList.add("dialog-open");
  }
}

async function fillEditTagPicker() {
  const picker = document.getElementById("edit-tag-picker");
  if (!picker) return;
  try {
    const data = await api("/api/tags");
    const ordered = data.tags || [];
    picker.innerHTML = ordered
      .map((t) => {
        const sel = editSelectedTags.has(t.key) ? ' class="selected"' : "";
        return `<button type="button"${sel} data-tag="${escapeAttr(t.key)}">${escapeHtml(t.label || t.key)}</button>`;
      })
      .join("");
  } catch {
    picker.innerHTML = `<span class="muted">Could not load tags</span>`;
  }
}

function closeEditSheet() {
  const sheet = document.getElementById("entry-edit-sheet");
  if (sheet) sheet.hidden = true;
  document.body.classList.remove("dialog-open");
  editingLogId = null;
  closeOpenSwipe();
}

async function saveEditSheet() {
  if (!editingLogId) return;
  const st = document.getElementById("entry-edit-status");
  const btn = document.getElementById("btn-entry-edit-save");
  st?.classList.remove("error");
  if (st) st.textContent = "Saving…";
  if (btn) btn.disabled = true;
  try {
    await api(`/api/logs/${encodeURIComponent(editingLogId)}`, {
      method: "PATCH",
      body: {
        pain_level: Number(document.getElementById("edit-pain-slider")?.value || 5),
        short_notes: document.getElementById("edit-log-notes")?.value || "",
        tags: [...editSelectedTags].join(","),
      },
    });
    closeEditSheet();
    await refreshLogs();
    const logSt = document.getElementById("log-status");
    if (logSt) {
      logSt.textContent = "Entry updated";
      logSt.classList.remove("error");
    }
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Update failed";
      st.classList.add("error");
    }
  } finally {
    if (btn) btn.disabled = false;
  }
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
