import {
  detectDeviceLabel,
  loginPasskey,
  logout,
  me,
  normalizeCode,
  registerPasskey,
} from "./auth.js";
import { initPatient, loadTags } from "./patient.js";
import { initPartner } from "./partner.js";
import { initClinician } from "./clinician.js";
import { initProfile, refreshProfile } from "./profile.js";
import { initAdmin, refreshAdmin } from "./admin.js";
import { APP_COMMIT, APP_VERSION } from "./version.js";

const views = {
  auth: document.getElementById("view-auth"),
  patient: document.getElementById("view-patient"),
  partner: document.getElementById("view-partner"),
  clinician: document.getElementById("view-clinician"),
  profile: document.getElementById("view-profile"),
  admin: document.getElementById("view-admin"),
};

const MODE_META = {
  patient: { title: "Home", context: "Daily check-in" },
  partner: { title: "Partner", context: "Care partner" },
  clinician: { title: "Summary", context: "Visit summary" },
  profile: { title: "Profile", context: "Account" },
  admin: { title: "Admin", context: "Admin workspace" },
};

let currentUser = null;
let mode = "patient";
let modulesReady = false;

const appEl = document.getElementById("app");
const tabbar = document.getElementById("tabbar");
const topbarActions = document.getElementById("topbar-actions");
const userChip = document.getElementById("user-chip");
const headerContext = document.getElementById("header-context");
const tabAdmin = document.getElementById("tab-admin");

function isAuthed() {
  return Boolean(currentUser && appEl?.classList.contains("is-authed"));
}

function hideAllAppViews() {
  Object.entries(views).forEach(([key, v]) => {
    if (!v) return;
    if (key === "auth") return;
    v.hidden = true;
  });
}

function lockToAuth(msg = "", isError = false) {
  currentUser = null;
  mode = "patient";
  hideAllAppViews();
  if (views.auth) views.auth.hidden = false;
  appEl?.classList.remove("is-authed");
  if (tabbar) {
    tabbar.hidden = true;
    tabbar.setAttribute("aria-hidden", "true");
  }
  if (topbarActions) topbarActions.hidden = true;
  if (userChip) userChip.hidden = true;
  if (tabAdmin) tabAdmin.hidden = true;
  if (headerContext) headerContext.textContent = "Health Registry";
  const st = document.getElementById("auth-status");
  if (st && msg) {
    st.textContent = msg;
    st.classList.toggle("error", isError);
  }
}

function showAuth(msg = "", isError = false) {
  lockToAuth(msg, isError);
}

function applyUserChip(user) {
  if (!userChip || !user) return;
  userChip.hidden = false;
  const name = user.display_name || user.username || user.email || "Profile";
  userChip.textContent = name;
  userChip.title = "Open profile";
}

function showApp() {
  if (!currentUser) {
    lockToAuth();
    return;
  }
  appEl?.classList.add("is-authed");
  if (tabbar) {
    tabbar.hidden = false;
    tabbar.setAttribute("aria-hidden", "false");
  }
  if (topbarActions) topbarActions.hidden = false;
  if (views.auth) views.auth.hidden = true;

  const isAdmin = currentUser.role === "admin";
  if (tabAdmin) tabAdmin.hidden = !isAdmin;

  applyUserChip(currentUser);

  if (mode === "admin" && !isAdmin) mode = "patient";
  setMode(mode === "auth" ? "patient" : mode);

  if (!modulesReady) {
    initPatient();
    initPartner();
    initClinician();
    initProfile({
      onChange: (user) => {
        if (!user) return;
        currentUser = user;
        applyUserChip(user);
        if (tabAdmin) tabAdmin.hidden = user.role !== "admin";
      },
    });
    initAdmin();
    modulesReady = true;
  } else if (mode === "patient") {
    loadTags();
  }
}

