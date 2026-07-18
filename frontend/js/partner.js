import { api } from "./api.js";
import { ensureTagCatalog, formatWhen, painBadge, renderTagBadges } from "./tags.js";

let selectedPatientId = null;
const obsSelectedTags = new Set();

export function initPartner() {
  document.getElementById("btn-save-obs")?.addEventListener("click", saveObservation);
  document.getElementById("btn-grant")?.addEventListener("click", grantAccess);
  document.getElementById("obs-tag-picker")?.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-tag]");
    if (!btn) return;
    const tag = btn.dataset.tag;
    if (obsSelectedTags.has(tag)) {
      obsSelectedTags.delete(tag);
      btn.classList.remove("selected");
    } else {
      obsSelectedTags.add(tag);
      btn.classList.add("selected");
    }
  });
  loadPatients();
}

async function loadPatients() {
  const box = document.getElementById("partner-patients");
  try {
    const data = await api("/api/partner/patients");
    const patients = data.patients || [];
    if (!patients.length) {
      box.innerHTML = "<p class='empty-state'>No linked patients yet. Ask them to grant you access.</p>";
      return;
    }
    box.innerHTML = patients
      .map(
        (p) => `
      <div class="patient-chip">
        <span>${escapeHtml(p.patient_email || p.patient_id)} ${p.can_write ? "· write" : "· read"}</span>
        <button type="button" class="secondary" data-patient="${p.patient_id}">Open</button>
      </div>`
      )
      .join("");
    box.querySelectorAll("[data-patient]").forEach((btn) => {
      btn.addEventListener("click", () => openPatient(btn.dataset.patient));
    });
  } catch (err) {
    box.innerHTML = `<p class="status error">${escapeHtml(err.message)}</p>`;
  }
}

async function openPatient(id) {
  selectedPatientId = id;
  document.getElementById("partner-detail").hidden = false;
  const list = document.getElementById("partner-logs");
  obsSelectedTags.clear();
  await ensureTagCatalog();
  await loadPatientTags(id);
  try {
    const data = await api(`/api/partner/patients/${id}/logs`);
    const logs = data.logs || [];
    if (!logs.length) {
      list.innerHTML = `<li class="empty-state" style="border:none;background:transparent">No logs yet.</li>`;
      return;
    }
    list.innerHTML = logs
      .map((l) => {
        const notes = (l.short_notes || "").trim();
        const pain = Number(l.pain_level) || 0;
        let band = "mid";
        if (pain <= 3) band = "low";
        else if (pain >= 7) band = "high";
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
      })
      .join("");
  } catch (err) {
    list.innerHTML = `<li class="status error">${escapeHtml(err.message)}</li>`;
  }
}

async function loadPatientTags(patientId) {
  const picker = document.getElementById("obs-tag-picker");
  if (!picker) return;
  try {
    const data = await api(`/api/partner/patients/${encodeURIComponent(patientId)}/tags`);
    const groups = data.groups || [];
    if (groups.length) {
      picker.innerHTML = groups
        .map((g) => {
          const chips = (g.tags || [])
            .map((t) => {
              const key = t.key;
              const label = t.label || key;
              return `<button type="button" data-tag="${escapeAttr(key)}">${escapeHtml(label)}</button>`;
            })
            .join("");
          if (!chips) return "";
          return `<div class="tag-group" data-pack="${escapeAttr(g.key)}">
            <div class="tag-group-label">${escapeHtml(g.label || g.key)}</div>
            <div class="tag-group-chips tags">${chips}</div>
          </div>`;
        })
        .join("");
      return;
    }
    const tags = data.tags || [];
    if (!tags.length) {
      picker.innerHTML = `<span class="muted">No tags for this patient yet — they can enable packs in Profile.</span>`;
      return;
    }
    picker.innerHTML = `<div class="tag-group-chips tags">${tags
      .map((t) => `<button type="button" data-tag="${escapeAttr(t.key)}">${escapeHtml(t.label || t.key)}</button>`)
      .join("")}</div>`;
  } catch {
    picker.innerHTML = `<span class="muted">Could not load tags</span>`;
  }
}

async function saveObservation() {
  const status = document.getElementById("obs-status");
  if (!selectedPatientId) {
    status.textContent = "Select a patient first";
    status.classList.add("error");
    return;
  }
  status.classList.remove("error");
  status.textContent = "Saving…";
  try {
    await api(`/api/partner/patients/${selectedPatientId}/observations`, {
      method: "POST",
      body: {
        short_notes: document.getElementById("obs-notes").value,
        tags: [...obsSelectedTags].join(","),
      },
    });
    status.textContent = "Observation saved";
    document.getElementById("obs-notes").value = "";
    obsSelectedTags.clear();
    document.querySelectorAll("#obs-tag-picker button.selected").forEach((b) => b.classList.remove("selected"));
    await openPatient(selectedPatientId);
  } catch (err) {
    status.textContent = err.message;
    status.classList.add("error");
  }
}

async function grantAccess() {
  const status = document.getElementById("grant-status");
  status.classList.remove("error");
  try {
    await api("/api/partner/grant", {
      method: "POST",
      body: {
        partner_username: document.getElementById("grant-username").value,
        can_write: document.getElementById("grant-write").checked,
      },
    });
    status.textContent = "Access granted";
  } catch (err) {
    status.textContent = err.message;
    status.classList.add("error");
  }
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

function escapeAttr(s) {
  return escapeHtml(s);
}
