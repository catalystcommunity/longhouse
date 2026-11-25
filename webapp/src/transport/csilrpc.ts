/** CSIL-RPC over HTTP for the generated Longhouse clients. */
import { type AsyncServiceTransport, fromServiceErrorCbor } from "@longhouse/client";
import { currentToken } from "~/stores/auth";
import { RpcRequest, RpcResponse, Status, statusName } from "~/transport/csil";

export class ServiceError extends Error {
  constructor(message: string, public readonly code: number) {
    super(message);
  }
}

const CONTENT_TYPE = "application/cbor";
// The canonical carrier mount is /csil/v1/rpc; longhouse also serves it under
// /api so it rides the SPA proxy and gateway (which only carry /api/*) with no
// extra routing. The envelope self-routes, so the path is not semantic.
const RPC_ENDPOINT = "/api/csil/v1/rpc";

// Called once when a request comes back unauthenticated. lib/session
// registers resumeSession here (the transport cannot import it: that module
// imports the generated clients, which import this one). It resolves true
// when there is a usable bearer to retry with.
let onUnauthenticated: (() => Promise<boolean>) | null = null;
export function setUnauthenticatedHandler(fn: () => Promise<boolean>): void {
  onUnauthenticated = fn;
}

// Ops that must not trigger a resume-and-retry: the resume itself, and the
// sign-out that clears the cookie.
const NO_RETRY_OPS = new Set(["resume-session", "end-session", "begin-browser-login", "complete"]);

// Preserve the variant for generated operations with multiple reply arms.
export const cborTransport: AsyncServiceTransport = {
  async call(service, op, req) {
    return (await this.callReply(service, op, req)).payload;
  },
  async callReply(service, op, req) {
    try {
      return await send(service, op, req);
    } catch (e) {
      const kebab = methodToOp(op);
      if (!(e instanceof ServiceError) || e.code !== 401 || !onUnauthenticated || NO_RETRY_OPS.has(kebab)) throw e;
      if (!(await onUnauthenticated())) throw e;
      return send(service, op, req);
    }
  },
};

async function send(service: string, op: string, req: Uint8Array): Promise<RpcResponse> {
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
  return resp;
}

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