function setMode(next) {
  // Never leave the auth screen without a session
  if (!currentUser || !appEl?.classList.contains("is-authed")) {
    lockToAuth();
    return;
  }

  if (next === "admin" && currentUser.role !== "admin") {
    next = "patient";
  }
  mode = next in MODE_META ? next : "patient";
  const meta = MODE_META[mode];

  document.querySelectorAll("#tabbar .tab").forEach((b) => {
    if (b.hidden) return;
    const on = b.dataset.mode === mode;
    b.classList.toggle("active", on);
    if (on) b.setAttribute("aria-current", "page");
    else b.removeAttribute("aria-current");
  });
  if (mode === "profile") {
    document.querySelectorAll("#tabbar .tab").forEach((b) => {
      b.classList.remove("active");
      b.removeAttribute("aria-current");
    });
  }

  if (headerContext) headerContext.textContent = meta.context;

  views.patient.hidden = mode !== "patient";
  views.partner.hidden = mode !== "partner";
  views.clinician.hidden = mode !== "clinician";
  if (views.profile) views.profile.hidden = mode !== "profile";
  if (views.admin) views.admin.hidden = mode !== "admin";
  if (views.auth) views.auth.hidden = true;

  if (mode === "profile") {
    refreshProfile().then((user) => {
      if (!isAuthed()) return;
      if (user) {
        currentUser = user;
        applyUserChip(user);
      }
    });
  }
  if (mode === "admin") {
    if (currentUser.role !== "admin") {
      setMode("patient");
      return;
    }
    refreshAdmin();
  }
  if (mode === "patient") {
    loadTags();
  }

  try {
    window.scrollTo(0, 0);
  } catch {
    /* ignore */
  }
}

tabbar?.addEventListener("click", (e) => {
  if (!isAuthed()) {
    e.preventDefault();
    lockToAuth();
    return;
  }
  const btn = e.target.closest("button[data-mode]");
  if (btn && !btn.hidden) setMode(btn.dataset.mode);
});

userChip?.addEventListener("click", () => {
  if (!isAuthed()) {
    lockToAuth();
    return;
  }
  setMode("profile");
});

/* ——— Auth tabs ——— */
function setAuthTab(which) {
  const panelSignin = document.getElementById("panel-signin");
  const panelSignup = document.getElementById("panel-signup");
  const panelLink = document.getElementById("panel-link");
  if (panelSignin) panelSignin.hidden = which !== "signin";
  if (panelSignup) panelSignup.hidden = which !== "signup";
  if (panelLink) panelLink.hidden = which !== "link";
  document.querySelectorAll(".auth-tab").forEach((t) => {
    const on = t.dataset.authTab === which;
    t.classList.toggle("active", on);
    t.setAttribute("aria-selected", on ? "true" : "false");
  });
  const st = document.getElementById("auth-status");
  if (st) {
    st.textContent = "";
    st.classList.remove("error");
  }
}

document.querySelectorAll(".auth-tab").forEach((t) => {
  t.addEventListener("click", () => setAuthTab(t.dataset.authTab));
});

function authStatusEl() {
  return document.getElementById("auth-status");
}

function setAuthBusy(busy) {
  const login = document.getElementById("btn-login");
  const reg = document.getElementById("btn-register");
  const link = document.getElementById("btn-link-device");
  if (login) login.disabled = busy;
  if (reg) reg.disabled = busy;
  if (link) link.disabled = busy;
}

function friendlyAuthError(err) {
  const msg = err?.message || String(err);
  const detail = err?.data?.detail;
  // Help seeded users who need a first passkey
  if (/no passkeys/i.test(msg)) {
    return "No passkey yet — open Create account with this username (invite not required for existing demo accounts).";
  }
  if (/device code|add another passkey/i.test(msg)) {
    return msg;
  }
  if (detail) return `${msg}: ${detail}`;
  return msg;
}

/** Live-format xxxx-xxxx in code inputs */
function wireCodeInput(id) {
  const el = document.getElementById(id);
  if (!el) return;
  el.addEventListener("input", () => {
    const digits = normalizeCode(el.value);
    const caretAtEnd = el.selectionStart === el.value.length;
    el.value = digits.length > 4 ? `${digits.slice(0, 4)}-${digits.slice(4)}` : digits;
    if (caretAtEnd) el.setSelectionRange(el.value.length, el.value.length);
  });
}
wireCodeInput("signup-invite");
wireCodeInput("link-code");

