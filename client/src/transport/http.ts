import { currentToken } from "~/stores/auth";

/** Transport error carrying the HTTP status. Shared by the JSON and CBOR
 *  clients so callers can branch on `.status` regardless of encoding. */
export class CborError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message);
  }
}

/**
 * JSON fetch helper for endpoints not yet served as CBOR (currently:
 * everything — the api is still on JSON). Attaches the bearer token when the
 * user is authenticated. Lives apart from cbor.ts so importing it doesn't
 * drag the cbor-x encoder/decoder into the bundle; the CBOR client gets
 * wired in only when the generated client + LiveRepo land.
 */
export async function jsonFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = currentToken();
  const res = await fetch(path, {
    ...init,
    headers: {
      accept: "application/json",
      ...(init.body !== undefined ? { "content-type": "application/json" } : {}),
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      ...(init.headers ?? {}),
    },
  });
  if (!res.ok) {
    let detail = "";
    try {
      detail = ((await res.json()) as { error?: string }).error ?? "";
    } catch {
      /* swallow */
    }
    throw new CborError(`${res.status} ${res.statusText}${detail ? `: ${detail}` : ""}`, res.status);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}
