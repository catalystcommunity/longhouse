/**
 * Session helpers — small wrappers around the generated AuthClient that
 * also push results into the auth store and refresh the house list. Pages
 * call finishLogin after any login flow (browser callback or dev-login)
 * and loadHouses on demand.
 *
 * A browser sign-in also starts a server-side browser session held in an
 * httpOnly cookie. resumeSession trades that cookie for a fresh bearer, so
 * the app stays signed in across launches without another IDP round trip.
 */

import { authClient } from "~/data/clients";
import { setHouses, signIn, signOut, storedSession, useSession } from "~/stores/auth";
import { setUnauthenticatedHandler } from "~/transport/csilrpc";
import type { LoginResponse } from "@longhouse/client";

const AUTH_RETURN_KEY = "longhouse.auth.return_to";
const RESUME_LOCK = "longhouse.session.resume";
/** A bearer with less than this left is renewed. */
export const REFRESH_AHEAD_MS = 5 * 60 * 1000;

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

const fresh = (expiresAt: string | undefined): boolean => {
  const ms = expiresAt ? Date.parse(expiresAt) - Date.now() : NaN;
  return !Number.isNaN(ms) && ms > REFRESH_AHEAD_MS;
};

let inFlight: Promise<boolean> | null = null;
// True while resumeLocked runs. A 401 from a call it makes (the /me in
// finishLogin) must not wait on the resume it is part of.
let resuming = false;

/**
 * Trade the browser session cookie for a fresh bearer. Returns true when the
 * app has a usable session afterwards. Calls share one request per tab, and
 * a Web Lock serializes tabs: the cookie rotates on each use, so two tabs
 * must not rotate it at once. Inside the lock a tab first adopts a bearer
 * another tab just stored.
 */
export function resumeSession(): Promise<boolean> {
  if (!inFlight) {
    inFlight = withResumeLock(resumeLocked).finally(() => {
      inFlight = null;
    });
  }
  return inFlight;
}

async function resumeLocked(): Promise<boolean> {
  resuming = true;
  try {
    return await resumeOnce();
  } finally {
    resuming = false;
  }
}

async function resumeOnce(): Promise<boolean> {
  const current = useSession()();
  const stored = storedSession();
  if (stored && fresh(stored.expiresAt) && stored.token !== current?.token) {
    signIn(stored);
    return true;
  }
  try {
    await finishLogin(await authClient.resumeSession({}));
    return true;
  } catch {
    // No cookie, or the session was revoked or expired. A bearer that still
    // works stays; an expired one is dropped so the sign-in card shows.
    const s = useSession()();
    if (s && Date.parse(s.expiresAt) > Date.now()) return true;
    if (s) signOut();
    return false;
  }
}

function withResumeLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== "undefined" ? navigator.locks : undefined;
  // request() resolves with the callback's resolved value.
  return locks ? (locks.request(RESUME_LOCK, fn) as Promise<T>) : fn();
}

/** End the browser session on the server (clears the cookie), then drop
 *  local state. Works with an expired bearer. */
export async function endSession(): Promise<void> {
  try {
    await authClient.endSession({});
  } catch {
    // Sign out locally anyway; the server session expires on its own.
  }
  signOut();
}

// A request that comes back unauthenticated gets one resume and one retry.
setUnauthenticatedHandler(() => (resuming ? Promise.resolve(false) : resumeSession()));

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