document.getElementById("btn-register")?.addEventListener("click", async () => {
  const invite = normalizeCode(document.getElementById("signup-invite")?.value || "");
  const username = document.getElementById("signup-username").value.trim();
  const email = document.getElementById("signup-email").value.trim();
  const displayName = document.getElementById("signup-display-name").value.trim();
  const st = authStatusEl();
  st.classList.remove("error");

  if (!username || username.length < 3) {
    st.textContent = "Choose a username (at least 3 characters)";
    st.classList.add("error");
    return;
  }
  if (!email || !email.includes("@")) {
    st.textContent = "Enter a valid email address";
    st.classList.add("error");
    return;
  }
  if (!displayName) {
    st.textContent = "Enter a display name";
    st.classList.add("error");
    return;
  }
  // Invite is optional in the form: required only for brand-new usernames (server enforces).
  if (invite && invite.length !== 8) {
    st.textContent = "Invite code must be 8 digits (xxxx-xxxx), or leave blank for a pre-created account";
    st.classList.add("error");
    return;
  }

  setAuthBusy(true);
  st.textContent = "Creating passkey…";
  try {
    await registerPasskey({
      username,
      email,
      display_name: displayName,
      invite_code: invite,
      device_type: detectDeviceLabel(),
    });
    currentUser = await me();
    st.textContent = "Account ready";
    showApp();
  } catch (err) {
    st.textContent = friendlyAuthError(err);
    st.classList.add("error");
  } finally {
    setAuthBusy(false);
  }
});

document.getElementById("btn-link-device")?.addEventListener("click", async () => {
  const username = document.getElementById("link-username")?.value.trim() || "";
  const code = normalizeCode(document.getElementById("link-code")?.value || "");
  const st = authStatusEl();
  st.classList.remove("error");
  if (!username || username.length < 3) {
    st.textContent = "Enter your username";
    st.classList.add("error");
    return;
  }
  if (code.length !== 8) {
    st.textContent = "Enter the 8-digit device code (xxxx-xxxx)";
    st.classList.add("error");
    return;
  }
  setAuthBusy(true);
  st.textContent = "Creating passkey on this device…";
  try {
    await registerPasskey({
      username,
      device_link_code: code,
      device_type: detectDeviceLabel(),
    });
    currentUser = await me();
    st.textContent = "Device linked — you’re signed in";
    showApp();
  } catch (err) {
    st.textContent = friendlyAuthError(err);
    st.classList.add("error");
  } finally {
    setAuthBusy(false);
  }
});

document.getElementById("btn-login")?.addEventListener("click", async () => {
  const identifier = document.getElementById("login-identifier").value.trim();
  const st = authStatusEl();
  st.classList.remove("error");
  if (!identifier) {
    st.textContent = "Enter your username or email";
    st.classList.add("error");
    return;
  }
  setAuthBusy(true);
  st.textContent = "Waiting for passkey…";
  try {
    await loginPasskey(identifier);
    currentUser = await me();
    st.textContent = "Signed in";
    showApp();
  } catch (err) {
    st.textContent = friendlyAuthError(err);
    st.classList.add("error");
  } finally {
    setAuthBusy(false);
  }
});

document.getElementById("btn-logout")?.addEventListener("click", async () => {
  try {
    await logout();
  } catch {
    /* ignore */
  }
  lockToAuth("Logged out");
  setAuthTab("signin");
});

function paintVersion() {
  // Short pill for layout (long "+commit" was wrapping the header subtitle).
  // Full identity stays on title hover / long-press tooltip.
  const short = `v${APP_VERSION}`;
  const full =
    APP_COMMIT && APP_COMMIT !== "dev" ? `v${APP_VERSION}+${APP_COMMIT}` : short;
  document.querySelectorAll("#app-version, #app-version-authed").forEach((el) => {
    el.textContent = short;
    el.title = `L5S1 ${full}`;
  });
  // Prefer server-reported version when available (matches container image)
  fetch("/api/version", { credentials: "include" })
    .then((r) => (r.ok ? r.json() : null))
    .then((data) => {
      if (!data?.display && !data?.version) return;
      const product = data.version ? `v${data.version}` : short;
      const detail = data.display || product;
      const tip =
        `L5S1 ${detail}` + (data.build_time ? ` · built ${data.build_time}` : "");
      document.querySelectorAll("#app-version, #app-version-authed").forEach((el) => {
        el.textContent = product;
        el.title = tip;
      });
    })
    .catch(() => {});
}

paintVersion();

// Session restore
(async () => {
  try {
    currentUser = await me();
    showApp();
  } catch {
    lockToAuth();
    setAuthTab("signin");
  }
})();
