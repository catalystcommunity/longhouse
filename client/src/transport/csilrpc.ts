/**
 * CSIL-RPC transport. POSTs CBOR-encoded request bodies to
 * /api/csil/{service}/{method}, returns the CBOR-decoded response, and
 * maps non-2xx responses to a thrown ServiceError carrying the api's
 * {code, message} body. Implements the ServiceTransport interface the
 * generated client classes expect — one instance, used everywhere.
 *
 * Wire convention quirk: csilgen emits Go structs with snake_case JSON
 * tags AND camelCase TypeScript interfaces. fxamacker/cbor on the Go side
 * follows the JSON tag, so the wire is snake_case. We close the gap right
 * here — every outbound body is walked camel→snake before encode, every
 * decoded response is walked snake→camel before return — so the generated
 * client classes see the camelCase types they were promised.
 */

import { decode, encode } from "cbor-x";
import { currentToken } from "~/stores/auth";

export class ServiceError extends Error {
  constructor(message: string, public readonly code: number) {
    super(message);
  }
}

export interface ServiceTransport {
  call<TReq, TRes>(
    service: string,
    method: string,
    req: TReq,
    opts?: { signal?: AbortSignal },
  ): Promise<TRes>;
}

const CONTENT_TYPE = "application/cbor";

/**
 * cborTransport is the singleton ServiceTransport every generated client
 * uses. Each call:
 *   - CBOR-encodes the request body (skipping the encode entirely for the
 *     EmptyRequest case — the api accepts a zero-length body as the empty
 *     map so we don't pay the encoder cost for /me, /refresh etc.)
 *   - Attaches the current bearer token, if any
 *   - Reads the response body and either decodes the success payload or
 *     throws a ServiceError with the api's {code, message}
 */
export const cborTransport: ServiceTransport = {
  async call<TReq, TRes>(
    service: string,
    method: string,
    req: TReq,
    opts?: { signal?: AbortSignal },
  ): Promise<TRes> {
    const headers: Record<string, string> = { "Content-Type": CONTENT_TYPE, Accept: CONTENT_TYPE };
    const tok = currentToken();
    if (tok) headers["Authorization"] = `Bearer ${tok}`;

    const body = isEmpty(req) ? new Uint8Array() : encode(camelToSnakeDeep(req));

    const res = await fetch(`/api/csil/${service}/${method}`, {
      method: "POST",
      headers,
      body,
      signal: opts?.signal,
    });

    const buf = new Uint8Array(await res.arrayBuffer());
    if (!res.ok) {
      // The api always returns a CBOR ServiceError on failure. Fall back to
      // a generic message if decoding fails so the throw still carries the
      // status.
      let msg = `${res.status} ${res.statusText}`;
      try {
        const e = decode(buf) as { code?: number; message?: string };
        if (e?.message) msg = e.message;
      } catch {
        /* swallow */
      }
      throw new ServiceError(msg, res.status);
    }
    if (buf.byteLength === 0) return undefined as TRes;
    return snakeToCamelDeep(decode(buf)) as TRes;
  },
};

// EmptyRequest is the canonical zero-arg call (Me/Refresh/ListDevUsers).
// The generated client passes a plain `{}` for those; encode-skip avoids
// shipping `a0` (CBOR-encoded empty map) when we can just send nothing.
function isEmpty(v: unknown): boolean {
  return v !== null && typeof v === "object" && !Array.isArray(v) && Object.keys(v as object).length === 0;
}

// ---- key-case bridges ------------------------------------------------
//
// camelToSnake / snakeToCamel walk plain JS values (arrays + objects +
// primitives) and rewrite map keys. Cycles aren't expected — request and
// response shapes are CSIL value types, all trees. Uint8Array, Date and
// other non-plain objects pass through untouched so a `bytes` field
// (Member.cached_public_key) survives the round trip.

function camelToSnakeDeep(v: unknown): unknown {
  if (Array.isArray(v)) return v.map(camelToSnakeDeep);
  if (isPlainObject(v)) {
    const out: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v)) out[camelToSnake(k)] = camelToSnakeDeep(val);
    return out;
  }
  return v;
}

function snakeToCamelDeep(v: unknown): unknown {
  if (Array.isArray(v)) return v.map(snakeToCamelDeep);
  if (isPlainObject(v)) {
    const out: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v)) out[snakeToCamel(k)] = snakeToCamelDeep(val);
    return out;
  }
  return v;
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  if (v === null || typeof v !== "object") return false;
  if (v instanceof Uint8Array || v instanceof Date || v instanceof Map || v instanceof Set) return false;
  const proto = Object.getPrototypeOf(v);
  return proto === Object.prototype || proto === null;
}

function camelToSnake(k: string): string {
  return k.replace(/[A-Z]/g, (m) => "_" + m.toLowerCase());
}

function snakeToCamel(k: string): string {
  return k.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase());
}
