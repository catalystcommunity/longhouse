import { describe, expect, it, vi } from "vitest";
import type { Event as ApiEvent, EventListRequest } from "@longhouse/client";
import { loadEvents, viewRange } from "./events";

const event = (i: number): ApiEvent => ({
  eventId: `event-${i}`, houseId: "house-1", ownerMemberId: "member-1",
  title: `Event ${i}`, createdAt: "", updatedAt: "",
});

describe("loadEvents", () => {
  it("loads every page, including more than 500 matches", async () => {
    const rows = Array.from({ length: 1005 }, (_, i) => event(i));
    const listEvents = vi.fn(async (req: EventListRequest) => rows.slice(Number(req.offset), Number(req.offset) + Number(req.limit)));
    const range = { houseId: "house-1", rangeStart: "2026-11-01T06:00:00Z", rangeEnd: "2026-11-02T07:00:00Z" };
    expect(await loadEvents({ listEvents }, range)).toEqual(rows);
    expect(listEvents.mock.calls.map(([req]) => req.offset)).toEqual([0, 500, 1000]);
    for (const [req] of listEvents.mock.calls) expect(req).toMatchObject(range);
  });

  it("checks for another page after an exactly full page", async () => {
    const rows = Array.from({ length: 500 }, (_, i) => event(i));
    const listEvents = vi.fn(async (req: EventListRequest) => req.offset === 0 ? rows : []);
    expect(await loadEvents({ listEvents }, { houseId: "house-1", rangeStart: "2026-11-01T00:00:00Z" })).toEqual(rows);
    expect(listEvents).toHaveBeenCalledTimes(2);
    expect(listEvents.mock.calls[1][0].rangeEnd).toBeUndefined();
  });

  it("does not return a partial calendar when a later page fails", async () => {
    const listEvents = vi.fn()
      .mockResolvedValueOnce(Array.from({ length: 500 }, (_, i) => event(i)))
      .mockRejectedValueOnce(new Error("Unavailable"));
    await expect(loadEvents({ listEvents }, { houseId: "house-1" })).rejects.toThrow("Unavailable");
  });
});

describe("calendar date ranges", () => {
  it("includes all 42 month cells and uses an exclusive end", () => {
    expect(viewRange("month", new Date(2026, 10, 15))).toEqual({
      start: new Date(2026, 9, 26), end: new Date(2026, 11, 7),
    });
  });

  it("starts weeks on Monday and ends at the next Monday", () => {
    expect(viewRange("week", new Date(2026, 10, 1, 17))).toEqual({
      start: new Date(2026, 9, 26), end: new Date(2026, 10, 2),
    });
  });

  it("uses local midnights across the daylight saving boundary", () => {
    const anchor = new Date(2026, 10, 1, 17);
    expect(viewRange("day", anchor)).toEqual({ start: new Date(2026, 10, 1), end: new Date(2026, 10, 2) });
    expect(viewRange("3day", anchor)).toEqual({ start: new Date(2026, 10, 1), end: new Date(2026, 10, 4) });
    expect(anchor.getHours()).toBe(17);
  });
});
