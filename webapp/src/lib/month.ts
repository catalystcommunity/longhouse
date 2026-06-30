/**
 * Pure month-grid math for the calendar. No DOM, no Solid — just dates and
 * events in, grid structures out. Extracted from Calendar.tsx so it can be
 * unit-tested directly.
 *
 * Events arrive in the CSIL shape (starts_at / ends_at as ISO timestamps).
 * We slice them to local-day YYYY-MM-DD strings up front so the rest of
 * the math is timezone-independent.
 */

import type { Event as ApiEvent } from "@longhouse/client";

export interface Cell {
  date: Date;
  ymd: string;
  inMonth: boolean;
  isWeekend: boolean;
  isToday: boolean;
}

export interface SpanPlacement {
  evt: ApiEvent;
  /** 1-indexed grid row */
  row: number;
  /** 1-indexed start column (Mon=1..Sun=7) */
  col: number;
  span: number;
}

/** YYYY-MM-DD for a local Date. */
export const ymd = (d: Date): string =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

/** Parse a YYYY-MM-DD string as a local Date (midnight, no TZ shift). */
export const parseYMD = (s: string): Date => {
  const [y, m, d] = s.split("-").map(Number);
  return new Date(y, m - 1, d);
};

/** The Monday on or before the given date. */
export const startOfMonFirstWeek = (d: Date): Date => {
  const day = d.getDay(); // 0=Sun
  const diff = day === 0 ? -6 : 1 - day;
  const r = new Date(d);
  r.setDate(d.getDate() + diff);
  return r;
};

/**
 * Build a 6-row (42-cell), Monday-first month grid for the given year/month
 * (month is 0-indexed). `todayYMD` flags which cell is "today".
 */
export const buildCells = (year: number, month: number, todayYMD: string): Cell[] => {
  const monthStart = new Date(year, month, 1);
  const gridStart = startOfMonFirstWeek(monthStart);
  const out: Cell[] = [];
  for (let i = 0; i < 42; i++) {
    const date = new Date(gridStart);
    date.setDate(gridStart.getDate() + i);
    const s = ymd(date);
    out.push({
      date,
      ymd: s,
      inMonth: date.getMonth() === month,
      isWeekend: date.getDay() === 0 || date.getDay() === 6,
      isToday: s === todayYMD,
    });
  }
  return out;
};

// startYMD/endYMD pull a local YYYY-MM-DD from the ISO timestamps without
// dragging the whole derive module in (this module is unit-tested in
// isolation). All-day events with no times are treated as the same day.
const startYMD = (e: ApiEvent): string => {
  if (e.startsAt) return ymdOfIso(e.startsAt);
  if (e.endsAt) return ymdOfIso(e.endsAt);
  return "";
};

const endYMD = (e: ApiEvent): string => {
  if (e.endsAt) return ymdOfIso(e.endsAt);
  return startYMD(e);
};

const ymdOfIso = (iso: string): string => {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso.slice(0, 10);
  return ymd(d);
};

/** Group single-day events by their date string. Multi-day events skipped. */
export const groupSingleByDate = (events: ApiEvent[]): Map<string, ApiEvent[]> => {
  const m = new Map<string, ApiEvent[]>();
  for (const evt of events) {
    const s = startYMD(evt);
    const e = endYMD(evt);
    if (!s || s !== e) continue;
    const arr = m.get(s) ?? [];
    arr.push(evt);
    m.set(s, arr);
  }
  return m;
};

/**
 * Place multi-day events as horizontal bars on the grid. Each placement is a
 * (row, col, span) for CSS grid. Spans that would cross a week boundary are
 * clipped to the end of their starting week — multi-week bars need splitting,
 * which we defer until real data needs it.
 */
export const placeSpans = (events: ApiEvent[], cells: Cell[]): SpanPlacement[] => {
  const placements: SpanPlacement[] = [];
  for (const evt of events) {
    const s = startYMD(evt);
    const e = endYMD(evt);
    if (!s || s === e) continue;
    const startIdx = cells.findIndex((c) => c.ymd === s);
    if (startIdx < 0) continue;
    const start = parseYMD(s);
    const end = parseYMD(e);
    const days = Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1;
    const row = Math.floor(startIdx / 7) + 1;
    const col = (startIdx % 7) + 1;
    const maxSpan = 8 - col;
    placements.push({ evt, row, col, span: Math.min(days, maxSpan) });
  }
  return placements;
};
