import { api } from "./api.js";
import {
  ensureTagCatalog,
  formatWhen,
  labelForTag,
  painBadge,
  renderTagBadges,
  renderTagCountBadges,
} from "./tags.js";

let activePreset = "90";
let activePackFilter = "all"; // all | heart | stenosis | ...
let lastSummary = null;
let lastVisitISO = null; // date YYYY-MM-DD or null

export function initClinician() {
  // exported for mode switches
  document.getElementById("btn-clin-refresh")?.addEventListener("click", () => {
    activePreset = "custom";
    syncPresetUI();
    refreshSummary();
  });

  document.getElementById("clin-presets")?.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-days], button[data-preset]");
    if (!btn) return;
    if (btn.dataset.preset === "last_visit") {
      activePreset = "last_visit";
      const input = document.getElementById("clin-since");
      if (input) input.value = "";
      syncPresetUI();
      refreshSummary();
      return;
    }
    activePreset = btn.dataset.days;
    const days = Number(btn.dataset.days);
    const input = document.getElementById("clin-since");
    if (input) {
      const d = new Date();
      d.setDate(d.getDate() - days);
      input.value = toDatetimeLocal(d);
    }
    syncPresetUI();
    refreshSummary();
  });

  document.getElementById("clin-pack-filters")?.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-pack-filter]");
    if (!btn) return;
    activePackFilter = btn.dataset.packFilter || "all";
    syncPackFilterUI();
    refreshSummary();
  });

  document.getElementById("btn-clin-print")?.addEventListener("click", () => {
    window.print();
  });

  document.getElementById("btn-clin-share")?.addEventListener("click", shareSummary);

  document.getElementById("btn-clin-set-visit-today")?.addEventListener("click", () => {
    const el = document.getElementById("clin-last-visit");
    if (el) el.value = toDateInput(new Date());
  });

  document.getElementById("btn-clin-save-visit")?.addEventListener("click", saveLastVisit);

  syncPresetUI();
  refreshSummary();
}

/** Reload when user opens Summary tab. */
export function loadClinicianSummary() {
  return refreshSummary();
}

function syncPresetUI() {
  document.querySelectorAll("#clin-presets button").forEach((b) => {
    const id = b.dataset.preset || b.dataset.days;
    b.classList.toggle("active", id === activePreset);
  });
  const lastBtn = document.getElementById("btn-preset-last-visit");
  if (lastBtn) {
    lastBtn.disabled = !lastVisitISO;
    lastBtn.title = lastVisitISO
      ? `Since ${lastVisitISO}`
      : "Save a last visit date below first";
  }
}

function syncPackFilterUI() {
  document.querySelectorAll("#clin-pack-filters button[data-pack-filter]").forEach((b) => {
    b.classList.toggle("active", (b.dataset.packFilter || "all") === activePackFilter);
  });
}

function renderPackFilters(filters) {
  const box = document.getElementById("clin-pack-filters");
  if (!box) return;
  const list = Array.isArray(filters) && filters.length
    ? filters
    : [{ key: "all", label: "All conditions" }];
  // Keep selection if still valid
  if (!list.some((f) => f.key === activePackFilter)) {
    activePackFilter = "all";
  }
  box.innerHTML = list
    .map((f) => {
      const key = f.key || "all";
      const on = key === activePackFilter ? " active" : "";
      return `<button type="button" class="clin-pack-btn${on}" data-pack-filter="${escapeAttr(key)}">${escapeHtml(f.label || key)}</button>`;
    })
    .join("");
}

