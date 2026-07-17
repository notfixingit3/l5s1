import { api } from "./api.js";

/** @type {Map<string, { key: string, label: string, kind: string }>} */
let catalog = new Map();
let loaded = false;

const KIND_BY_KEY = {
  left: "side",
  right: "side",
  "both-sides": "side",
  "lower-back": "region",
  hips: "region",
  glute: "region",
  leg: "region",
  thigh: "region",
  calf: "region",
  foot: "region",
  numbing: "sensation",
  "pins-needles": "sensation",
  tingling: "sensation",
  burning: "sensation",
  "sharp-pain": "sensation",
  "dull-ache": "sensation",
  radiating: "sensation",
  cramping: "sensation",
  weakness: "function",
  stiffness: "function",
  limping: "function",
  stenosis: "condition",
  "uc-flare": "condition",
  "bp-high": "condition",
  "bp-ok": "condition",
  "glucose-high": "condition",
  "glucose-low": "condition",
  observation: "meta",
};

function kindFor(key) {
  return KIND_BY_KEY[key] || "default";
}

function humanizeKey(key) {
  return String(key || "")
    .split("-")
    .map((w) => (w ? w[0].toUpperCase() + w.slice(1) : ""))
    .join(" ");
}

export async function ensureTagCatalog() {
  if (loaded && catalog.size) return catalog;
  try {
    const data = await api("/api/tags");
    catalog = new Map();
    for (const t of data.tags || []) {
      catalog.set(t.key, {
        key: t.key,
        label: t.label || humanizeKey(t.key),
        kind: kindFor(t.key),
      });
    }
    loaded = true;
  } catch {
    /* keep empty; fallbacks still work */
  }
  return catalog;
}

export function invalidateTagCatalog() {
  loaded = false;
  catalog = new Map();
}

export function parseTagKeys(tags) {
  if (!tags) return [];
  if (Array.isArray(tags)) return tags.map(String).map((t) => t.trim()).filter(Boolean);
  return String(tags)
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
}

export function labelForTag(key) {
  const hit = catalog.get(key);
  return hit?.label || humanizeKey(key);
}

/**
 * @param {string|string[]} tags
 * @param {{ count?: number, interactive?: boolean }=} opts
 */
export function renderTagBadges(tags, opts = {}) {
  const keys = parseTagKeys(tags);
  if (!keys.length) return "";
  return `<div class="tag-badges">${keys
    .map((key) => {
      const kind = kindFor(key);
      const label = labelForTag(key);
      const count =
        opts.count != null
          ? `<span class="tag-badge-count">${opts.count}</span>`
          : typeof opts.counts === "object" && opts.counts[key] != null
            ? `<span class="tag-badge-count">${opts.counts[key]}</span>`
            : "";
      return `<span class="tag-badge tag-badge--${kind}" data-tag="${escapeAttr(key)}">${escapeHtml(label)}${count}</span>`;
    })
    .join("")}</div>`;
}

/**
 * Render frequency map as badges sorted by count desc.
 * @param {Record<string, number>} tagCounts
 */
export function renderTagCountBadges(tagCounts) {
  const entries = Object.entries(tagCounts || {}).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  if (!entries.length) {
    return `<p class="muted empty-inline">No tags in this period.</p>`;
  }
  return `<div class="tag-badges tag-badges--wrap">${entries
    .map(([key, n]) => {
      const kind = kindFor(key);
      const label = labelForTag(key);
      return `<span class="tag-badge tag-badge--${kind} tag-badge--count" data-tag="${escapeAttr(key)}">${escapeHtml(label)}<span class="tag-badge-count">${n}</span></span>`;
    })
    .join("")}</div>`;
}

export function painBadge(level) {
  const n = Number(level) || 0;
  let band = "mid";
  if (n <= 3) band = "low";
  else if (n >= 7) band = "high";
  return `<span class="badge pain-badge pain-badge--${band}">Pain ${n}</span>`;
}

export function formatWhen(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  const now = new Date();
  const sameDay =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate();
  if (sameDay) {
    return `Today · ${d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;
  }
  const yday = new Date(now);
  yday.setDate(yday.getDate() - 1);
  const isYday =
    d.getFullYear() === yday.getFullYear() &&
    d.getMonth() === yday.getMonth() &&
    d.getDate() === yday.getDate();
  if (isYday) {
    return `Yesterday · ${d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;
  }
  return d.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
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
