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

async function refreshSummary() {
  const box = document.getElementById("clin-summary");
  const tagsBox = document.getElementById("clin-tags");
  const histBox = document.getElementById("clin-histogram");
  const trend = document.getElementById("clin-trend");
  const obsList = document.getElementById("clin-observations");
  const periodEl = document.getElementById("clin-period-label");
  const sinceInput = document.getElementById("clin-since");
  const printMeta = document.getElementById("clin-print-meta");

  let q = "";
  if (activePreset === "last_visit") {
    q = "?since_last_visit=1";
  } else if (sinceInput?.value) {
    const d = new Date(sinceInput.value);
    if (!Number.isNaN(d.getTime())) {
      q = `?since=${encodeURIComponent(d.toISOString())}`;
    }
  }

  await ensureTagCatalog();

  try {
    const data = await api(`/api/logs/summary${q}`);
    lastSummary = data;

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
      let suffix = "";
      if (data.since_source === "last_visit") suffix = " · since last visit";
      periodEl.textContent = rangeText + suffix;
    }
    if (printMeta) {
      const name = data.patient_name || "Patient";
      printMeta.hidden = false;
      printMeta.textContent = `${name} · ${rangeText} · L5S1 visit summary`;
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
      tagsBox.innerHTML = renderGroupedTagCounts(data.tag_groups, data.tag_counts);
    }

    if (histBox) {
      histBox.innerHTML = renderPainHistogram(data.pain_histogram || []);
    }

    if (obsList) {
      const obs = data.observations || [];
      if (!obs.length) {
        obsList.innerHTML = `<li class="empty-state" style="border:none;background:transparent">No partner observations in this period.</li>`;
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
        trend.innerHTML = `<li class="empty-state" style="border:none;background:transparent">No patient entries in this period.</li>`;
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
  const lines = [
    `L5S1 visit summary — ${name}`,
    `Period: ${since} – ${until}`,
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