async function refreshSummary() {
  const box = document.getElementById("clin-summary");
  const tagsBox = document.getElementById("clin-tags");
  const histBox = document.getElementById("clin-histogram");
  const trend = document.getElementById("clin-trend");
  const obsList = document.getElementById("clin-observations");
  const periodEl = document.getElementById("clin-period-label");
  const sinceInput = document.getElementById("clin-since");
  const printMeta = document.getElementById("clin-print-meta");

  const params = new URLSearchParams();
  if (activePreset === "last_visit") {
    params.set("since_last_visit", "1");
  } else if (sinceInput?.value) {
    const d = new Date(sinceInput.value);
    if (!Number.isNaN(d.getTime())) {
      params.set("since", d.toISOString());
    }
  }
  if (activePackFilter && activePackFilter !== "all") {
    params.set("pack", activePackFilter);
  }
  const qs = params.toString();
  const q = qs ? `?${qs}` : "";

  await ensureTagCatalog();

  try {
    const data = await api(`/api/logs/summary${q}`);
    lastSummary = data;

    renderPackFilters(data.pack_filters);
    if (data.pack_filter) {
      activePackFilter = data.pack_filter;
    }
    syncPackFilterUI();

    if (data.last_visit_at) {
      lastVisitISO = toDateInput(new Date(data.last_visit_at));
      const visitEl = document.getElementById("clin-last-visit");
      if (visitEl && !visitEl.value) visitEl.value = lastVisitISO;
    } else {
      lastVisitISO = null;
    }
    syncPresetUI();

    const since = data.since ? new Date(data.since) : null;
    const until = data.until ? new Date(data.until) : new Date();
    const rangeText =
      since && !Number.isNaN(since.getTime())
        ? `${since.toLocaleDateString()} – ${until.toLocaleDateString()}`
        : "Last 90 days";
    if (periodEl) {
      const bits = [rangeText];
      if (data.since_source === "last_visit") bits.push("since last visit");
      if (data.pack_filter_label) bits.push(data.pack_filter_label);
      periodEl.textContent = bits.join(" · ");
    }
    if (printMeta) {
      const name = data.patient_name || "Patient";
      const focus = data.pack_filter_label ? ` · Focus: ${data.pack_filter_label}` : "";
      printMeta.hidden = false;
      printMeta.textContent = `${name} · ${rangeText}${focus} · L5S1 visit summary`;
    }

    if (box) {
      box.innerHTML = `
        <div class="stat"><span>Entries</span><strong>${data.count ?? 0}</strong></div>
        <div class="stat"><span>Avg pain</span><strong>${Number(data.avg_pain || 0).toFixed(1)}</strong></div>
        <div class="stat"><span>Lowest</span><strong>${data.count ? data.min_pain : "—"}</strong></div>
        <div class="stat"><span>Highest</span><strong>${data.count ? data.max_pain : "—"}</strong></div>
        <div class="stat"><span>Per week</span><strong>${Number(data.entries_per_week || 0).toFixed(1)}</strong></div>
        <div class="stat"><span>Partner notes</span><strong>${data.observation_count ?? 0}</strong></div>
      `;
    }

    if (tagsBox) {
      const groups = data.tag_groups || [];
      const counts = data.tag_counts || {};
      const empty = !groups.length && !Object.keys(counts).length;
      if (empty && data.pack_filter_label) {
        tagsBox.innerHTML = `<p class="muted empty-inline">No ${escapeHtml(data.pack_filter_label)} tags in this period. Switch to <strong>All conditions</strong> or tag check-ins with this pack.</p>`;
      } else {
        tagsBox.innerHTML = renderGroupedTagCounts(groups, counts);
      }
    }

    if (histBox) {
      histBox.innerHTML = renderPainHistogram(data.pain_histogram || []);
    }

    if (obsList) {
      const obs = data.observations || [];
      if (!obs.length) {
        obsList.innerHTML = emptyCoachHTML({
          kind: "observations",
          packLabel: data.pack_filter_label,
          packFilter: data.pack_filter,
        });
      } else {
        obsList.innerHTML = obs
          .map((o) => {
            const who = o.author_name || "Partner";
            const notes = (o.short_notes || "").trim();
            return `
            <li class="entry-card entry-card--observation" data-pain-band="${painBand(o.pain_level)}">
              <div class="entry-top">
                <time class="entry-when">${escapeHtml(formatWhen(o.created_at))}</time>
                <span class="badge badge-obs">${escapeHtml(who)}</span>
              </div>
              ${notes ? `<div class="entry-notes">${escapeHtml(notes)}</div>` : ""}
              ${o.tags ? renderTagBadges(o.tags) : ""}
            </li>`;
          })
          .join("");
      }
    }

    if (trend) {
      const rows = (data.trend || []).slice().reverse().slice(0, 40);
      if (!rows.length) {
        trend.innerHTML = emptyCoachHTML({
          kind: "entries",
          packLabel: data.pack_filter_label,
          packFilter: data.pack_filter,
          totalHint: data.count,
        });
      } else {
        trend.innerHTML = rows
          .map((t) => {
            const notes = (t.short_notes || "").trim();
            return `
            <li class="entry-card" data-pain-band="${painBand(t.pain_level)}">
              <div class="entry-top">
                <time class="entry-when">${escapeHtml(formatWhen(t.created_at))}</time>
                ${painBadge(t.pain_level)}
              </div>
              ${notes ? `<div class="entry-notes">${escapeHtml(notes)}</div>` : ""}
              ${t.tags ? renderTagBadges(t.tags) : ""}
            </li>`;
          })
          .join("");
      }
    }
  } catch (err) {
    if (box) box.innerHTML = `<p class="status error">${escapeHtml(err.message)}</p>`;
  }
}

function emptyCoachHTML({ kind, packLabel, packFilter }) {
  const focus = packLabel || packFilter;
  if (focus) {
    const what = kind === "observations" ? "partner notes" : "check-ins";
    return `<li class="empty-state clin-empty-coach" style="border:none;background:transparent">
      <p>No ${what} with <strong>${escapeHtml(focus)}</strong> tags in this period.</p>
      <p class="muted">Try <strong>All conditions</strong>, or tag entries with this pack on Home / Partner so they show up in a focused visit.</p>
    </li>`;
  }
  if (kind === "observations") {
    return `<li class="empty-state" style="border:none;background:transparent">No partner observations in this period.</li>`;
  }
  return `<li class="empty-state" style="border:none;background:transparent">No patient entries in this period.</li>`;
}

