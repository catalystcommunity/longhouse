/**
 * CBOR-over-HTTP transport stub.
 *
 * Longhouse is CSIL-first; this client uses HTTP+CBOR as its secondary
 * transport (browsers don't speak raw TCP). When the API exposes the
 * matching endpoints, this file becomes the single place that knows
 * about wire format. Pages stay unchanged — they go through repo.ts,
 * which will swap MockRepo for an HttpCborRepo.
 *
 * For now this exists so the import graph is set up; flipping the
 * client from mocks to live is a one-file swap.
 */

import { Encoder, Decoder } from "cbor-x";
import { currentToken } from "~/stores/auth";
import { CborError } from "./http";

export { CborError };

const encoder = new Encoder({
  // tag handling for time/date/etc. can be configured here when we
  // see the actual API payloads.
  useRecords: false,
});
const decoder = new Decoder({
  // CBOR maps decode to plain JS objects, not Map instances — our domain
  // types are interfaces, and callers expect `resp.token`, not `resp.get(...)`.
  mapsAsObjects: true,
  useRecords: false,
});

const APPLICATION_CBOR = "application/cbor";

export interface CborRequestInit extends Omit<RequestInit, "body" | "headers"> {
  body?: unknown;
  headers?: Record<string, string>;
}

export class CborHttpClient {
  constructor(private readonly baseUrl: string) {}

  async request<T = unknown>(path: string, init: CborRequestInit = {}): Promise<T> {
    const { body, headers = {}, ...rest } = init;
    const token = currentToken();
    const res = await fetch(this.baseUrl + path, {
      ...rest,
      headers: {
        accept: APPLICATION_CBOR,
        ...(body !== undefined ? { "content-type": APPLICATION_CBOR } : {}),
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        ...headers,
      },
      // cbor-x.encode returns a Uint8Array at runtime; cast through
      // BodyInit because TS's strict Uint8Array<ArrayBufferLike> generic
      // doesn't unify cleanly with the lib.dom BufferSource here.
      body: body !== undefined ? (encoder.encode(body) as unknown as BodyInit) : undefined,
    });
    if (!res.ok) {
      throw new CborError(`${res.status} ${res.statusText}`, res.status);
    }
    const buf = new Uint8Array(await res.arrayBuffer());
    if (buf.length === 0) return undefined as T;
    return decoder.decode(buf) as T;
  }

  get<T>(path: string)  { return this.request<T>(path, { method: "GET" }); }
  post<T>(path: string, body: unknown) { return this.request<T>(path, { method: "POST", body }); }
  put<T>(path: string,  body: unknown) { return this.request<T>(path, { method: "PUT",  body }); }
  del<T>(path: string)  { return this.request<T>(path, { method: "DELETE" }); }
}
