import { jsonFetch } from "~/transport/http";
import { setHouses, signIn } from "~/stores/auth";

/** Shape of the api's login / complete / dev-login response (snake_case). */
export interface LoginResponse {
  token: string;
  domain: string;
  user_id: string;
  display_name?: string;
  expires_at: string;
}

interface MeResponse {
  domain: string;
  user_id: string;
  display_name?: string;
  expires_at: string;
  houses: { house_id: string; name: string }[];
}

/** Store the session from a login response, then load the caller's houses.
 *  Shared by the browser callback and the dev-login shortcut. */
export async function finishLogin(resp: LoginResponse): Promise<void> {
  signIn({
    token: resp.token,
    domain: resp.domain,
    userId: resp.user_id,
    displayName: resp.display_name,
    expiresAt: resp.expires_at,
  });
  await loadHouses();
}

/** Refresh the house list from /me (sent with the current bearer). */
export async function loadHouses(): Promise<void> {
  const me = await jsonFetch<MeResponse>("/api/v1/me");
  setHouses((me.houses ?? []).map((h) => ({ id: h.house_id, name: h.name })));
}