function renderGroupedTagCounts(groups, fallbackCounts) {
  if (Array.isArray(groups) && groups.length) {
    return groups
      .map((g) => {
        const chips = (g.tags || [])
          .map((t) => {
            const key = t.key;
            const n = t.count;
            const label = labelForTag(key);
            return `<span class="tag-badge tag-badge--count" data-tag="${escapeAttr(key)}">${escapeHtml(label)}<span class="tag-badge-count">${n}</span></span>`;
          })
          .join("");
        if (!chips) return "";
        return `<div class="tag-group clin-tag-group">
          <div class="tag-group-label">${escapeHtml(g.label || g.key)}</div>
          <div class="tag-badges tag-badges--wrap">${chips}</div>
        </div>`;
      })
      .join("");
  }
  return renderTagCountBadges(fallbackCounts || {});
}

async function saveLastVisit() {
  const el = document.getElementById("clin-last-visit");
  const st = document.getElementById("clin-visit-status");
  const raw = el?.value || "";
  st?.classList.remove("error");
  if (st) st.textContent = "Saving…";
  try {
    const body = { last_visit_at: raw || "" };
    const data = await api("/api/auth/profile", { method: "PATCH", body });
    if (data.last_visit_at) {
      lastVisitISO = toDateInput(new Date(data.last_visit_at));
      if (el) el.value = lastVisitISO;
    } else {
      lastVisitISO = null;
      if (el) el.value = "";
    }
    if (st) st.textContent = raw ? "Last visit date saved" : "Last visit date cleared";
    syncPresetUI();
    if (activePreset === "last_visit") refreshSummary();
  } catch (err) {
    if (st) {
      st.textContent = err.message || "Could not save";
      st.classList.add("error");
    }
  }
}

async function shareSummary() {
  const data = lastSummary;
  if (!data) return;
  const since = data.since ? new Date(data.since).toLocaleDateString() : "—";
  const until = data.until ? new Date(data.until).toLocaleDateString() : "—";
  const name = data.patient_name || "Patient";
  const focus = data.pack_filter_label ? `Focus: ${data.pack_filter_label}` : "Focus: All conditions";
  const lines = [
    `L5S1 visit summary — ${name}`,
    `Period: ${since} – ${until}`,
    focus,
    `Entries: ${data.count ?? 0} · Avg pain: ${Number(data.avg_pain || 0).toFixed(1)} · Range: ${data.count ? data.min_pain : "—"}–${data.count ? data.max_pain : "—"}`,
    `Partner notes: ${data.observation_count ?? 0}`,
  ];
  const groups = data.tag_groups || [];
  if (groups.length) {
    lines.push("Tags:");
    for (const g of groups) {
      const bits = (g.tags || []).map((t) => `${labelForTag(t.key)} (${t.count})`).join(", ");
      if (bits) lines.push(`  ${g.label}: ${bits}`);
    }
  }
  const obs = (data.observations || []).slice(0, 8);
  if (obs.length) {
    lines.push("Partner notes:");
    for (const o of obs) {
      const when = o.created_at ? new Date(o.created_at).toLocaleDateString() : "";
      lines.push(`  · ${when} ${o.author_name || ""}: ${(o.short_notes || "").trim()}`);
    }
  }
  const text = lines.join("\n");
  try {
    if (navigator.share) {
      await navigator.share({ title: `L5S1 summary — ${name}`, text });
      return;
    }
  } catch {
    /* user cancelled or failed — fall through to clipboard */
  }
  try {
    await navigator.clipboard.writeText(text);
    const st = document.getElementById("clin-visit-status");
    if (st) {
      st.classList.remove("error");
      st.textContent = "Summary copied to clipboard";
    }
  } catch {
    window.prompt("Copy summary:", text);
  }
}

function painBand(level) {
  const n = Number(level) || 0;
  if (n <= 3) return "low";
  if (n >= 7) return "high";
  return "mid";
}

function renderPainHistogram(hist) {
  const max = Math.max(1, ...hist.slice(1, 11));
  const bars = [];
  for (let i = 1; i <= 10; i++) {
    const n = hist[i] || 0;
    const pct = Math.round((n / max) * 100);
    const band = painBand(i);
    bars.push(`
      <div class="hist-col" title="Pain ${i}: ${n} entries">
        <div class="hist-bar-wrap">
          <div class="hist-bar hist-bar--${band}" style="height:${pct}%"></div>
        </div>
        <span class="hist-label">${i}</span>
        <span class="hist-count">${n || ""}</span>
      </div>`);
  }
  return `<div class="pain-histogram">${bars.join("")}</div>`;
}

function toDatetimeLocal(d) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function toDateInput(d) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

function escapeAttr(s) {
  return escapeHtml(s).replace(/`/g, "");
}
