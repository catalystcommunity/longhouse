import { createSignal, createEffect, createRoot } from "solid-js";

/**
 * Auth store — the signed-in identity, the houses it can act in, and which
 * house is currently selected. The token is identity-scoped and carries
 * per-house roles internally; the SPA treats it as opaque and just sends it
 * as a bearer. House selection is an in-app concern (you log into the
 * instance, then choose a house), so it lives here, not in the token.
 *
 * Pure state — no network. The /me fetch that populates `houses` lives in
 * lib/session.ts to avoid an import cycle with the transport.
 */

const SESSION_KEY = "longhouse.session";
const HOUSES_KEY = "longhouse.houses";
const HOUSE_KEY = "longhouse.house";

export interface Session {
  /** opaque bearer — sent verbatim in Authorization: Bearer <token> */
  token: string;
  domain: string;
  userId: string;
  displayName?: string;
  /** ISO timestamp from the api's login/complete response */
  expiresAt: string;
}

export interface House {
  id: string;
  name: string;
  /** The caller's member_id in this house. Used by mutations and to filter
   *  the caller out of self-aware lists (Active members, mentions, etc.). */
  memberId: string;
  /** Role names the caller holds in this house (e.g. ["admin", "member"]).
   *  Server always re-checks; this is for UI gating of admin-only buttons. */
  roles: string[];
}

const readSession = (): Session | null => {
  try {
    const raw = localStorage.getItem(SESSION_KEY);
    if (!raw) return null;
    const s = JSON.parse(raw) as Session;
    if (s.expiresAt && Date.parse(s.expiresAt) < Date.now()) return null;
    return s;
  } catch {
    return null;
  }
};
const readHouses = (): House[] => {
  try {
    return JSON.parse(localStorage.getItem(HOUSES_KEY) ?? "[]") as House[];
  } catch {
    return [];
  }
};

const [session, setSessionSig] = createSignal<Session | null>(readSession());
const [houses, setHousesSig] = createSignal<House[]>(readHouses());
// True once a /me round-trip has populated the house list this session. Lets
// the UI tell "still loading houses" apart from "loaded, but you're in zero
// houses" — the latter needs an admin to trust your domain, not a spinner.
const [housesLoaded, setHousesLoadedSig] = createSignal<boolean>(false);
// True while the app tries to resume a browser session from its cookie (no
// usable bearer at boot). Lets the UI show "signing you in" instead of the
// sign-in card for a person who is still signed in.
const [restoring, setRestoringSig] = createSignal<boolean>(false);
const [currentHouseId, setCurrentHouseSig] = createSignal<string | null>(
  localStorage.getItem(HOUSE_KEY),
);

// Owner-wrap these effects so Solid stops warning about
// computations-without-an-owner. They are intentionally app-lifetime: the
// auth store outlives every page in the SPA, so the createRoot dispose
// handle is never called.
createRoot(() => {
  createEffect(() => {
    const s = session();
    if (s) localStorage.setItem(SESSION_KEY, JSON.stringify(s));
    else localStorage.removeItem(SESSION_KEY);
  });
  createEffect(() => localStorage.setItem(HOUSES_KEY, JSON.stringify(houses())));
  createEffect(() => {
    const id = currentHouseId();
    if (id) localStorage.setItem(HOUSE_KEY, id);
    else localStorage.removeItem(HOUSE_KEY);
  });
});

// Another tab that resumes or signs out writes localStorage; follow it so
// every tab sends the newest bearer.
if (typeof window !== "undefined") {
  window.addEventListener("storage", (e) => {
    if (e.key === SESSION_KEY) setSessionSig(readSession());
  });
}

export const useSession = () => session;
export const useHouses = () => houses;
export const useHousesLoaded = () => housesLoaded;
export const useRestoring = () => restoring;
export const setRestoring = (v: boolean) => setRestoringSig(v);

/** The session another tab stored, if it is still valid. Read inside the
 *  cross-tab resume lock so a tab adopts a bearer another tab just minted
 *  instead of rotating the cookie again. */
export const storedSession = (): Session | null => readSession();
// No session, no current house. Pages key their data resources on the house
// id; gating it here keeps them from fetching without a bearer (and failing)
// while the app resumes a session at boot. The stored id still comes back as
// soon as the session does.
const gatedHouseId = () => (session() ? currentHouseId() : null);
export const useCurrentHouseId = () => gatedHouseId;
export const isAuthenticated = () => session() !== null;

/** Resolve the currently-selected House (full record, not just id). Returns
 *  null when there's no session or no selection — components can use this
 *  to read memberId/roles without juggling lookups themselves. */
export const useCurrentHouse = () => () => {
  const id = gatedHouseId();
  if (!id) return null;
  return houses().find((h) => h.id === id) ?? null;
};

/** The caller's member_id in the currently-selected house, or null. */
export const currentMemberId = (): string | null => useCurrentHouse()()?.memberId ?? null;

/** Does the caller hold any of these role names in the current house? */
export const hasRole = (...anyOf: string[]): boolean => {
  const h = useCurrentHouse()();
  if (!h) return false;
  if (anyOf.length === 0) return true;
  // h.roles is `string[]` on the type, but a session persisted before this
  // field existed will deserialize without it. Tolerate either shape; a
  // background /me refresh on startup heals the localStorage shape.
  const roles = h.roles ?? [];
  return anyOf.some((want) => roles.includes(want));
};

/** Synchronous token read for the transport on every request. */
export const currentToken = (): string | null => session()?.token ?? null;

export const signIn = (s: Session) => setSessionSig(s);

export const signOut = () => {
  setSessionSig(null);
  setHousesSig([]);
  setCurrentHouseSig(null);
  setHousesLoadedSig(false);
};

/** Replace the house list (from /me). Keeps the current selection if it's
 *  still present, otherwise falls back to the first house. */
export const setHouses = (hs: House[]) => {
  setHousesSig(hs);
  setHousesLoadedSig(true);
  const cur = currentHouseId();
  if (!cur || !hs.some((h) => h.id === cur)) {
    setCurrentHouseSig(hs[0]?.id ?? null);
  }
};

export const selectHouse = (id: string) => setCurrentHouseSig(id);
