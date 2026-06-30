/**
 * CSIL-RPC carrier for the generated async client (@longhouse/client).
 *
 * The generated client owns (de)serialization via its codec; this carrier is the
 * "dumb byte transport" seam it calls — `call(service, op, reqBytes) ->
 * Promise<respBytes>`. Each call wraps the already-encoded request payload in a
 * CSIL-RPC envelope and POSTs it to the single carrier endpoint:
 *
 *   POST /api/csil/v1/rpc
 *   Content-Type: application/cbor
 *   body  = CsilRpcRequest { v, service, op, payload: 24(reqBytes) }
 *   reply = CsilRpcResponse { v, status, payload: 24(respBytes), ? variant, ? error }
 *
 * Routing lives in the envelope (`service`/`op`); the HTTP path is not semantic.
 * The op the generated client passes is the PascalCase CSIL method name
 * ("ListTasks"); the server's envelope handler keys on the canonical kebab-case op
 * ("list-tasks"), so we convert here. The envelope `status` is the transport
 * outcome (HTTP is always 200 when an envelope is present). Two failure channels,
 * never conflated:
 *
 *   - a non-zero transport `status` (malformed/unknown-op/unauthenticated/…)
 *   - an application error: status 0 with `variant === "ServiceError"`, carrying
 *     the typed { code, message } arm.
 *
 * Both surface to callers as a thrown {@link ServiceError}, preserving the
 * { message, code } shape the app's catch sites expect.
 *
 * Unlike the previous transport, there is no camel↔snake bridging and no cbor-x:
 * the generated codec emits the request bytes with the correct snake_case wire
 * keys directly, and decodes the response bytes the generated client hands back.
 * This carrier only moves the envelope.
 */

import { fromServiceErrorCbor } from "@longhouse/client";
import { currentToken } from "~/stores/auth";
import { RpcRequest, RpcResponse, Status, statusName } from "~/transport/csil";

export class ServiceError extends Error {
  constructor(message: string, public readonly code: number) {
    super(message);
  }
}

/**
 * The byte-transport seam the generated async client constructs over. Mirrors
 * `AsyncServiceTransport` from @longhouse/client (kept local so the carrier does
 * not depend on the generated type for its own shape).
 */
export interface ServiceTransport {
  call(service: string, op: string, req: Uint8Array): Promise<Uint8Array>;
}

const CONTENT_TYPE = "application/cbor";
// The canonical carrier mount is /csil/v1/rpc; longhouse also serves it under
// /api so it rides the SPA proxy and gateway (which only carry /api/*) with no
// extra routing. The envelope self-routes, so the path is not semantic.
const RPC_ENDPOINT = "/api/csil/v1/rpc";

/**
 * cborTransport is the singleton carrier every generated client shares. Each call
 * wraps the request bytes in a CSIL-RPC envelope, POSTs it, and routes the
 * response: the success arm's inner payload bytes are returned for the client to
 * decode; a declared `ServiceError` arm or a non-zero transport status becomes a
 * thrown {@link ServiceError}.
 */
export const cborTransport: ServiceTransport = {
  async call(service: string, op: string, req: Uint8Array): Promise<Uint8Array> {
    const headers: Record<string, string> = { "Content-Type": CONTENT_TYPE, Accept: CONTENT_TYPE };
    const tok = currentToken();
    if (tok) headers["Authorization"] = `Bearer ${tok}`;

    const envelope = new RpcRequest(service, methodToOp(op), req).encode();

    const res = await fetch(RPC_ENDPOINT, {
      method: "POST",
      headers,
      body: new Uint8Array(envelope),
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
      // Declared application-error arm: decode the typed { code, message } with
      // the generated codec and rethrow as the app's ServiceError.
      const e = fromServiceErrorCbor(resp.payload) as { code?: number; message?: string };
      throw new ServiceError(e?.message ?? "request failed", e?.code ?? 500);
    }

    // Success arm: hand the inner payload bytes back to the generated client,
    // which decodes them into the typed response.
    return resp.payload;
  },
};

// methodToOp converts the generated PascalCase method ("ListTasks") to the
// canonical kebab-case CSIL op ("list-tasks") the envelope carries — the exact
// inverse of the server's op→method mapping. longhouse ops contain no digit or
// acronym runs, so this is unambiguous.
function methodToOp(method: string): string {
  return method.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
}

// transportStatusToCode maps a CSIL-RPC transport status onto the HTTP-shaped
// code the app's error handling already keys on, so auth/availability failures
// carried in the envelope keep existing catch sites working.
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
