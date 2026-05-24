import { jsonFetch } from "~/transport/http";
import { useSession } from "~/stores/auth";
import { finishLogin, type LoginResponse } from "./session";

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
    const resp = await jsonFetch<LoginResponse>("/api/v1/auth/refresh", { method: "POST" });
    await finishLogin(resp);
  } catch (e) {
    // eslint-disable-next-line no-console
    console.warn("token refresh failed:", e);
  }
};

export function startTokenRefresh() {
  void tick();
  setInterval(() => void tick(), CHECK_INTERVAL_MS);
}
