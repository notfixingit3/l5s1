import { api } from "./api.js";
import {
  ensureTagCatalog,
  formatWhen,
  painBadge,
  renderTagBadges,
  renderTagCountBadges,
} from "./tags.js";

const PRESETS = [
  { id: "7", label: "7 days", days: 7 },
  { id: "30", label: "30 days", days: 30 },
  { id: "90", label: "90 days", days: 90 },
];

let activePreset = "90";

export function initClinician() {
  document.getElementById("btn-clin-refresh")?.addEventListener("click", () => {
    activePreset = "custom";
    syncPresetUI();
    refreshSummary();
  });

  const presets = document.getElementById("clin-presets");
  presets?.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-days]");
    if (!btn) return;
    activePreset = btn.dataset.days;
    const days = Number(btn.dataset.days);
    const input = document.getElementById("clin-since");
    if (input) {
      const d = new Date();
      d.setDate(d.getDate() - days);
      // datetime-local value
      const pad = (n) => String(n).padStart(2, "0");
      input.value = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
    }
    syncPresetUI();
    refreshSummary();
  });

  syncPresetUI();
  refreshSummary();
}

function syncPresetUI() {
  document.querySelectorAll("#clin-presets button").forEach((b) => {
    b.classList.toggle("active", b.dataset.days === activePreset);
  });
}

async function refreshSummary() {
  const box = document.getElementById("clin-summary");
  const tagsBox = document.getElementById("clin-tags");
  const histBox = document.getElementById("clin-histogram");
  const trend = document.getElementById("clin-trend");
  const periodEl = document.getElementById("clin-period-label");
  const sinceInput = document.getElementById("clin-since");

  let q = "";
  if (sinceInput?.value) {
    const d = new Date(sinceInput.value);
    if (!Number.isNaN(d.getTime())) {
      q = `?since=${encodeURIComponent(d.toISOString())}`;
    }
  }

  await ensureTagCatalog();

  try {
    const data = await api(`/api/logs/summary${q}`);
    const since = data.since ? new Date(data.since) : null;
    const until = data.until ? new Date(data.until) : new Date();
    if (periodEl) {
      periodEl.textContent =
        since && !Number.isNaN(since.getTime())
          ? `${since.toLocaleDateString()} – ${until.toLocaleDateString()}`
          : "Last 90 days";
    }

    if (box) {
      box.innerHTML = `
        <div class="stat"><span>Entries</span><strong>${data.count ?? 0}</strong></div>
        <div class="stat"><span>Avg pain</span><strong>${Number(data.avg_pain || 0).toFixed(1)}</strong></div>
        <div class="stat"><span>Lowest</span><strong>${data.count ? data.min_pain : "—"}</strong></div>
        <div class="stat"><span>Highest</span><strong>${data.count ? data.max_pain : "—"}</strong></div>
        <div class="stat"><span>Per week</span><strong>${Number(data.entries_per_week || 0).toFixed(1)}</strong></div>
        <div class="stat"><span>Days in range</span><strong>${data.days_covered ?? "—"}</strong></div>
      `;
    }

    if (tagsBox) {
      tagsBox.innerHTML = renderTagCountBadges(data.tag_counts || {});
    }

    if (histBox) {
      histBox.innerHTML = renderPainHistogram(data.pain_histogram || []);
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

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}
