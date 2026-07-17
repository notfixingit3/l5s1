/**
 * In-app dialogs (confirm / prompt / alert) — no browser native popups.
 */

let root = null;
let resolveFn = null;
let wired = false;

function ensureRoot() {
  if (!root) {
    root = document.getElementById("app-dialog");
  }
  if (!root) {
    root = document.createElement("div");
    root.id = "app-dialog";
    root.className = "app-dialog";
    root.hidden = true;
    root.innerHTML = `
      <div class="app-dialog-backdrop" data-dialog-dismiss></div>
      <div class="app-dialog-panel" role="dialog" aria-modal="true" aria-labelledby="app-dialog-title">
        <h3 id="app-dialog-title" class="app-dialog-title"></h3>
        <p class="app-dialog-message"></p>
        <div class="app-dialog-field" hidden>
          <label class="app-dialog-label">
            <span class="app-dialog-label-text"></span>
            <input type="text" class="app-dialog-input" autocomplete="off" />
          </label>
        </div>
        <div class="app-dialog-actions">
          <button type="button" class="secondary app-dialog-cancel">Cancel</button>
          <button type="button" class="primary app-dialog-confirm">OK</button>
        </div>
      </div>`;
    document.body.appendChild(root);
  }

  if (!wired) {
    wire(root);
    wired = true;
  }
  return root;
}

function wire(el) {
  const backdrop = el.querySelector("[data-dialog-dismiss]");
  const cancel = el.querySelector(".app-dialog-cancel");
  const confirm = el.querySelector(".app-dialog-confirm");
  const input = el.querySelector(".app-dialog-input");

  backdrop?.addEventListener("click", () => close(null));
  cancel?.addEventListener("click", () => close(null));
  confirm?.addEventListener("click", () => {
    const field = el.querySelector(".app-dialog-field");
    if (field && !field.hidden) {
      close(input.value);
    } else {
      close(true);
    }
  });
  input?.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      close(input.value);
    }
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && root && !root.hidden) {
      e.preventDefault();
      close(null);
    }
  });
}

function close(value) {
  if (!root || root.hidden) return;
  root.hidden = true;
  document.body.classList.remove("dialog-open");
  const fn = resolveFn;
  resolveFn = null;
  if (fn) fn(value);
}

/**
 * @returns {Promise<boolean>}
 */
export function appConfirm({
  title = "Confirm",
  message = "",
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  variant = "primary",
} = {}) {
  const el = ensureRoot();
  return new Promise((resolve) => {
    resolveFn = (v) => resolve(v === true);
    el.querySelector(".app-dialog-title").textContent = title;
    const msg = el.querySelector(".app-dialog-message");
    msg.textContent = message;
    msg.hidden = !message;
    el.querySelector(".app-dialog-field").hidden = true;
    const cancel = el.querySelector(".app-dialog-cancel");
    const confirmBtn = el.querySelector(".app-dialog-confirm");
    cancel.hidden = false;
    cancel.textContent = cancelLabel;
    confirmBtn.textContent = confirmLabel;
    confirmBtn.classList.toggle("danger", variant === "danger");
    confirmBtn.classList.toggle("primary", variant !== "danger");
    el.hidden = false;
    document.body.classList.add("dialog-open");
    confirmBtn.focus();
  });
}

/**
 * @returns {Promise<string|null>}
 */
export function appPrompt({
  title = "Enter a value",
  message = "",
  label = "Value",
  defaultValue = "",
  confirmLabel = "Save",
  cancelLabel = "Cancel",
  placeholder = "",
} = {}) {
  const el = ensureRoot();
  return new Promise((resolve) => {
    resolveFn = (v) => {
      if (v === null || v === true) {
        resolve(null);
        return;
      }
      resolve(String(v));
    };
    el.querySelector(".app-dialog-title").textContent = title;
    const msg = el.querySelector(".app-dialog-message");
    msg.textContent = message;
    msg.hidden = !message;
    el.querySelector(".app-dialog-field").hidden = false;
    el.querySelector(".app-dialog-label-text").textContent = label;
    const input = el.querySelector(".app-dialog-input");
    input.value = defaultValue;
    input.placeholder = placeholder;
    const cancel = el.querySelector(".app-dialog-cancel");
    const confirmBtn = el.querySelector(".app-dialog-confirm");
    cancel.hidden = false;
    cancel.textContent = cancelLabel;
    confirmBtn.textContent = confirmLabel;
    confirmBtn.classList.remove("danger");
    confirmBtn.classList.add("primary");
    el.hidden = false;
    document.body.classList.add("dialog-open");
    setTimeout(() => {
      input.focus();
      input.select();
    }, 30);
  });
}

/**
 * @returns {Promise<true>}
 */
export function appAlert({ title = "Notice", message = "", confirmLabel = "OK" } = {}) {
  const el = ensureRoot();
  return new Promise((resolve) => {
    resolveFn = () => resolve(true);
    el.querySelector(".app-dialog-title").textContent = title;
    const msg = el.querySelector(".app-dialog-message");
    msg.textContent = message;
    msg.hidden = !message;
    el.querySelector(".app-dialog-field").hidden = true;
    const cancel = el.querySelector(".app-dialog-cancel");
    const confirmBtn = el.querySelector(".app-dialog-confirm");
    cancel.hidden = true;
    confirmBtn.textContent = confirmLabel;
    confirmBtn.classList.remove("danger");
    confirmBtn.classList.add("primary");
    el.hidden = false;
    document.body.classList.add("dialog-open");
    confirmBtn.focus();
  });
}
