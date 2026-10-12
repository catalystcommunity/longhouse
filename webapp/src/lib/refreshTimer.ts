import { useSession } from "~/stores/auth";
import { REFRESH_AHEAD_MS, resumeSession } from "./session";

/**
 * Token-refresh loop. The bearer token is a mint-time snapshot of the
 * caller's per-house roles, so role/membership changes only take effect on
 * the next refresh (or relogin). We resume the browser session (cookie) when
 * there are fewer than REFRESH_AHEAD_MS left on the token, polling every
 * CHECK_INTERVAL_MS.
 *
 * Phones pause timers in background tabs, so the loop also checks when the
 * page becomes visible again, comes back from the back/forward cache, or
 * goes back online. An expired bearer still resumes: the cookie, not the
 * bearer, proves the session.
 *
 * Failures are left to resumeSession: it drops an expired bearer so the
 * sign-in card shows, and keeps one that still works.
 */

const CHECK_INTERVAL_MS = 60 * 1000;

const session = useSession();

const tick = async () => {
  const s = session();
  if (!s) return;
  const expiresMs = Date.parse(s.expiresAt) - Date.now();
  if (!Number.isNaN(expiresMs) && expiresMs > REFRESH_AHEAD_MS) return;
  await resumeSession();
};

export function startTokenRefresh() {
  void tick();
  // App-lifetime timer and listeners: deliberately not retained or cleared.
  // The SPA has no "shut down" path that would need cleanup, and the per-tick
  // work is a no-op when there's no session.
  setInterval(() => void tick(), CHECK_INTERVAL_MS);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") void tick();
  });
  window.addEventListener("pageshow", () => void tick());
  window.addEventListener("online", () => void tick());
}
