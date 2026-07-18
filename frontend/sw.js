/* L5S1 service worker — offline shell cache + web push */
const CACHE = "l5s1-v25-mobile-notification-panel";
const PRECACHE = [
  "/",
  "/css/app.css",
  "/js/app.js",
  "/js/api.js",
  "/js/auth.js",
  "/js/patient.js",
  "/js/partner.js",
  "/js/clinician.js",
  "/js/profile.js",
  "/js/admin.js",
  "/js/tags.js",
  "/js/dialog.js",
  "/js/notifications.js",
  "/js/push.js",
  "/js/version.js",
  "/manifest.webmanifest",
  "/assets/brand/app-icon-192.png",
  "/assets/brand/app-icon-512.png",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(PRECACHE)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  // Never cache API
  if (url.pathname.startsWith("/api/")) return;

  event.respondWith(
    caches.match(request).then((cached) => {
      const network = fetch(request)
        .then((resp) => {
          if (resp.ok && url.origin === self.location.origin) {
            const clone = resp.clone();
            caches.open(CACHE).then((c) => c.put(request, clone));
          }
          return resp;
        })
        .catch(() => cached);
      return cached || network;
    })
  );
});

/* ——— Web Push ——— */
self.addEventListener("push", (event) => {
  let data = { title: "L5S1", body: "New activity", url: "/" };
  try {
    if (event.data) {
      const parsed = event.data.json();
      data = { ...data, ...parsed };
    }
  } catch {
    try {
      const text = event.data && event.data.text();
      if (text) data.body = text;
    } catch {
      /* ignore */
    }
  }
  const title = data.title || "L5S1";
  const options = {
    body: data.body || "",
    icon: "/assets/brand/app-icon-192.png",
    badge: "/assets/brand/app-icon-192.png",
    tag: data.tag || data.kind || "l5s1",
    renotify: true,
    data: { url: data.url || "/", kind: data.kind || "" },
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const path = (event.notification.data && event.notification.data.url) || "/";
  const target = new URL(path, self.location.origin).href;
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if (client.url.startsWith(self.location.origin) && "focus" in client) {
          client.navigate(target);
          return client.focus();
        }
      }
      if (self.clients.openWindow) {
        return self.clients.openWindow(target);
      }
    })
  );
});
