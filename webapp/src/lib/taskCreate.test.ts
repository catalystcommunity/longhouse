import { describe, expect, it } from "vitest";
import { decode, mapGet, toTaskCbor } from "@longhouse/client";
import { normalizeTaskCreate } from "./taskCreate";

describe("normalizeTaskCreate", () => {
  it("fills generated-codec-required receive-only fields for blank-tag creates", () => {
    const task = normalizeTaskCreate({
      houseId: "house-1",
      ownerMemberId: "",
      title: "Repair gate",
      tag: undefined,
    });

    const value = decode(toTaskCbor(task));

    expect(mapGet(value, "task_id")).toBe("");
    expect(mapGet(value, "created_at")).toBe("");
    expect(mapGet(value, "updated_at")).toBe("");
    expect(mapGet(value, "tag")).toBeUndefined();
  });
});
