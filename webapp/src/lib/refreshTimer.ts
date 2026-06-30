import { authClient } from "~/data/clients";
import { useSession } from "~/stores/auth";
import { finishLogin } from "./session";

/**
 * Token-refresh loop. The bearer token is a mint-time snapshot of the
 * caller's per-house roles, so role/membership changes only take effect on
 * the next refresh (or relogin). We re-mint when there are fewer than
 * REFRESH_AHEAD_MS left on the token, polling every CHECK_INTERVAL_MS. This
 * keeps the staleness window bounded without forcing the user through the
 * full assertion flow.
 *
 * Failures (expired, revoked, api unreachable) are logged but don't sign
 * the user out — let the next protected request hit a 401 and let the
 * Sign-in flow take it from there.
 */

const REFRESH_AHEAD_MS = 5 * 60 * 1000;
const CHECK_INTERVAL_MS = 60 * 1000;

const session = useSession();

const tick = async () => {
  const s = session();
  if (!s) return;
  const expiresMs = Date.parse(s.expiresAt) - Date.now();
  if (Number.isNaN(expiresMs) || expiresMs > REFRESH_AHEAD_MS || expiresMs < 0) return;
  try {
    const resp = await authClient.refresh({});
    await finishLogin(resp);
  } catch (e) {
    // eslint-disable-next-line no-console
    console.warn("token refresh failed:", e);
  }
};

export function startTokenRefresh() {
  void tick();
  // App-lifetime timer: deliberately not retained or cleared. The SPA has
  // no "shut down" path that would need cleanup, and the per-tick work is
  // a no-op when there's no session.
  setInterval(() => void tick(), CHECK_INTERVAL_MS);
}
