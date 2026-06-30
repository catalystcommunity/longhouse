// End-to-end test of the CSIL-RPC carrier: it must speak the exact envelope wire
// the Go server decodes. The carrier is a dumb byte seam now — the generated
// client owns payload (de)serialization — so we assert the carrier wraps the
// request bytes verbatim, derives the kebab op, sets the bearer header, and
// returns the success payload bytes untouched. We decode the envelope it sends
// with the same reference codec the server uses, and feed back encoded responses
// across the reply channels.

import { afterEach, describe, expect, it, vi } from "vitest";
import { RpcRequest, RpcResponse } from "./csil/rpc";
import { Status } from "./csil/conventions";
import { toServiceErrorCbor } from "@longhouse/client";

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
  it("wraps request bytes in a canonical envelope: kebab op, verbatim payload, bearer header", async () => {
    // The carrier never inspects the payload — these are opaque bytes to it.
    const reqBytes = new Uint8Array([0xa1, 0x01, 0x02]);
    const respBytes = new Uint8Array([0x83, 0x01, 0x02, 0x03]);
    mockFetch(RpcResponse.ok("TaskList", respBytes));

    const result = await cborTransport.call("task", "ListTasks", reqBytes);

    expect(lastRequest?.service).toBe("task");
    expect(lastRequest?.op).toBe("list-tasks");
    // Request payload rides through the envelope untouched.
    expect(lastRequest?.payload).toEqual(reqBytes);
    // Success payload bytes are returned for the generated client to decode.
    expect(result).toEqual(respBytes);

    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(call[0]).toBe("/api/csil/v1/rpc");
    expect((call[1].headers as Record<string, string>).Authorization).toBe("Bearer test-token");
  });

  it("derives a multi-word kebab op from the method name", async () => {
    mockFetch(RpcResponse.ok("DevUsersResponse", new Uint8Array()));
    await cborTransport.call("devauth", "ListDevUsers", new Uint8Array());
    expect(lastRequest?.op).toBe("list-dev-users");
  });

  it("throws ServiceError for the declared application-error arm at status 0", async () => {
    mockFetch(RpcResponse.ok("ServiceError", toServiceErrorCbor({ code: 403, message: "forbidden" })));

    await expect(cborTransport.call("task", "DeleteTask", new Uint8Array())).rejects.toMatchObject({
      message: "forbidden",
      code: 403,
    });
  });

  it("throws ServiceError mapped from a non-zero transport status", async () => {
    mockFetch(RpcResponse.transportError(Status.Unauthenticated, "no token"));

    const err = (await cborTransport.call("task", "ListTasks", new Uint8Array()).catch((e) => e)) as ServiceError;
    expect(err).toBeInstanceOf(ServiceError);
    expect(err.code).toBe(401); // Unauthenticated → 401
    expect(err.message).toBe("no token");
  });
});
