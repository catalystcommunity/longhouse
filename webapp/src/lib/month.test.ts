import { describe, expect, it } from "vitest";
import { buildCells, groupSingleByDate, placeSpans, parseYMD, ymd } from "./month";
import type { Event } from "@longhouse/client";

// Tiny event factory that pretends an all-day event for a single date —
// enough to drive the grouping/spanning math without exercising the
// transport's snake↔camel bridge.
const isoOnDay = (y: number, m: number, d: number) => new Date(y, m - 1, d, 9).toISOString();

const ev = (over: Partial<Event>): Event => ({
  eventId: "e",
  houseId: "h",
  ownerMemberId: "m",
  title: "x",
  startsAt: isoOnDay(2026, 5, 1),
  endsAt: isoOnDay(2026, 5, 1),
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
  ...over,
});

describe("ymd / parseYMD", () => {
  it("formats local dates without TZ shift", () => {
    expect(ymd(new Date(2026, 4, 1))).toBe("2026-05-01");
    expect(ymd(new Date(2026, 11, 9))).toBe("2026-12-09");
  });

  it("parses a YYYY-MM-DD into a local-midnight Date", () => {
    expect(parseYMD("2026-05-01").getMonth()).toBe(4);
    expect(parseYMD("2026-05-01").getDate()).toBe(1);
  });
});

describe("buildCells", () => {
  it("returns 42 cells starting on a Monday", () => {
    const cells = buildCells(2026, 4, "2026-05-18");
    expect(cells.length).toBe(42);
    expect(cells[0].date.getDay()).toBe(1); // Monday
    const today = cells.find((c) => c.ymd === "2026-05-18");
    expect(today?.isToday).toBe(true);
  });
});

describe("groupSingleByDate", () => {
  it("groups single-day events and skips spans", () => {
    const single = ev({ eventId: "s", startsAt: isoOnDay(2026, 5, 4), endsAt: isoOnDay(2026, 5, 4) });
    const span = ev({ eventId: "p", startsAt: isoOnDay(2026, 5, 9), endsAt: isoOnDay(2026, 5, 10) });
    const map = groupSingleByDate([single, span]);
    expect(map.get("2026-05-04")?.length).toBe(1);
    expect(map.get("2026-05-09")).toBeUndefined();
  });
});

describe("placeSpans", () => {
  it("places multi-day events with the right row/col/span", () => {
    const cells = buildCells(2026, 4, "2026-05-18");
    const span = ev({ eventId: "p", startsAt: isoOnDay(2026, 5, 9), endsAt: isoOnDay(2026, 5, 10) });
    const placements = placeSpans([span], cells);
    expect(placements.length).toBe(1);
    expect(placements[0].span).toBe(2);
  });
});
