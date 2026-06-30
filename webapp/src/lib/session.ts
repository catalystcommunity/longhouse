/**
 * Session helpers — small wrappers around the generated AuthClient that
 * also push results into the auth store and refresh the house list. Pages
 * call finishLogin after any login flow (browser callback or dev-login)
 * and loadHouses on demand.
 */

import { authClient } from "~/data/clients";
import { setHouses, signIn } from "~/stores/auth";
import type { LoginResponse } from "@longhouse/client";

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

export type { LoginResponse };
