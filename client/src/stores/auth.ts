import { createSignal, createEffect } from "solid-js";

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
const [currentHouseId, setCurrentHouseSig] = createSignal<string | null>(
  localStorage.getItem(HOUSE_KEY),
);

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

export const useSession = () => session;
export const useHouses = () => houses;
export const useCurrentHouseId = () => currentHouseId;
export const isAuthenticated = () => session() !== null;

/** Synchronous token read for the transport on every request. */
export const currentToken = (): string | null => session()?.token ?? null;

export const signIn = (s: Session) => setSessionSig(s);

export const signOut = () => {
  setSessionSig(null);
  setHousesSig([]);
  setCurrentHouseSig(null);
};

/** Replace the house list (from /me). Keeps the current selection if it's
 *  still present, otherwise falls back to the first house. */
export const setHouses = (hs: House[]) => {
  setHousesSig(hs);
  const cur = currentHouseId();
  if (!cur || !hs.some((h) => h.id === cur)) {
    setCurrentHouseSig(hs[0]?.id ?? null);
  }
};

export const selectHouse = (id: string) => setCurrentHouseSig(id);
