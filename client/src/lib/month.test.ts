import { describe, expect, it } from "vitest";
import { buildCells, groupSingleByDate, placeSpans, parseYMD, ymd } from "./month";
import type { Event } from "~/data/types";

const ev = (over: Partial<Event>): Event => ({
  id: "e",
  title: "x",
  timeLabel: "09:00",
  startDate: "2026-05-01",
  endDate: "2026-05-01",
  tone: "ocean",
  organizerColor: "#4f7ea0",
  organizer: "tod",
  ...over,
});

describe("ymd / parseYMD", () => {
  it("formats local dates without TZ shift", () => {
    expect(ymd(new Date(2026, 4, 1))).toBe("2026-05-01");
    expect(ymd(new Date(2026, 11, 9))).toBe("2026-12-09");
  });
  it("round-trips", () => {
    expect(ymd(parseYMD("2026-05-18"))).toBe("2026-05-18");
  });
});

describe("buildCells — May 2026 (Mon-first)", () => {
  const cells = buildCells(2026, 4, "2026-05-18");

  it("is a full 6-week grid", () => {
    expect(cells).toHaveLength(42);
  });

  it("starts on the Monday before May 1 (Apr 27)", () => {
    expect(cells[0].ymd).toBe("2026-04-27");
    expect(cells[0].inMonth).toBe(false);
    expect(cells[0].date.getDay()).toBe(1); // Monday
  });

  it("flags today (May 18)", () => {
    const today = cells.filter((c) => c.isToday);
    expect(today).toHaveLength(1);
    expect(today[0].ymd).toBe("2026-05-18");
  });

  it("marks May days in-month and April/June days out", () => {
    const may1 = cells.find((c) => c.ymd === "2026-05-01")!;
    const apr30 = cells.find((c) => c.ymd === "2026-04-30")!;
    expect(may1.inMonth).toBe(true);
    expect(apr30.inMonth).toBe(false);
  });

  it("flags weekends (Sat + Sun)", () => {
    const may9 = cells.find((c) => c.ymd === "2026-05-09")!; // Saturday
    const may10 = cells.find((c) => c.ymd === "2026-05-10")!; // Sunday
    const may11 = cells.find((c) => c.ymd === "2026-05-11")!; // Monday
    expect(may9.isWeekend).toBe(true);
    expect(may10.isWeekend).toBe(true);
    expect(may11.isWeekend).toBe(false);
  });
});

describe("groupSingleByDate", () => {
  it("buckets single-day events by date and skips multi-day", () => {
    const events = [
      ev({ id: "a", startDate: "2026-05-06", endDate: "2026-05-06" }),
      ev({ id: "b", startDate: "2026-05-06", endDate: "2026-05-06" }),
      ev({ id: "span", startDate: "2026-05-20", endDate: "2026-05-22" }),
    ];
    const grouped = groupSingleByDate(events);
    expect(grouped.get("2026-05-06")).toHaveLength(2);
    expect(grouped.has("2026-05-20")).toBe(false);
  });
});

describe("placeSpans", () => {
  const cells = buildCells(2026, 4, "2026-05-18");

  it("places a Sat–Sun bar at row 2, col 6, span 2", () => {
    const events = [ev({ id: "openhouse", startDate: "2026-05-09", endDate: "2026-05-10" })];
    const [p] = placeSpans(events, cells);
    expect(p.row).toBe(2);
    expect(p.col).toBe(6);
    expect(p.span).toBe(2);
  });

  it("places a Wed–Fri bar at row 4, col 3, span 3", () => {
    const events = [ev({ id: "inv", startDate: "2026-05-20", endDate: "2026-05-22" })];
    const [p] = placeSpans(events, cells);
    expect(p.row).toBe(4);
    expect(p.col).toBe(3);
    expect(p.span).toBe(3);
  });

  it("ignores single-day events", () => {
    expect(placeSpans([ev({ startDate: "2026-05-06", endDate: "2026-05-06" })], cells)).toHaveLength(0);
  });

  it("clips spans that would cross the week boundary", () => {
    // Fri May 1 (col 5) → Mon May 4 would be 4 days, but the week ends Sun.
    const events = [ev({ id: "weekend-cross", startDate: "2026-05-01", endDate: "2026-05-04" })];
    const [p] = placeSpans(events, cells);
    expect(p.col).toBe(5);
    expect(p.span).toBe(3); // clipped to end of week (Fri,Sat,Sun)
  });
});
