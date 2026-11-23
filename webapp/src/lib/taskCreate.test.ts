import { describe, expect, it } from "vitest";
import { decode, mapGet, toCommentCbor, toEventCbor, toProjectCbor, toTaskCbor } from "@longhouse/client";
import { normalizeCommentCreate, normalizeEventCreate, normalizeProjectCreate, normalizeTaskCreate } from "./taskCreate";

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
    expect(mapGet(value, "created_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
    expect(mapGet(value, "updated_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
    expect(mapGet(value, "tag")).toBeUndefined();
  });

  it("fills generated-codec-required receive-only fields for project creates", () => {
    const value = decode(toProjectCbor(normalizeProjectCreate({
      houseId: "house-1",
      name: "Barn roof",
    })));

    expect(mapGet(value, "project_id")).toBe("");
    expect(mapGet(value, "created_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
    expect(mapGet(value, "updated_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
  });

  it("fills generated-codec-required receive-only fields for event creates", () => {
    const value = decode(toEventCbor(normalizeEventCreate({
      houseId: "house-1",
      title: "Work party",
    })));

    expect(mapGet(value, "event_id")).toBe("");
    expect(mapGet(value, "owner_member_id")).toBe("");
    expect(mapGet(value, "created_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
    expect(mapGet(value, "updated_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
  });

  it("fills generated-codec-required receive-only fields for comment creates", () => {
    const value = decode(toCommentCbor(normalizeCommentCreate({
      houseId: "house-1",
      targetType: "task",
      targetId: "task-1",
      body: "Done.",
    })));

    expect(mapGet(value, "comment_id")).toBe("");
    expect(mapGet(value, "member_id")).toBe("");
    expect(mapGet(value, "created_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
    expect(mapGet(value, "updated_at")).toEqual({ tag: 0, value: "1970-01-01T00:00:00Z" });
  });
});
