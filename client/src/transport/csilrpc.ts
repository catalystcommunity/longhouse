/**
 * CSIL-RPC v1 transport (canonical envelope wire).
 *
 * The generated client calls `call(service, method, req)`; this transport maps
 * that onto a CSIL-RPC envelope and POSTs it to the single carrier endpoint:
 *
 *   POST /csil/v1/rpc
 *   Content-Type: application/cbor
 *   body  = CsilRpcRequest { v, service, op, payload: 24(CBOR(req)), ? auth }
 *   reply = CsilRpcResponse { v, status, payload: 24(CBOR(res)), ? variant, ? error }
 *
 * Routing lives in the envelope (`service`/`op`), so the HTTP path is not
 * semantic. The op is the canonical kebab-case CSIL operation name, derived from
 * the generated PascalCase method; the service is the generated lower-case name
 * verbatim. The envelope `status` is the transport outcome — HTTP is always 200
 * when an envelope is present. Two failure channels, never conflated:
 *
 *   - a non-zero transport `status` (malformed/unknown-op/unauthenticated/…)
 *   - an application error: status 0 with `variant === "ServiceError"`, carrying
 *     the typed { code, message } arm.
 *
 * Both surface to callers as a thrown {@link ServiceError}, preserving the
 * { message, code } shape the app already expects.
 *
 * Wire quirk (unchanged): csilgen emits Go structs with snake_case CBOR tags but
 * camelCase TS interfaces. The inner payload is therefore walked camel→snake on
 * the way out and snake→camel on the way back, so generated client classes see
 * the camelCase types they were promised. The envelope itself is snake/kebab by
 * spec and is handled entirely by the vendored reference codec.
 */

import { decode, encode } from "cbor-x";
import { currentToken } from "~/stores/auth";
import { RpcRequest, RpcResponse, Status, statusName } from "~/transport/csil";

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
// The canonical carrier mount is /csil/v1/rpc; longhouse also serves it under
// /api so it rides the SPA proxy and gateway (which only carry /api/*) with no
// extra routing. The envelope self-routes, so the path is not semantic.
const RPC_ENDPOINT = "/api/csil/v1/rpc";

/**
 * cborTransport is the singleton ServiceTransport every generated client uses.
 * Each call wraps the request in a CSIL-RPC envelope, POSTs it, and routes the
 * response: the success arm becomes the return value; a declared `ServiceError`
 * arm or a non-zero transport status becomes a thrown {@link ServiceError}.
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

    // Inner payload = CBOR(request type), snake-cased to match the Go wire.
    // EmptyRequest (a plain {}) sends an empty payload, which the server reads as
    // the zero-value request — no need to ship `a0`.
    const payload = isEmpty(req) ? new Uint8Array() : encode(camelToSnakeDeep(req));
    const envelope = new RpcRequest(service, methodToOp(method), payload).encode();

    const res = await fetch(RPC_ENDPOINT, {
      method: "POST",
      headers,
      body: new Uint8Array(envelope),
      signal: opts?.signal,
    });

    const buf = new Uint8Array(await res.arrayBuffer());

    // The carrier returns 200 with a CBOR envelope for every outcome (the real
    // status is inside it). A non-200 with an undecodable body is a genuine
    // network/proxy failure — surface the HTTP status.
    let resp: RpcResponse;
    try {
      resp = RpcResponse.decode(buf);
    } catch {
      throw new ServiceError(res.ok ? "invalid response envelope" : `${res.status} ${res.statusText}`, res.status);
    }

    if (resp.status !== Status.Ok) {
      // Transport failure — distinct from an application error. Map onto the
      // HTTP-shaped codes the app's catch sites understand.
      throw new ServiceError(resp.error ?? statusName(resp.status), transportStatusToCode(resp.status));
    }

    if (resp.variant === "ServiceError") {
      // Declared application-error arm: the typed { code, message }.
      const e = decode(resp.payload) as { code?: number; message?: string };
      throw new ServiceError(e?.message ?? "request failed", e?.code ?? 500);
    }

    // Success arm. An empty payload (e.g. a bare EmptyResponse encoded as `a0`
    // is non-empty; a truly empty payload only happens on a zero-byte reply)
    // returns undefined; otherwise decode + camelCase.
    if (resp.payload.byteLength === 0) return undefined as TRes;
    return snakeToCamelDeep(decode(resp.payload)) as TRes;
  },
};

// methodToOp converts a generated PascalCase method ("ListTasks") to the
// canonical kebab-case CSIL op ("list-tasks") the envelope carries. longhouse
// ops contain no digit or acronym runs, so this is the exact inverse of the
// server's op→method mapping.
function methodToOp(method: string): string {
  return method.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
}

// transportStatusToCode maps a CSIL-RPC transport status onto the HTTP-shaped
// code the app's error handling already keys on, so moving auth/availability
// failures from the HTTP layer into the envelope keeps existing catch sites working.
function transportStatusToCode(status: number): number {
  switch (status) {
    case Status.MalformedEnvelope:
      return 400;
    case Status.UnknownServiceOrOp:
      return 404;
    case Status.Unauthenticated:
      return 401;
    case Status.Forbidden:
      return 403;
    case Status.Unavailable:
      return 503;
    case Status.DeadlineExceeded:
      return 504;
    default:
      return 500;
  }
}

// EmptyRequest is the canonical zero-arg call (Me/Refresh/ListDevUsers). The
// generated client passes a plain `{}` for those; an empty payload avoids
// shipping `a0` when we can just send nothing.
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
