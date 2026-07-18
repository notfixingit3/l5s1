import { api } from "./api.js";

/** @returns {Promise<boolean>} */
export async function isPushSupported() {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

export async function getPushStatus() {
  try {
    return await api("/api/push/status");
  } catch {
    return { enabled: false, subscribed: false, subscription_count: 0 };
  }
}

/**
 * Request permission, subscribe with VAPID, register with API.
 * @returns {Promise<{ok:boolean, message:string}>}
 */
export async function enablePush() {
  if (!(await isPushSupported())) {
    return { ok: false, message: "This browser does not support push notifications" };
  }
  if (!window.isSecureContext && location.hostname !== "localhost") {
    return { ok: false, message: "Push requires HTTPS" };
  }

  let keyData;
  try {
    keyData = await api("/api/push/vapid-public-key");
  } catch (err) {
    return { ok: false, message: err.message || "Push not available on server" };
  }
  if (!keyData?.public_key) {
    return { ok: false, message: "Push not configured on server" };
  }

  const perm = await Notification.requestPermission();
  if (perm !== "granted") {
    return { ok: false, message: "Notification permission denied" };
  }

  const reg = await navigator.serviceWorker.ready;
  let sub = await reg.pushManager.getSubscription();
  if (!sub) {
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(keyData.public_key),
    });
  }

  const json = sub.toJSON();
  await api("/api/push/subscribe", {
    method: "POST",
    body: {
      endpoint: json.endpoint,
      keys: {
        p256dh: json.keys.p256dh,
        auth: json.keys.auth,
      },
    },
  });
  return { ok: true, message: "Push notifications enabled on this device" };
}

/** Unsubscribe this browser and remove server row. */
export async function disablePush() {
  if (!(await isPushSupported())) {
    return { ok: true, message: "Nothing to disable" };
  }
  const reg = await navigator.serviceWorker.ready;
  const sub = await reg.pushManager.getSubscription();
  if (sub) {
    const endpoint = sub.endpoint;
    try {
      await api("/api/push/subscribe", {
        method: "DELETE",
        body: { endpoint },
      });
    } catch {
      /* still unsubscribe locally */
    }
    await sub.unsubscribe();
  }
  return { ok: true, message: "Push notifications disabled on this device" };
}

function urlBase64ToUint8Array(base64String) {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const arr = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) arr[i] = raw.charCodeAt(i);
  return arr;
}
