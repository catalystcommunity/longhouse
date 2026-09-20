/**
 * Session helpers — small wrappers around the generated AuthClient that
 * also push results into the auth store and refresh the house list. Pages
 * call finishLogin after any login flow (browser callback or dev-login)
 * and loadHouses on demand.
 */

import { authClient } from "~/data/clients";
import { setHouses, signIn } from "~/stores/auth";
import type { LoginResponse } from "@longhouse/client";

const AUTH_RETURN_KEY = "longhouse.auth.return_to";

/** Store the session from a login response, then load the caller's houses.
 *  Shared by the browser callback and the dev-login shortcut. */
export async function finishLogin(resp: LoginResponse): Promise<void> {
  signIn({
    token: resp.token,
    domain: resp.domain,
    userId: resp.userId,
    displayName: resp.displayName,
    expiresAt: resp.expiresAt,
  });
  await loadHouses();
}

/** Refresh the house list from auth.Me (sent with the current bearer). */
export async function loadHouses(): Promise<void> {
  const me = await authClient.me({});
  setHouses(
    (me.houses ?? []).map((h) => ({
      id: h.houseId,
      name: h.name,
      memberId: h.memberId,
      roles: h.roles ?? [],
    })),
  );
}

/** Remember one safe, same-origin route across the external Linkkeys redirect. */
export function rememberAuthReturnTo(path: string): void {
  try {
    const target = new URL(path, window.location.origin);
    if (target.origin !== window.location.origin || target.pathname === "/auth/callback") return;
    sessionStorage.setItem(AUTH_RETURN_KEY, `${target.pathname}${target.search}${target.hash}`);
  } catch {
    // Ignore an invalid return route. The callback will use the dashboard.
  }
}

/** Read and remove the route that started the browser login. */
export function takeAuthReturnTo(): string {
  const target = sessionStorage.getItem(AUTH_RETURN_KEY) ?? "/";
  sessionStorage.removeItem(AUTH_RETURN_KEY);
  return target.startsWith("/") && !target.startsWith("//") ? target : "/";
}

export type { LoginResponse };
