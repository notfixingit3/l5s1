/* Service worker registration, kept external so CSP can forbid inline scripts. */
if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").catch(() => {});
}
