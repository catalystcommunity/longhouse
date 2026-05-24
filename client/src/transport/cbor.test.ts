import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Decoder, Encoder } from "cbor-x";
import { CborError, CborHttpClient } from "./cbor";
import { signIn, signOut } from "~/stores/auth";

const encoder = new Encoder({ useRecords: false });
const decoder = new Decoder({ mapsAsObjects: true, useRecords: false });

/** Build a minimal Response-like object the client can consume.
 *  cbor-x.encode returns a Buffer that's a *view* into a reused pool, so we
 *  copy out exactly [byteOffset, byteOffset+byteLength) — otherwise the
 *  decoder sees trailing pool bytes and throws "end of buffer not reached". */
const cborResponse = (obj: unknown, init: { ok?: boolean; status?: number } = {}) => {
  const enc = encoder.encode(obj);
  const ab = enc.buffer.slice(enc.byteOffset, enc.byteOffset + enc.byteLength);
  return {
    ok: init.ok ?? true,
    status: init.status ?? 200,
    statusText: "OK",
    arrayBuffer: async () => ab,
  } as Response;
};

describe("CborHttpClient", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    signOut();
    localStorage.clear();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  it("encodes the request body as CBOR and decodes the CBOR response", async () => {
    fetchMock.mockResolvedValueOnce(cborResponse({ token: "abc", roles: ["admin"] }));
    const client = new CborHttpClient("/api");

    const result = await client.post<{ token: string; roles: string[] }>("/v1/auth/login", {
      member_id: "m-tod",
      house_id: "h-1",
    });

    expect(result).toEqual({ token: "abc", roles: ["admin"] });

    // request was shaped correctly
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/auth/login");
    expect(init.method).toBe("POST");
    expect(init.headers["content-type"]).toBe("application/cbor");
    expect(init.headers["accept"]).toBe("application/cbor");

    // body round-trips through CBOR (runtime body is a Uint8Array)
    const sent = decoder.decode(init.body as Uint8Array);
    expect(sent).toEqual({ member_id: "m-tod", house_id: "h-1" });
  });

  it("omits the Authorization header when unauthenticated", async () => {
    fetchMock.mockResolvedValueOnce(cborResponse({}));
    await new CborHttpClient("/api").get("/v1/me");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers.authorization).toBeUndefined();
  });

  it("attaches the bearer token when authenticated", async () => {
    signIn({
      token: "jwt.abc.def",
      domain: "todandlorna.com",
      userId: "tod",
      expiresAt: new Date(Date.now() + 3600_000).toISOString(),
    });
    fetchMock.mockResolvedValueOnce(cborResponse({}));
    await new CborHttpClient("/api").get("/v1/me");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers.authorization).toBe("Bearer jwt.abc.def");
  });

  it("throws CborError with the status on a non-ok response", async () => {
    fetchMock.mockResolvedValueOnce(cborResponse({}, { ok: false, status: 401 }));
    await expect(new CborHttpClient("/api").get("/v1/me")).rejects.toMatchObject({
      status: 401,
    });
  });

  it("CborError carries the numeric status", () => {
    const e = new CborError("nope", 503);
    expect(e.status).toBe(503);
    expect(e).toBeInstanceOf(Error);
  });
});
