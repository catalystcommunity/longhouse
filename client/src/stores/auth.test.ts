import { beforeEach, describe, expect, it } from "vitest";
import {
  currentToken,
  isAuthenticated,
  selectHouse,
  setHouses,
  signIn,
  signOut,
  useCurrentHouseId,
  useHouses,
  useSession,
  type Session,
} from "./auth";

const aSession = (over: Partial<Session> = {}): Session => ({
  token: "jwt.payload.sig",
  domain: "todandlorna.com",
  userId: "tod",
  displayName: "Tod Hansmann",
  expiresAt: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
  ...over,
});

describe("auth store", () => {
  beforeEach(() => {
    signOut();
    localStorage.clear();
  });

  it("starts unauthenticated", () => {
    expect(isAuthenticated()).toBe(false);
    expect(currentToken()).toBeNull();
    expect(useSession()()).toBeNull();
  });

  it("signs in, exposes token + session, and persists", () => {
    signIn(aSession());
    expect(isAuthenticated()).toBe(true);
    expect(currentToken()).toBe("jwt.payload.sig");
    expect(useSession()()?.displayName).toBe("Tod Hansmann");
    expect(JSON.parse(localStorage.getItem("longhouse.session")!).userId).toBe("tod");
  });

  it("signs out and clears persistence", () => {
    signIn(aSession());
    setHouses([{ id: "h-1", name: "Longhouse" }]);
    signOut();
    expect(isAuthenticated()).toBe(false);
    expect(currentToken()).toBeNull();
    expect(localStorage.getItem("longhouse.session")).toBeNull();
    expect(useHouses()()).toHaveLength(0);
    expect(useCurrentHouseId()()).toBeNull();
  });

  it("setHouses defaults the selection to the first house", () => {
    signIn(aSession());
    setHouses([
      { id: "h-1", name: "Longhouse" },
      { id: "h-2", name: "Acme HQ" },
    ]);
    expect(useCurrentHouseId()()).toBe("h-1");
  });

  it("keeps the current selection if still present after a refresh", () => {
    signIn(aSession());
    setHouses([
      { id: "h-1", name: "Longhouse" },
      { id: "h-2", name: "Acme HQ" },
    ]);
    selectHouse("h-2");
    // /me refresh returns the same houses → selection sticks
    setHouses([
      { id: "h-1", name: "Longhouse" },
      { id: "h-2", name: "Acme HQ (renamed)" },
    ]);
    expect(useCurrentHouseId()()).toBe("h-2");
  });
});
