/**
 * Pure month-grid math for the calendar. No DOM, no Solid — just dates and
 * events in, grid structures out. Extracted from Calendar.tsx so it can be
 * unit-tested directly.
 */

import type { Event } from "~/data/types";

export interface Cell {
  date: Date;
  ymd: string;
  inMonth: boolean;
  isWeekend: boolean;
  isToday: boolean;
}

export interface SpanPlacement {
  evt: Event;
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

/** Group single-day events by their date string. Multi-day events skipped. */
export const groupSingleByDate = (events: Event[]): Map<string, Event[]> => {
  const m = new Map<string, Event[]>();
  for (const evt of events) {
    if (evt.startDate !== evt.endDate) continue;
    const arr = m.get(evt.startDate) ?? [];
    arr.push(evt);
    m.set(evt.startDate, arr);
  }
  return m;
};

/**
 * Place multi-day events as horizontal bars on the grid. Each placement is a
 * (row, col, span) for CSS grid. Spans that would cross a week boundary are
 * clipped to the end of their starting week — multi-week bars need splitting,
 * which we defer until real data needs it.
 */
export const placeSpans = (events: Event[], cells: Cell[]): SpanPlacement[] => {
  const placements: SpanPlacement[] = [];
  for (const evt of events) {
    if (evt.startDate === evt.endDate) continue;
    const startIdx = cells.findIndex((c) => c.ymd === evt.startDate);
    if (startIdx < 0) continue;
    const start = parseYMD(evt.startDate);
    const end = parseYMD(evt.endDate);
    const days = Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1;
    const row = Math.floor(startIdx / 7) + 1;
    const col = (startIdx % 7) + 1;
    const maxSpan = 8 - col;
    placements.push({ evt, row, col, span: Math.min(days, maxSpan) });
  }
  return placements;
};
