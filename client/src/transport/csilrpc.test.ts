// End-to-end test of the CSIL-RPC transport: it must speak the exact envelope
// wire the Go server decodes. We mock fetch, decode the request the transport
// sends with the same reference codec the server uses, and feed back encoded
// responses across all three reply channels.

import { afterEach, describe, expect, it, vi } from "vitest";
import { decode, encode } from "cbor-x";
import { RpcRequest, RpcResponse } from "./csil/rpc";
import { Status } from "./csil/conventions";

vi.mock("~/stores/auth", () => ({ currentToken: () => "test-token" }));

import { ServiceError, cborTransport } from "./csilrpc";

let lastRequest: RpcRequest | undefined;

// Install a fetch that captures the request envelope and replies with `reply`.
function mockFetch(reply: RpcResponse, status = 200): void {
  globalThis.fetch = vi.fn(async (_url: unknown, init: { body?: BodyInit }) => {
    lastRequest = RpcRequest.decode(new Uint8Array(init.body as Uint8Array));
    return new Response(reply.encode() as unknown as BodyInit, { status });
  }) as unknown as typeof fetch;
}

afterEach(() => {
  lastRequest = undefined;
  vi.restoreAllMocks();
});

describe("cborTransport", () => {
  it("encodes a canonical envelope: kebab op, snake payload, bearer header", async () => {
    mockFetch(RpcResponse.ok("TaskList", encode({ tasks: [] })));

    const result = await cborTransport.call("task", "ListTasks", { houseId: "h-1" });

    expect(lastRequest?.service).toBe("task");
    expect(lastRequest?.op).toBe("list-tasks");
    // Payload is snake-cased to match the Go wire.
    expect(decode(lastRequest!.payload)).toEqual({ house_id: "h-1" });
    // Response payload is camel-cased back for the generated client.
    expect(result).toEqual({ tasks: [] });

    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(call[0]).toBe("/api/csil/v1/rpc");
    expect((call[1].headers as Record<string, string>).Authorization).toBe("Bearer test-token");
  });

  it("sends an empty payload for an empty request", async () => {
    mockFetch(RpcResponse.ok("MeResponse", encode({ memberId: "m-1" })));
    await cborTransport.call("auth", "Me", {});
    expect(lastRequest?.op).toBe("me");
    expect(lastRequest?.payload.byteLength).toBe(0);
  });

  it("throws ServiceError for the declared application-error arm at status 0", async () => {
    mockFetch(RpcResponse.ok("ServiceError", encode({ code: 403, message: "forbidden" })));

    await expect(cborTransport.call("task", "DeleteTask", { taskId: "t-1" })).rejects.toMatchObject({
      message: "forbidden",
      code: 403,
    });
  });

  it("throws ServiceError mapped from a non-zero transport status", async () => {
    mockFetch(RpcResponse.transportError(Status.Unauthenticated, "no token"));

    const err = (await cborTransport.call("task", "ListTasks", { houseId: "h-1" }).catch((e) => e)) as ServiceError;
    expect(err).toBeInstanceOf(ServiceError);
    expect(err.code).toBe(401); // Unauthenticated → 401
    expect(err.message).toBe("no token");
  });
});
